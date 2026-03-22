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

	"archive-images/internal/archive"
	"archive-images/internal/classify"
	"archive-images/internal/filter"
)

type walkerItem struct {
	Root  string
	Depth int
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

	if cfg.MaxArchiveDepth < 0 {
		return report, errors.New("max archive depth must be >= 0")
	}

	if !cfg.DryRun {
		if err := os.MkdirAll(cfg.DestinationRoot, 0o755); err != nil {
			return report, fmt.Errorf("create destination root: %w", err)
		}
	}

	tempRoot, err := os.MkdirTemp("", "archive-images-work-")
	if err != nil {
		return report, fmt.Errorf("create temp root: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	hashes := map[string]string{}
	destinations := map[string]struct{}{}

	queue := make([]walkerItem, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		queue = append(queue, walkerItem{Root: source, Depth: 0})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		err := filepath.WalkDir(item.Root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				reportFailure(&report, fmt.Sprintf("walk error at %s: %v", path, walkErr))
				return nil
			}
			if d.IsDir() {
				return nil
			}

			report.TotalFilesSeen++
			if isArchiveLike(path) {
				report.ArchivesProcessed++
				if archive.IsSupportedArchive(path) {
					if item.Depth >= cfg.MaxArchiveDepth {
						reportFailure(&report, fmt.Sprintf("archive depth limit reached: %s", path))
						return nil
					}

					extractTo, err := os.MkdirTemp(tempRoot, "extract-")
					if err != nil {
						reportFailure(&report, fmt.Sprintf("create extract temp for %s: %v", path, err))
						return nil
					}
					if err := archive.Extract(path, extractTo); err != nil {
						reportFailure(&report, fmt.Sprintf("extract %s: %v", path, err))
						return nil
					}
					report.ArchivesExtracted++
					queue = append(queue, walkerItem{Root: extractTo, Depth: item.Depth + 1})
				} else {
					report.UnsupportedArchive++
				}
				return nil
			}

			if filter.IsLikelyProgram(path) {
				report.SkippedPrograms++
				return nil
			}

			hash, err := fileMD5(path)
			if err != nil {
				reportFailure(&report, fmt.Sprintf("hash %s: %v", path, err))
				return nil
			}
			if _, seen := hashes[hash]; seen {
				report.SkippedDuplicates++
				return nil
			}
			hashes[hash] = path

			category := classify.CategoryFor(path)
			report.ByCategory[category]++

			destinationPath, err := uniqueDestinationPath(cfg.DestinationRoot, category, filepath.Base(path), destinations)
			if err != nil {
				reportFailure(&report, fmt.Sprintf("plan destination %s: %v", path, err))
				return nil
			}

			if cfg.DryRun {
				cfg.Logf("[DRY-RUN] COPY %s -> %s", path, destinationPath)
				report.CopiedFiles++
				return nil
			}

			if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
				reportFailure(&report, fmt.Sprintf("mkdir %s: %v", filepath.Dir(destinationPath), err))
				return nil
			}
			if err := copyFile(path, destinationPath); err != nil {
				reportFailure(&report, fmt.Sprintf("copy %s -> %s: %v", path, destinationPath, err))
				return nil
			}

			report.CopiedFiles++
			cfg.Logf("COPY %s -> %s", path, destinationPath)
			return nil
		})
		if err != nil {
			reportFailure(&report, fmt.Sprintf("walk root %s: %v", item.Root, err))
		}
	}

	if cfg.ReportPath != "" {
		if err := writeReport(cfg.ReportPath, report); err != nil {
			return report, fmt.Errorf("write report: %w", err)
		}
	}

	return report, nil
}

func reportFailure(report *Report, msg string) {
	report.Failures++
	report.Errors = append(report.Errors, msg)
}

func fileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func uniqueDestinationPath(root, category, base string, planned map[string]struct{}) (string, error) {
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "", errors.New("invalid file name")
	}

	categoryDir := filepath.Join(root, category)
	candidate := filepath.Join(categoryDir, base)

	for i := 0; ; i++ {
		path := candidate
		if i > 0 {
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			path = filepath.Join(categoryDir, fmt.Sprintf("%s_%d%s", name, i, ext))
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
