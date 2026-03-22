package components

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/LynnColeArt/Inkbite/internal/ocr"
)

// InstallOCR installs the managed OCR helper foundation for the selected backend.
func (m Manager) InstallOCR(requestedBackend string) (InstalledComponent, error) {
	baseDir, err := m.baseDir()
	if err != nil {
		return InstalledComponent{}, err
	}

	backend, err := ocr.ResolveBackend(requestedBackend)
	if err != nil {
		return InstalledComponent{}, err
	}

	executablePath, err := m.executablePath()
	if err != nil {
		return InstalledComponent{}, err
	}

	installDir := OCRInstallDir(baseDir, m.version())
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

	if err := m.selfTest(helperPath, backend); err != nil {
		return InstalledComponent{}, err
	}

	cfgPath := ConfigPath(baseDir)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return InstalledComponent{}, err
	}
	cfg.OCR = &OCRConfig{
		Enabled:    true,
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
		Backend:    backend,
		Version:    manifest.Version,
		InstallDir: installDir,
	}, nil
}

func (m Manager) selfTest(helperPath string, backend string) error {
	if m.HelperSelfTest != nil {
		return m.HelperSelfTest(helperPath, backend)
	}
	return ocr.SelfTest(backend)
}

func (m Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
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
