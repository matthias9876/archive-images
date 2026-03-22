package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func IsSupportedArchive(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".zip") || strings.HasSuffix(p, ".tar") || strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz")
}

func Extract(path string, destination string) error {
	p := strings.ToLower(path)
	switch {
	case strings.HasSuffix(p, ".zip"):
		return extractZip(path, destination)
	case strings.HasSuffix(p, ".tar"):
		return extractTar(path, destination, false)
	case strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz"):
		return extractTar(path, destination, true)
	default:
		return fmt.Errorf("unsupported archive: %s", path)
	}
}

func extractZip(path string, destination string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target, err := secureJoin(destination, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		if err := writeFileFromReader(target, rc, f.Mode()); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func extractTar(path string, destination string, compressed bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader = f
	if compressed {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := secureJoin(destination, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFileFromReader(target, tr, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeFileFromReader(target string, r io.Reader, mode os.FileMode) error {
	w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, sanitizeMode(mode))
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, r)
	return err
}

func sanitizeMode(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o644
	}
	return mode.Perm()
}

func secureJoin(root string, relative string) (string, error) {
	clean := filepath.Clean(relative)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("archive path escapes root: %s", relative)
	}
	target := filepath.Join(root, clean)
	rootClean := filepath.Clean(root)
	rel, err := filepath.Rel(rootClean, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("archive path escapes root: %s", relative)
	}
	return target, nil
}
