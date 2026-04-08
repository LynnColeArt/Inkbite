package components

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LynnColeArt/Inkbite/internal/ocr"
)

// InstallOCR installs the managed OCR helper foundation for the selected backend.
func (m Manager) InstallOCR(requestedBackend string, requestedProvider string) (InstalledComponent, error) {
	baseDir, err := m.baseDir()
	if err != nil {
		return InstalledComponent{}, err
	}

	backend, err := ocr.ResolveBackend(requestedBackend)
	if err != nil {
		return InstalledComponent{}, err
	}

	provider, err := normalizeProvider(requestedProvider)
	if err != nil {
		return InstalledComponent{}, err
	}

	switch provider {
	case "builtin":
		return m.installOCRBuiltin(baseDir, backend)
	case "paddleocr":
		return m.installOCRPaddle(baseDir, backend)
	default:
		return InstalledComponent{}, fmt.Errorf("unknown ocr provider %q", provider)
	}
}

func normalizeProvider(requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	switch requested {
	case "", "builtin":
		return "builtin", nil
	case "paddle", "paddleocr":
		return "paddleocr", nil
	default:
		return "", fmt.Errorf("unknown ocr provider %q", requested)
	}
}

func (m Manager) installOCRBuiltin(baseDir string, backend string) (InstalledComponent, error) {
	executablePath, err := m.executablePath()
	if err != nil {
		return InstalledComponent{}, err
	}

	installDir := OCRInstallDir(baseDir, m.version()+"-builtin")
	helperPath := OCRHelperPath(installDir)

	if err := os.RemoveAll(installDir); err != nil {
		return InstalledComponent{}, err
	}
	if err := os.MkdirAll(filepath.Dir(helperPath), 0o755); err != nil {
		return InstalledComponent{}, err
	}
	if err := os.MkdirAll(filepath.Join(installDir, "models"), 0o755); err != nil {
		return InstalledComponent{}, err
	}

	sum, err := copyExecutable(executablePath, helperPath)
	if err != nil {
		return InstalledComponent{}, err
	}

	manifest := BundleManifest{
		Component: "ocr",
		Provider:  "builtin",
		BundleID:  fmt.Sprintf("ocr-builtin-%s-%s-%s", backend, runtime.GOOS, runtime.GOARCH),
		Version:   m.version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Backend:   backend,
		Requirements: BundleRequirements{
			MinVRAMMB: 0,
		},
		Files: []BundleFile{
			{
				Path:   filepath.ToSlash(filepath.Join("bin", helperBinaryName())),
				SHA256: sum,
			},
		},
	}
	if err := SaveManifest(OCRManifestPath(installDir), manifest); err != nil {
		return InstalledComponent{}, err
	}

	if err := m.selfTest(helperPath, manifest.Provider, backend); err != nil {
		return InstalledComponent{}, err
	}

	cfgPath := ConfigPath(baseDir)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return InstalledComponent{}, err
	}
	cfg.OCR = &OCRConfig{
		Enabled:    true,
		Provider:   manifest.Provider,
		Backend:    backend,
		Component:  manifest.BundleID,
		Version:    manifest.Version,
		InstallDir: installDir,
	}
	if err := SaveConfig(cfgPath, cfg); err != nil {
		return InstalledComponent{}, err
	}

	return InstalledComponent{
		Name:       "ocr",
		Provider:   manifest.Provider,
		Backend:    backend,
		Version:    manifest.Version,
		InstallDir: installDir,
	}, nil
}

func (m Manager) selfTest(helperPath string, provider string, backend string) error {
	if m.HelperSelfTest != nil {
		return m.HelperSelfTest(helperPath, provider, backend)
	}
	return ocr.SelfTest(backend)
}

func (m Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
}

func (m Manager) progressf(format string, args ...any) {
	if m.ProgressWriter == nil {
		return
	}
	_, _ = fmt.Fprintf(m.ProgressWriter, format, args...)
}

func copyExecutable(srcPath string, dstPath string) (string, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return "", err
	}

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return "", err
	}
	defer dst.Close()

	hash := sha256.New()
	writer := io.MultiWriter(dst, hash)
	if _, err := io.Copy(writer, src); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
