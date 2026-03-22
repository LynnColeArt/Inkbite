package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunConvertDefaultPathBehavior(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{path}, &stdout, &stderr, runtimeDeps{version: "test"})
	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != "hello world\n" {
		t.Fatalf("expected converted stdout, got %q", got)
	}
}

func TestComponentsListEmpty(t *testing.T) {
	t.Setenv("INKBITE_HOME", t.TempDir())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"components", "list"}, &stdout, &stderr, runtimeDeps{version: "test"})
	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no managed components installed") {
		t.Fatalf("expected empty components message, got %q", stdout.String())
	}
}

func TestInstallOCRAndDoctor(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("INKBITE_HOME", baseDir)

	executablePath := filepath.Join(t.TempDir(), "inkbite")
	if err := os.WriteFile(executablePath, []byte("fake executable"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	deps := runtimeDeps{
		version:        "v0.1.0-test",
		executablePath: executablePath,
		helperSelfTest: func(helperPath string, backend string) error {
			if backend != "cpu" {
				t.Fatalf("expected cpu backend, got %q", backend)
			}
			if _, err := os.Stat(helperPath); err != nil {
				t.Fatalf("expected helper to exist: %v", err)
			}
			return nil
		},
	}

	var installOut bytes.Buffer
	var installErr bytes.Buffer
	code := run([]string{"install", "ocr", "--dir", baseDir}, &installOut, &installErr, deps)
	if code != 0 {
		t.Fatalf("install code = %d, stderr = %q", code, installErr.String())
	}
	if !strings.Contains(installOut.String(), "installed managed ocr component") {
		t.Fatalf("expected install output, got %q", installOut.String())
	}

	var listOut bytes.Buffer
	var listErr bytes.Buffer
	code = run([]string{"components", "list"}, &listOut, &listErr, deps)
	if code != 0 {
		t.Fatalf("components code = %d, stderr = %q", code, listErr.String())
	}
	if !strings.Contains(listOut.String(), "ocr\tbackend=cpu\tversion=v0.1.0-test") {
		t.Fatalf("expected components list output, got %q", listOut.String())
	}

	var doctorOut bytes.Buffer
	var doctorErr bytes.Buffer
	code = run([]string{"doctor"}, &doctorOut, &doctorErr, deps)
	if code != 0 {
		t.Fatalf("doctor code = %d, stderr = %q", code, doctorErr.String())
	}
	if !strings.Contains(doctorOut.String(), "ocr: installed") || !strings.Contains(doctorOut.String(), "status: ok") {
		t.Fatalf("expected healthy doctor output, got %q", doctorOut.String())
	}

	var configOut bytes.Buffer
	var configErr bytes.Buffer
	code = run([]string{"config", "show"}, &configOut, &configErr, deps)
	if code != 0 {
		t.Fatalf("config code = %d, stderr = %q", code, configErr.String())
	}

	var cfg struct {
		OCR struct {
			Enabled   bool   `json:"enabled"`
			Backend   string `json:"backend"`
			Component string `json:"component"`
			Version   string `json:"version"`
		} `json:"ocr"`
	}
	if err := json.Unmarshal(configOut.Bytes(), &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !cfg.OCR.Enabled || cfg.OCR.Backend != "cpu" || cfg.OCR.Version != "v0.1.0-test" {
		t.Fatalf("unexpected config output: %s", configOut.String())
	}
}

func TestInstallOCRRejectsUnsupportedBackend(t *testing.T) {
	baseDir := t.TempDir()
	executablePath := filepath.Join(t.TempDir(), "inkbite")
	if err := os.WriteFile(executablePath, []byte("fake executable"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"install", "ocr", "--dir", baseDir, "--backend", "cuda"}, &stdout, &stderr, runtimeDeps{
		version:        "test",
		executablePath: executablePath,
		helperSelfTest: func(helperPath string, backend string) error { return nil },
	})
	if code == 0 {
		t.Fatal("expected non-zero exit code for unsupported backend")
	}
	if !strings.Contains(stderr.String(), `ocr backend "cuda" is not yet available`) {
		t.Fatalf("expected unsupported backend error, got %q", stderr.String())
	}
}

func TestOCRHelperSelfTestCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"__ocr_helper", "--self-test", "--backend", "cpu"}, &stdout, &stderr, runtimeDeps{version: "test"})
	if code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status":"ok"`) || !strings.Contains(stdout.String(), `"backend":"cpu"`) {
		t.Fatalf("expected helper self-test output, got %q", stdout.String())
	}
}
