package components

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	paddleOCRVersion       = "3.4.0"
	paddleChardetVersion   = "5.2.0"
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

	m.progressf("creating paddleocr virtual environment\n")
	if err := runCommand(m.ProgressWriter, pythonPath, "-m", "venv", venvDir); err != nil {
		return InstalledComponent{}, fmt.Errorf("create paddleocr venv: %w", err)
	}

	venvPython := filepath.Join(venvDir, "bin", "python")
	m.progressf("upgrading pip in managed paddleocr environment\n")
	if err := runCommand(m.ProgressWriter, venvPython, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
		return InstalledComponent{}, fmt.Errorf("upgrade pip: %w", err)
	}
	m.progressf("installing paddlepaddle cpu runtime; this can take a while\n")
	if err := runCommand(m.ProgressWriter, venvPython, "-m", "pip", "install", "paddlepaddle=="+paddlePaddleVersion, "-i", paddleCPUWheelIndexURL); err != nil {
		return InstalledComponent{}, fmt.Errorf("install paddlepaddle cpu runtime: %w", err)
	}
	m.progressf("installing paddleocr package set\n")
	if err := runCommand(m.ProgressWriter, venvPython, "-m", "pip", "install", "paddleocr=="+paddleOCRVersion); err != nil {
		return InstalledComponent{}, fmt.Errorf("install paddleocr: %w", err)
	}
	m.progressf("normalizing python dependency pins for managed paddleocr runtime\n")
	if err := runCommand(m.ProgressWriter, venvPython, "-m", "pip", "install", "chardet=="+paddleChardetVersion); err != nil {
		return InstalledComponent{}, fmt.Errorf("pin chardet compatibility: %w", err)
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

	m.progressf("running managed paddleocr self-test\n")
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

func runCommand(progress io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var output bytes.Buffer
	if progress != nil {
		stream := io.MultiWriter(progress, &output)
		cmd.Stdout = stream
		cmd.Stderr = stream
	} else {
		cmd.Stdout = &output
		cmd.Stderr = &output
	}

	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(output.String())
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
export PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK="True"
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
import contextlib
import io
import json
import logging
import os
import sys
import tempfile

import paddlex.utils.logging as paddlex_logging


def build_ocr(backend: str):
    if backend != "cpu":
        raise SystemExit(f"unsupported paddleocr backend: {backend}")

    from paddleocr import PaddleOCR

    return PaddleOCR(
        text_detection_model_name="PP-OCRv5_mobile_det",
        text_recognition_model_name="PP-OCRv5_mobile_rec",
        use_doc_orientation_classify=False,
        use_doc_unwarping=False,
        use_textline_orientation=False,
        device="cpu",
    )


@contextlib.contextmanager
def capture_native_output():
    stdout_fd = sys.__stdout__.fileno()
    stderr_fd = sys.__stderr__.fileno()
    saved_stdout = os.dup(stdout_fd)
    saved_stderr = os.dup(stderr_fd)

    with tempfile.TemporaryFile(mode="w+b") as stdout_tmp, tempfile.TemporaryFile(mode="w+b") as stderr_tmp:
        try:
            os.dup2(stdout_tmp.fileno(), stdout_fd)
            os.dup2(stderr_tmp.fileno(), stderr_fd)
            yield stdout_tmp, stderr_tmp
        finally:
            os.dup2(saved_stdout, stdout_fd)
            os.dup2(saved_stderr, stderr_fd)
            os.close(saved_stdout)
            os.close(saved_stderr)


def read_native_output(handle) -> str:
    if handle is None:
        return ""
    handle.seek(0)
    return handle.read().decode("utf-8", errors="replace").strip()


def main() -> int:
    parser = argparse.ArgumentParser(prog="inkbite-ocr-helper")
    parser.add_argument("--self-test", action="store_true", dest="self_test")
    parser.add_argument("--backend", default="cpu")
    args = parser.parse_args()

    if not args.self_test:
        parser.error("only --self-test is currently supported")

    captured_stdout = io.StringIO()
    captured_stderr = io.StringIO()
    native_stdout = None
    native_stderr = None
    previous_disable = logging.root.manager.disable
    paddlex_logger = logging.getLogger("paddlex")
    previous_paddlex_disabled = paddlex_logger.disabled
    previous_paddlex_level = paddlex_logger.level
    previous_paddlex_warning = paddlex_logging.warning
    try:
        logging.disable(logging.CRITICAL)
        paddlex_logger.disabled = True
        paddlex_logger.setLevel(logging.CRITICAL + 1)
        paddlex_logging.warning = lambda *args, **kwargs: None
        with capture_native_output() as (native_stdout, native_stderr):
            with contextlib.redirect_stdout(captured_stdout), contextlib.redirect_stderr(captured_stderr):
                build_ocr(args.backend)
    except Exception:
        details = captured_stderr.getvalue().strip()
        if not details:
            details = read_native_output(native_stderr)
        if not details:
            details = captured_stdout.getvalue().strip()
        if not details:
            details = read_native_output(native_stdout)
        if details:
            print(details, file=sys.stderr)
        raise
    finally:
        paddlex_logging.warning = previous_paddlex_warning
        paddlex_logger.disabled = previous_paddlex_disabled
        paddlex_logger.setLevel(previous_paddlex_level)
        logging.disable(previous_disable)

    print(json.dumps({"status": "ok", "provider": "paddleocr", "backend": "cpu"}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
`
