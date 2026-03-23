package components

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	paddleProvider         = "paddleocr"
	paddleCPUWheelIndexURL = "https://www.paddlepaddle.org.cn/packages/stable/cpu/"
	paddlePaddleVersion    = "3.2.0"
	paddleHelperScriptName = "paddle_ocr_helper.py"
)

func (m Manager) installOCRPaddle(baseDir string, backend string) (InstalledComponent, error) {
	if runtime.GOOS == "windows" {
		return InstalledComponent{}, fmt.Errorf("ocr provider %q is not yet available on windows", paddleProvider)
	}

	pythonPath, err := lookupPython()
	if err != nil {
		return InstalledComponent{}, err
	}

	installDir := OCRInstallDir(baseDir, m.version()+"-"+paddleProvider)
	helperPath := OCRHelperPath(installDir)
	venvDir := filepath.Join(installDir, "venv")
	libexecDir := filepath.Join(installDir, "libexec")
	scriptPath := filepath.Join(libexecDir, paddleHelperScriptName)
	modelCacheDir := filepath.Join(installDir, "models")
	homeDir := filepath.Join(installDir, "home")

	if err := os.RemoveAll(installDir); err != nil {
		return InstalledComponent{}, err
	}
	for _, dir := range []string{filepath.Dir(helperPath), libexecDir, modelCacheDir, homeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return InstalledComponent{}, err
		}
	}

	if err := runCommand(pythonPath, "-m", "venv", venvDir); err != nil {
		return InstalledComponent{}, fmt.Errorf("create paddleocr venv: %w", err)
	}

	venvPython := filepath.Join(venvDir, "bin", "python")
	if err := runCommand(venvPython, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
		return InstalledComponent{}, fmt.Errorf("upgrade pip: %w", err)
	}
	if err := runCommand(venvPython, "-m", "pip", "install", "paddlepaddle=="+paddlePaddleVersion, "-i", paddleCPUWheelIndexURL); err != nil {
		return InstalledComponent{}, fmt.Errorf("install paddlepaddle cpu runtime: %w", err)
	}
	if err := runCommand(venvPython, "-m", "pip", "install", "paddleocr"); err != nil {
		return InstalledComponent{}, fmt.Errorf("install paddleocr: %w", err)
	}

	if err := os.WriteFile(scriptPath, []byte(paddleHelperScript), 0o644); err != nil {
		return InstalledComponent{}, err
	}
	sum, err := writePaddleWrapper(helperPath, venvPython, scriptPath, modelCacheDir, homeDir)
	if err != nil {
		return InstalledComponent{}, err
	}

	manifest := BundleManifest{
		Component: "ocr",
		Provider:  paddleProvider,
		BundleID:  fmt.Sprintf("ocr-%s-%s-%s-%s", paddleProvider, backend, runtime.GOOS, runtime.GOARCH),
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
			{
				Path:   filepath.ToSlash(filepath.Join("libexec", paddleHelperScriptName)),
				SHA256: fileSHA256(scriptPath),
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

func lookupPython() (string, error) {
	for _, candidate := range []string{"python3", "python"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("python3 is required for ocr provider %q", paddleProvider)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s: %s", strings.Join(append([]string{name}, args...), " "), message)
	}
	return nil
}

func writePaddleWrapper(helperPath string, venvPython string, scriptPath string, modelCacheDir string, homeDir string) (string, error) {
	wrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
export HOME="%s"
export XDG_CACHE_HOME="%s"
export PADDLE_PDX_MODEL_SOURCE="BOS"
exec "%s" "%s" "$@"
`, homeDir, modelCacheDir, venvPython, scriptPath)

	if err := os.WriteFile(helperPath, []byte(wrapper), 0o755); err != nil {
		return "", err
	}
	return fileSHA256(helperPath), nil
}

func fileSHA256(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

const paddleHelperScript = `#!/usr/bin/env python3
import argparse
import json
import sys

from paddleocr import PaddleOCR


def build_ocr(backend: str) -> PaddleOCR:
    if backend != "cpu":
        raise SystemExit(f"unsupported paddleocr backend: {backend}")

    return PaddleOCR(
        text_detection_model_name="PP-OCRv5_mobile_det",
        text_recognition_model_name="PP-OCRv5_mobile_rec",
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=False,
        device="cpu",
    )


def main() -> int:
    parser = argparse.ArgumentParser(prog="inkbite-ocr-helper")
    parser.add_argument("--self-test", action="store_true", dest="self_test")
    parser.add_argument("--backend", default="cpu")
    args = parser.parse_args()

    if not args.self_test:
        parser.error("only --self-test is currently supported")

    build_ocr(args.backend)
    print(json.dumps({"status": "ok", "provider": "paddleocr", "backend": "cpu"}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
`
