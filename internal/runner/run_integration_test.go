package runner

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Integration_NestedArchivesAndDuplicates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")

	if err := os.MkdirAll(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, "images"), 0o755); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(source, "Downloads"), 0o755); err != nil {
		t.Fatalf("mkdir downloads: %v", err)
	}

	if err := os.WriteFile(filepath.Join(source, "docs", "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "docs", "dup1.txt"), []byte("dup-content"), 0o644); err != nil {
		t.Fatalf("write dup1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "images", "photo.jpg"), []byte("jpg-bytes"), 0o644); err != nil {
		t.Fatalf("write photo.jpg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "Downloads", "setup.exe"), []byte("program"), 0o644); err != nil {
		t.Fatalf("write setup.exe: %v", err)
	}

	innerZip, err := zipBytes(map[string][]byte{
		"inner/doc2.txt": []byte("inside-doc"),
		"inner/dup2.txt": []byte("dup-content"),
	})
	if err != nil {
		t.Fatalf("build inner zip: %v", err)
	}

	outerZipPath := filepath.Join(source, "outer.zip")
	if err := writeZipFile(outerZipPath, map[string][]byte{
		"nested/inner.zip": innerZip,
		"nested/pic.png":   []byte("png-bytes"),
	}); err != nil {
		t.Fatalf("write outer zip: %v", err)
	}

	report, err := Run(Config{
		Sources:         []string{source},
		DestinationRoot: destination,
		DryRun:          false,
		MaxArchiveDepth: 5,
	})
	if err != nil {
		t.Fatalf("runner run: %v", err)
	}

	if report.Failures != 0 {
		t.Fatalf("expected no failures, got %d (%v)", report.Failures, report.Errors)
	}
	if report.CopiedFiles != 5 {
		t.Fatalf("expected 5 copied files, got %d", report.CopiedFiles)
	}
	if report.SkippedDuplicates != 1 {
		t.Fatalf("expected 1 skipped duplicate, got %d", report.SkippedDuplicates)
	}
	if report.SkippedPrograms != 1 {
		t.Fatalf("expected 1 skipped program file, got %d", report.SkippedPrograms)
	}
	if report.ArchivesProcessed != 2 {
		t.Fatalf("expected 2 processed archives (outer+inner), got %d", report.ArchivesProcessed)
	}
	if report.ArchivesExtracted != 2 {
		t.Fatalf("expected 2 extracted archives (outer+inner), got %d", report.ArchivesExtracted)
	}
	if report.ByCategory["Documents"] != 3 {
		t.Fatalf("expected 3 documents copied, got %d", report.ByCategory["Documents"])
	}
	if report.ByCategory["Photos"] != 1 {
		t.Fatalf("expected 1 photo copied, got %d", report.ByCategory["Photos"])
	}
	if report.ByCategory["Pictures"] != 1 {
		t.Fatalf("expected 1 picture copied, got %d", report.ByCategory["Pictures"])
	}

	runFolder := report.RunFolder
	docCount := countFilesInDir(filepath.Join(runFolder, "Documents"))
	photoCount := countFilesInDir(filepath.Join(runFolder, "Photos"))
	picCount := countFilesInDir(filepath.Join(runFolder, "Pictures"))
	if docCount != 3 {
		t.Fatalf("expected 3 files in destination Documents, got %d", docCount)
	}
	if photoCount != 1 {
		t.Fatalf("expected 1 file in destination Photos, got %d", photoCount)
	}
	if picCount != 1 {
		t.Fatalf("expected 1 file in destination Pictures, got %d", picCount)
	}

	assertFileExists(t, filepath.Join(runFolder, "Documents", "source", "docs", "a.txt"))
	assertFileExists(t, filepath.Join(runFolder, "Photos", "source", "images", "photo.jpg"))
	assertFileExists(t, filepath.Join(runFolder, "Pictures", "source", "outer.zip", "nested", "pic.png"))
	assertFileExists(t, filepath.Join(runFolder, "Documents", "source", "outer.zip", "nested", "inner.zip", "inner", "doc2.txt"))

	// The run log must exist and contain duplicate and stats entries.
	logPath := filepath.Join(runFolder, "run.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected run.log at %s: %v", logPath, err)
	}
	logContent := string(logBytes)
	if !strings.Contains(logContent, "[DUPLICATE]") {
		t.Fatalf("expected run.log to contain [DUPLICATE] entry; got:\n%s", logContent)
	}
	if !strings.Contains(logContent, "[ARCHIVE]") {
		t.Fatalf("expected run.log to contain [ARCHIVE] entry; got:\n%s", logContent)
	}
	if !strings.Contains(logContent, "=== RUN STATS ===") {
		t.Fatalf("expected run.log to contain RUN STATS section; got:\n%s", logContent)
	}
}

func zipBytes(entries map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipFile(path string, entries map[string][]byte) error {
	payload, err := zipBytes(entries)
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func countFilesInDir(path string) int {
	count := 0
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %s (%v)", path, err)
	}
}
