package components

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadManifest reads an installed bundle manifest.
func LoadManifest(path string) (BundleManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BundleManifest{}, err
	}

	var manifest BundleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

// SaveManifest writes an installed bundle manifest.
func SaveManifest(path string, manifest BundleManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
