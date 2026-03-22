package runner

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
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
	if report.ByCategory["Pictures"] != 2 {
		t.Fatalf("expected 2 pictures copied, got %d", report.ByCategory["Pictures"])
	}

	docCount := countFilesInDir(filepath.Join(destination, "Documents"))
	picCount := countFilesInDir(filepath.Join(destination, "Pictures"))
	if docCount != 3 {
		t.Fatalf("expected 3 files in destination Documents, got %d", docCount)
	}
	if picCount != 2 {
		t.Fatalf("expected 2 files in destination Pictures, got %d", picCount)
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
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		count++
	}
	return count
}
