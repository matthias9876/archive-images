package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestIsSupportedArchive(t *testing.T) {
	t.Parallel()

	if !IsSupportedArchive("backup.ZIP") {
		t.Fatal("expected zip to be supported")
	}
	if !IsSupportedArchive("backup.tgz") {
		t.Fatal("expected tgz to be supported")
	}
	if IsSupportedArchive("backup.rar") {
		t.Fatal("did not expect rar to be supported")
	}
}

func TestExtractZip(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "data.zip")
	destination := filepath.Join(root, "out")

	if err := createZip(archivePath, map[string]string{
		"docs/report.txt": "hello zip",
	}); err != nil {
		t.Fatalf("create zip: %v", err)
	}

	if err := Extract(archivePath, destination); err != nil {
		t.Fatalf("extract zip: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(destination, "docs", "report.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "hello zip" {
		t.Fatalf("unexpected extracted content: %q", string(content))
	}
}

func TestExtractTarGz(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "data.tar.gz")
	destination := filepath.Join(root, "out")

	if err := createTarGz(archivePath, map[string]string{
		"pictures/photo.txt": "hello tar",
	}); err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}

	if err := Extract(archivePath, destination); err != nil {
		t.Fatalf("extract tar.gz: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(destination, "pictures", "photo.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "hello tar" {
		t.Fatalf("unexpected extracted content: %q", string(content))
	}
}

func TestExtractRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "evil.zip")
	destination := filepath.Join(root, "out")

	if err := createZip(archivePath, map[string]string{
		"../evil.txt": "nope",
	}); err != nil {
		t.Fatalf("create zip: %v", err)
	}

	if err := Extract(archivePath, destination); err == nil {
		t.Fatal("expected extract to fail for path traversal entry")
	}
}

func TestSecureJoin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	joined, err := secureJoin(root, "docs/file.txt")
	if err != nil {
		t.Fatalf("secureJoin valid path: %v", err)
	}
	if filepath.Dir(joined) != filepath.Join(root, "docs") {
		t.Fatalf("unexpected secure path: %s", joined)
	}

	if _, err := secureJoin(root, "../escape.txt"); err == nil {
		t.Fatal("expected secureJoin to reject parent traversal")
	}
}

func createZip(path string, entries map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := io.Copy(w, bytes.NewBufferString(content)); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

func createTarGz(path string, entries map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	for name, content := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return err
		}
	}

	if err := tw.Close(); err != nil {
		_ = gz.Close()
		return err
	}
	return gz.Close()
}
