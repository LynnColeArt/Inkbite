package components

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ResolveBaseDir returns the component base directory.
func ResolveBaseDir(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return filepath.Clean(path), nil
	}
	if path := strings.TrimSpace(os.Getenv("INKBITE_HOME")); path != "" {
		return filepath.Clean(path), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		if root := strings.TrimSpace(os.Getenv("LocalAppData")); root != "" {
			return filepath.Join(root, "inkbite"), nil
		}
		return filepath.Join(home, "AppData", "Local", "inkbite"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "inkbite"), nil
	default:
		if root := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); root != "" {
			return filepath.Join(root, "inkbite"), nil
		}
		return filepath.Join(home, ".local", "share", "inkbite"), nil
	}
}

// ConfigPath returns the config file path for the given base directory.
func ConfigPath(baseDir string) string {
	return filepath.Join(baseDir, "config", "config.json")
}

// OCRVersionsDir returns the versioned OCR bundle directory.
func OCRVersionsDir(baseDir string) string {
	return filepath.Join(baseDir, "components", "ocr", "versions")
}

// OCRInstallDir returns the installed OCR version directory.
func OCRInstallDir(baseDir string, version string) string {
	return filepath.Join(OCRVersionsDir(baseDir), sanitizeVersion(version))
}

// OCRManifestPath returns the installed OCR manifest path.
func OCRManifestPath(installDir string) string {
	return filepath.Join(installDir, "manifest.json")
}

// OCRHelperPath returns the installed OCR helper binary path.
func OCRHelperPath(installDir string) string {
	return filepath.Join(installDir, "bin", helperBinaryName())
}

func helperBinaryName() string {
	if runtime.GOOS == "windows" {
		return "inkbite-ocr-helper.exe"
	}
	return "inkbite-ocr-helper"
}

func sanitizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "dev"
	}

	var builder strings.Builder
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}

	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "dev"
	}
	return out
}

func (m Manager) baseDir() (string, error) {
	return ResolveBaseDir(m.BaseDir)
}

func (m Manager) version() string {
	return sanitizeVersion(m.Version)
}

func (m Manager) executablePath() (string, error) {
	path := strings.TrimSpace(m.ExecutablePath)
	if path == "" {
		return "", fmt.Errorf("missing executable path")
	}
	return filepath.Clean(path), nil
}
