package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueDestinationPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	categoryDir := filepath.Join(root, "Documents", "backup", "docs")
	if err := os.MkdirAll(categoryDir, 0o755); err != nil {
		t.Fatalf("mkdir category: %v", err)
	}

	existing := filepath.Join(categoryDir, "report.txt")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	planned := map[string]struct{}{}

	first, err := uniqueDestinationPath(root, "Documents", filepath.Join("backup", "docs", "report.txt"), planned)
	if err != nil {
		t.Fatalf("first unique path: %v", err)
	}
	if filepath.Base(first) != "report_1.txt" {
		t.Fatalf("expected report_1.txt, got %s", filepath.Base(first))
	}
	if filepath.Dir(first) != categoryDir {
		t.Fatalf("expected preserved directory %s, got %s", categoryDir, filepath.Dir(first))
	}

	second, err := uniqueDestinationPath(root, "Documents", filepath.Join("backup", "docs", "report.txt"), planned)
	if err != nil {
		t.Fatalf("second unique path: %v", err)
	}
	if filepath.Base(second) != "report_2.txt" {
		t.Fatalf("expected report_2.txt, got %s", filepath.Base(second))
	}
}

func TestUniqueDestinationPathRejectsInvalidBase(t *testing.T) {
	t.Parallel()

	_, err := uniqueDestinationPath(t.TempDir(), "Other", "", map[string]struct{}{})
	if err == nil {
		t.Fatal("expected invalid base name error")
	}
}

func TestRelativeDestinationPath(t *testing.T) {
	t.Parallel()

	got, err := relativeDestinationPath("/data/source", "source", "/data/source/photos/trip/pic.jpg")
	if err != nil {
		t.Fatalf("relativeDestinationPath: %v", err)
	}

	want := filepath.Join("source", "photos", "trip", "pic.jpg")
	if got != want {
		t.Fatalf("relativeDestinationPath() = %q, want %q", got, want)
	}
}

func TestFileMD5(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "sample.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	sum, err := fileMD5(path)
	if err != nil {
		t.Fatalf("fileMD5: %v", err)
	}
	if sum != "900150983cd24fb0d6963f7d28e17f72" {
		t.Fatalf("unexpected md5: %s", sum)
	}
}

func TestIsArchiveLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{path: "a.zip", want: true},
		{path: "a.tar", want: true},
		{path: "a.tar.gz", want: true},
		{path: "a.tgz", want: true},
		{path: "a.rar", want: true},
		{path: "a.7z", want: true},
		{path: "a.txt", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			if got := isArchiveLike(tc.path); got != tc.want {
				t.Fatalf("isArchiveLike(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
