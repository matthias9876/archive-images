// Package runner executes the end-to-end archive-images pipeline.
//
// High-level processing blocks in Run:
// 1) Setup and safety checks
//    - Initializes report fields and default log/debug callbacks.
//    - Validates max archive depth and creates destination root when not dry-run.
//    - Loads the manifest from destination to enable cross-run deduplication.
//
// 2) Work queue traversal
//    - Builds a queue of walkerItem roots (initial sources and extracted archives).
//    - Each root is walked with filepath.WalkDir.
//    - Archive files are extracted into temp dirs and re-enqueued with depth+1,
//      up to cfg.MaxArchiveDepth.
//
// 3) Early file skipping (no copy)
//    - Program/installer filtering happens first via filter.IsLikelyProgram(path).
//      Matching files are counted as skipped programs and never copied.
//    - Category classification then runs via classify.CategoryFor(path).
//      If cfg.EnabledCategories is set and the category is disabled, the file is
//      skipped before hashing/copying.
//
// 4) Destination planning
//    - relativeDestinationPath computes the path under category/source prefix.
//    - uniqueDestinationPath reserves a collision-safe destination path.
//      If a later step fails or detects duplicate, that reservation is released.
//
// 5) Hash and dedup behavior (dry-run vs real copy)
//    - Dry-run path:
//      * hashOnly reads source once and computes MD5 without writing any file.
//      * The hash is checked against:
//        a) manifest.Hashes (duplicates from previous runs), then
//        b) in-memory hashes map (duplicates seen earlier in current run).
//      * On duplicate: file is skipped.
//      * On unique: counters are updated and manifest map is updated in memory
//        only for planning/reporting (manifest is not persisted in dry-run).
//
//    - Real copy path:
//      * hashAndCopy reads source once and writes to destination.tmp while
//        computing MD5 using io.TeeReader.
//      * Dedup check is performed after temporary copy:
//        a) manifest duplicate => remove .tmp and skip
//        b) in-run duplicate   => remove .tmp and skip
//      * If unique, .tmp is atomically renamed to final path.
//      * Hash is recorded into manifest and run-local hash map.
//
// 6) Finalization
//    - Optional JSON report is written when cfg.ReportPath is set.
//    - Manifest is saved only when dry-run is false, making dedup state
//      resumable across future runs.
//
// Supporting helpers in this file:
// - hashOnly: MD5 of a source file (used by dry-run).
// - hashAndCopy: one-pass copy+MD5 computation for real runs.
// - reportFailure: records non-fatal errors without stopping the full walk.
package runner

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"archive-images/internal/archive"
	"archive-images/internal/classify"
	"archive-images/internal/filter"
)

type walkerItem struct {
	Root        string
	Depth       int
	DestPrefix  string
	ArchivePath string // non-empty when this root was extracted from an archive
}

