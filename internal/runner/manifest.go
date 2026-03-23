package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const manifestFileName = ".archive-images-manifest.json"

type Manifest struct {
	Version int            `json:"version"`
	Hashes  map[string]string `json:"hashes"` // md5 hash -> destination path
}

// LoadManifest reads the manifest from the destination directory.
// Returns an empty manifest if the file doesn't exist.
func LoadManifest(destinationRoot string) (Manifest, error) {
	manifestPath := filepath.Join(destinationRoot, manifestFileName)
	
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No manifest yet; return empty one
			return Manifest{Version: 1, Hashes: map[string]string{}}, nil
		}
		return Manifest{}, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	
	if m.Hashes == nil {
		m.Hashes = map[string]string{}
	}
	return m, nil
}

// SaveManifest writes the manifest to the destination directory.
func SaveManifest(destinationRoot string, m Manifest) error {
	if err := os.MkdirAll(destinationRoot, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(destinationRoot, manifestFileName)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