func Run(cfg Config) (Report, error) {
	report := Report{
		DryRun:     cfg.DryRun,
		ByCategory: map[string]int{},
		Errors:     []string{},
	}

	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	if cfg.Debugf == nil {
		cfg.Debugf = func(string, ...any) {}
	}

	if cfg.MaxArchiveDepth < 0 {
		return report, errors.New("max archive depth must be >= 0")
	}

	// Each run gets its own output folder named "<dest-base>_YYYY-MM-DD_HH-MM-SS"
	// so results are easy to review and runs never overwrite each other.
	stamp := runStamp(cfg.DestinationRoot)
	runRoot := filepath.Join(cfg.DestinationRoot, stamp)
	report.RunFolder = runRoot

	if !cfg.DryRun {
		// Parent dest must exist for the manifest; run folder holds copied files.
		if err := os.MkdirAll(cfg.DestinationRoot, 0o755); err != nil {
			return report, fmt.Errorf("create destination root: %w", err)
		}
		if err := os.MkdirAll(runRoot, 0o755); err != nil {
			return report, fmt.Errorf("create run folder: %w", err)
		}
	}

	// Load existing manifest from destination for cross-source deduplication
	manifest, err := LoadManifest(cfg.DestinationRoot)
	if err != nil {
		return report, fmt.Errorf("load manifest: %w", err)
	}
	cfg.Logf("Loaded manifest with %d known hashes", len(manifest.Hashes))

	tempRoot, err := os.MkdirTemp("", "archive-images-work-")
	if err != nil {
		return report, fmt.Errorf("create temp root: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	// Open a per-run log file (skipped on dry-run since no files are written).
	// The log records every duplicate decision and per-archive stats so the
	// run result can be reviewed without re-running the command.
	var logWriter io.Writer = io.Discard
	if !cfg.DryRun {
		logFile, err := os.OpenFile(filepath.Join(runRoot, "run.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return report, fmt.Errorf("create run log: %w", err)
		}
		defer logFile.Close()
		logWriter = logFile
		fmt.Fprintf(logWriter, "=== archive-images run: %s ===\n", stamp)
		fmt.Fprintf(logWriter, "Started: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	}

	enabledCats := map[string]bool{}
	for _, c := range cfg.EnabledCategories {
		enabledCats[c] = true
	}

	// Single buffer reused across all file operations to avoid GC pressure and
	// to ensure every read issues large sequential requests to the HDD.
	ioBuf := make([]byte, copyBufSize)

	hashes := map[string]string{}
	destinations := map[string]struct{}{}

	// BFS-style queue of roots to scan. Initial items are user sources; extracted
	// archives are appended later so nested archives are processed in order.
	queue := make([]walkerItem, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		queue = append(queue, walkerItem{
			Root:       source,
			Depth:      0,
			DestPrefix: sourcePrefix(source),
		})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		cfg.Debugf("scanning %s (archive depth=%d, prefix=%s, queue remaining=%d)", item.Root, item.Depth, item.DestPrefix, len(queue))

		// Snapshot counters before walking so per-archive stats can be computed after.
		beforeSnap := report

		err := filepath.WalkDir(item.Root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				// Non-fatal: record and continue scanning other files.
				reportFailure(&report, fmt.Sprintf("walk error at %s: %v", path, walkErr))
				return nil
			}
			if d.IsDir() {
				if path != item.Root && filter.ShouldSkipDirectory(path) {
					cfg.Debugf("skipping directory subtree: %s", path)
					return filepath.SkipDir
				}
				cfg.Debugf("entering dir %s", path)
				return nil
			}

			report.TotalFilesSeen++
			if isArchiveLike(path) {
				report.ArchivesProcessed++
				if archive.IsSupportedArchive(path) {
					// Prevent unbounded recursion from deeply nested archives.
					if item.Depth >= cfg.MaxArchiveDepth {
						cfg.Debugf("archive depth limit (%d) reached, skipping: %s", cfg.MaxArchiveDepth, path)
						reportFailure(&report, fmt.Sprintf("archive depth limit reached: %s", path))
						return nil
					}

					extractTo, err := os.MkdirTemp(tempRoot, "extract-")
					if err != nil {
						reportFailure(&report, fmt.Sprintf("create extract temp for %s: %v", path, err))
						return nil
					}
					cfg.Debugf("extracting %s -> %s", path, extractTo)
					if err := archive.Extract(path, extractTo); err != nil {
						reportFailure(&report, fmt.Sprintf("extract %s: %v", path, err))
						return nil
					}
					archivePrefix, err := relativeDestinationPath(item.Root, item.DestPrefix, path)
					if err != nil {
						reportFailure(&report, fmt.Sprintf("plan archive prefix %s: %v", path, err))
						return nil
					}
					// Queue extracted content as another root; this reuses the same
					// processing pipeline (filters, classification, dedup, copy).
					report.ArchivesExtracted++
					queue = append(queue, walkerItem{
					Root:        extractTo,
					Depth:       item.Depth + 1,
					DestPrefix:  archivePrefix,
					ArchivePath: path,
					})
					cfg.Debugf("extracted %s -> enqueued (archive depth=%d, queue size=%d)", path, item.Depth+1, len(queue))
				} else {
					cfg.Debugf("unsupported archive format, skipping: %s", path)
					report.UnsupportedArchive++
				}
				return nil
			}

			if filter.IsLikelyProgram(path) {
				// Safety filter: executable/installer-like files are intentionally excluded.
				cfg.Debugf("skipping program: %s", path)
				report.SkippedPrograms++
				return nil
			}

			// Classify and filter by category before any disk I/O so
			// unwanted categories are skipped without reading the file.
			category := classify.CategoryFor(path)
			if len(enabledCats) > 0 && !enabledCats[category] {
				cfg.Debugf("skipping (category %s not in filter): %s", category, path)
				return nil
			}

			relPath, err := relativeDestinationPath(item.Root, item.DestPrefix, path)
			if err != nil {
				reportFailure(&report, fmt.Sprintf("plan relative path %s: %v", path, err))
				return nil
			}

			destinationPath, err := uniqueDestinationPath(runRoot, category, relPath, destinations)
			if err != nil {
				reportFailure(&report, fmt.Sprintf("plan destination %s: %v", path, err))
				return nil
			}

			if cfg.DryRun {
				// Dry-run: hash only (single read, no write).
				cfg.Debugf("hashing %s", path)
				hash, err := hashOnly(path, ioBuf)
				if err != nil {
					// Release reserved destination because this file will not be copied.
					delete(destinations, destinationPath)
					reportFailure(&report, fmt.Sprintf("hash %s: %v", path, err))
					return nil
				}
				if _, inManifest := manifest.Hashes[hash]; inManifest {
					cfg.Debugf("duplicate (manifest, hash=%s): %s", hash, path)
					fmt.Fprintf(logWriter, "[DUPLICATE] %s\n  => already at: %s\n  hash: %s\n\n", path, manifest.Hashes[hash], hash)
					delete(destinations, destinationPath)
					report.SkippedDuplicates++
					return nil
				}
				if _, seen := hashes[hash]; seen {
					cfg.Debugf("duplicate (in-run, hash=%s): %s", hash, path)
					fmt.Fprintf(logWriter, "[DUPLICATE] %s\n  => same content as: %s\n  hash: %s\n\n", path, hashes[hash], hash)
					delete(destinations, destinationPath)
					report.SkippedDuplicates++
					return nil
				}
				hashes[hash] = path
				manifest.Hashes[hash] = destinationPath
				cfg.Logf("[DRY-RUN] COPY %s -> %s", path, destinationPath)
				report.ByCategory[category]++
				report.CopiedFiles++
				return nil
			}

			// Real copy: single sequential read from the source drive.
			// hashAndCopy computes the MD5 while writing via io.TeeReader so
			// the file is only read once, halving the I/O cost on an HDD.
			cfg.Debugf("copying %s -> %s", path, destinationPath)
			if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
				// Destination was reserved above; release it on failure.
				delete(destinations, destinationPath)
				reportFailure(&report, fmt.Sprintf("mkdir %s: %v", filepath.Dir(destinationPath), err))
				return nil
			}
			tmpPath := destinationPath + ".tmp"
			hash, err := hashAndCopy(path, tmpPath, ioBuf)
			if err != nil {
				os.Remove(tmpPath)
				delete(destinations, destinationPath)
				reportFailure(&report, fmt.Sprintf("copy %s: %v", path, err))
				return nil
			}

			// Deduplication check after the copy.  Files already known from
			// the manifest (previous runs) or seen earlier in this run are
			// discarded.  The temp file is removed so no duplicate ever lands
			// in the destination tree.
			if _, inManifest := manifest.Hashes[hash]; inManifest {
				cfg.Debugf("duplicate (manifest, hash=%s): %s", hash, path)
				fmt.Fprintf(logWriter, "[DUPLICATE] %s\n  => already at: %s\n  hash: %s\n\n", path, manifest.Hashes[hash], hash)
				os.Remove(tmpPath)
				delete(destinations, destinationPath)
				report.SkippedDuplicates++
				return nil
			}
			if _, seen := hashes[hash]; seen {
				cfg.Debugf("duplicate (in-run, hash=%s): %s", hash, path)
				fmt.Fprintf(logWriter, "[DUPLICATE] %s\n  => same content as: %s\n  hash: %s\n\n", path, hashes[hash], hash)
				os.Remove(tmpPath)
				delete(destinations, destinationPath)
				report.SkippedDuplicates++
				return nil
			}
			hashes[hash] = path

			if err := os.Rename(tmpPath, destinationPath); err != nil {
				// Keep temp cleanup + reservation rollback symmetric on all failures.
				os.Remove(tmpPath)
				delete(destinations, destinationPath)
				reportFailure(&report, fmt.Sprintf("rename %s: %v", tmpPath, err))
				return nil
			}

			manifest.Hashes[hash] = destinationPath
			report.ByCategory[category]++
			report.CopiedFiles++
			cfg.Logf("COPY %s -> %s", path, destinationPath)
			return nil
		})
		if err != nil {
			reportFailure(&report, fmt.Sprintf("walk root %s: %v", item.Root, err))
		}

		// Write per-archive stats when this root was produced by an extracted archive.
		if item.ArchivePath != "" {
			fmt.Fprintf(logWriter, "[ARCHIVE] %s\n", item.ArchivePath)
			fmt.Fprintf(logWriter, "  Files seen:       %d\n", report.TotalFilesSeen-beforeSnap.TotalFilesSeen)
			fmt.Fprintf(logWriter, "  Copied:           %d\n", report.CopiedFiles-beforeSnap.CopiedFiles)
			fmt.Fprintf(logWriter, "  Duplicates:       %d\n", report.SkippedDuplicates-beforeSnap.SkippedDuplicates)
			fmt.Fprintf(logWriter, "  Programs skipped: %d\n", report.SkippedPrograms-beforeSnap.SkippedPrograms)
			fmt.Fprintf(logWriter, "  Failures:         %d\n\n", report.Failures-beforeSnap.Failures)
		}
	}

	if cfg.ReportPath != "" {
		if err := writeReport(cfg.ReportPath, report); err != nil {
			return report, fmt.Errorf("write report: %w", err)
		}
	}

	// Save manifest for resumability and cross-source deduplication
	if !cfg.DryRun {
		if err := SaveManifest(cfg.DestinationRoot, manifest); err != nil {
			return report, fmt.Errorf("save manifest: %w", err)
		}
		cfg.Logf("Manifest saved with %d hashes", len(manifest.Hashes))
	}

	// Write final run stats at the bottom of the log file.
	fmt.Fprintf(logWriter, "=== RUN STATS ===\n")
	fmt.Fprintf(logWriter, "Destination:         %s\n", runRoot)
	fmt.Fprintf(logWriter, "Total files seen:    %d\n", report.TotalFilesSeen)
	fmt.Fprintf(logWriter, "Files copied:        %d\n", report.CopiedFiles)
	fmt.Fprintf(logWriter, "Duplicates skipped:  %d\n", report.SkippedDuplicates)
	fmt.Fprintf(logWriter, "Programs skipped:    %d\n", report.SkippedPrograms)
	fmt.Fprintf(logWriter, "Archives processed:  %d\n", report.ArchivesProcessed)
	fmt.Fprintf(logWriter, "Archives extracted:  %d\n", report.ArchivesExtracted)
	fmt.Fprintf(logWriter, "Failures:            %d\n", report.Failures)
	fmt.Fprintf(logWriter, "\nBy category:\n")
	for _, cat := range []string{
		classify.CategoryDocuments,
		classify.CategoryPhotos,
		classify.CategoryPictures,
		classify.CategoryVideos,
		classify.CategoryMusic,
		classify.CategoryOther,
	} {
		fmt.Fprintf(logWriter, "  %-12s %d\n", cat+":", report.ByCategory[cat])
	}

	return report, nil
}

func reportFailure(report *Report, msg string) {
	report.Failures++
	report.Errors = append(report.Errors, msg)
}

// copyBufSize is the I/O buffer size used for all file reads and writes.
// 4 MiB amortises HDD rotational latency by keeping the drive streaming
// sequentially rather than issuing many small requests.
const copyBufSize = 4 << 20 // 4 MiB

// hashOnly reads path and returns its MD5 hex string. Used in dry-run mode
// where no destination file is written.
func hashOnly(path string, buf []byte) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashAndCopy reads source once, writing to destination while computing the
// MD5 hash in parallel via io.TeeReader. This avoids a second seek-and-read
// pass over the spinning source drive. No Sync is called so the OS can
// pipeline destination writes through its write-back cache.
func hashAndCopy(source, destination string, buf []byte) (hash string, err error) {
	src, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := dst.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	h := md5.New()
	if _, err = io.CopyBuffer(dst, io.TeeReader(src, h), buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sourcePrefix(source string) string {
	base := filepath.Base(filepath.Clean(source))
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "source"
	}
	return base
}

func relativeDestinationPath(root, prefix, path string) (string, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	relPath = filepath.Clean(relPath)
	if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "..") {
		return "", errors.New("invalid relative path")
	}
	if prefix == "" {
		return relPath, nil
	}
	return filepath.Join(prefix, relPath), nil
}

func uniqueDestinationPath(root, category, relativePath string, planned map[string]struct{}) (string, error) {
	if relativePath == "" || relativePath == "." || relativePath == string(filepath.Separator) {
		return "", errors.New("invalid file name")
	}

	relPath := filepath.Clean(relativePath)
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "..") {
		return "", errors.New("invalid relative path")
	}

	categoryDir := filepath.Join(root, category)
	candidate := filepath.Join(categoryDir, relPath)
	candidateDir := filepath.Dir(candidate)
	base := filepath.Base(candidate)

	for i := 0; ; i++ {
		path := candidate
		if i > 0 {
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			path = filepath.Join(candidateDir, fmt.Sprintf("%s_%d%s", name, i, ext))
		}

		if _, exists := planned[path]; exists {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		planned[path] = struct{}{}
		return path, nil
	}
}

func writeReport(path string, report Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func isArchiveLike(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".zip") || strings.HasSuffix(p, ".tar") || strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz") || strings.HasSuffix(p, ".rar") || strings.HasSuffix(p, ".7z")
}

// runStamp returns a folder name derived from the destination base name and
// the current wall clock time, e.g. "organized_2026-03-26_14-30-00".
// Each run produces a distinct top-level folder so results never collide.
func runStamp(destRoot string) string {
	base := filepath.Base(filepath.Clean(destRoot))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "run"
	}
	return base + "_" + time.Now().Format("2006-01-02_15-04-05")
}
