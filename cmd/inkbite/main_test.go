package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LynnColeArt/Inkbite"
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
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

func TestRunConvertFailureSnapshots(t *testing.T) {
	dir := t.TempDir()
	unsupportedPath := filepath.Join(dir, "unsupported.bin")
	if err := os.WriteFile(unsupportedPath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	malformedPath := filepath.Join(dir, "malformed.epub")
	if err := os.WriteFile(malformedPath, []byte{'P', 'K', 3, 4, 0, 0}, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "unsupported", args: []string{unsupportedPath}, wantStderr: "unsupported format\n"},
		{name: "malformed", args: []string{malformedPath}, wantStderr: "converter-run: malformed input\n"},
		{name: "remote disabled", args: []string{"https://example.test/document"}, wantStderr: "remote fetching is disabled\n"},
		{name: "remote lacks caller transport", args: []string{"--http", "https://example.test/document"}, wantStderr: "remote-read: ingestion integrity failure\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run(test.args, &stdout, &stderr, runtimeDeps{version: "test"})
			if code != 1 {
				t.Fatalf("run() code = %d, want 1", code)
			}
			if got := stdout.String(); got != "" {
				t.Fatalf("run() stdout = %q, want empty", got)
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("run() stderr = %q, want %q", got, test.wantStderr)
			}
		})
	}
}

func TestRunConvertDocumentedOptions(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.dat")
	if err := os.WriteFile(inputPath, []byte("documented options"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Run("type hints", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--extension", ".xml", "--mime-type", "text/xml", "--charset", "utf-8", inputPath}, &stdout, &stderr, runtimeDeps{version: "test"})
		if code != 0 || stdout.String() != "documented options\n" || stderr.String() != "" {
			t.Fatalf("hinted run = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
	})

	t.Run("output file", func(t *testing.T) {
		outputPath := filepath.Join(dir, "output.md")
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"-o", outputPath, inputPath}, &stdout, &stderr, runtimeDeps{version: "test"})
		if code != 0 || stdout.String() != "" || stderr.String() != "" {
			t.Fatalf("output run = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if got, want := string(content), "documented options"; got != want {
			t.Fatalf("output file = %q, want %q", got, want)
		}
	})

	t.Run("list formats", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := run([]string{"--list-formats"}, &stdout, &stderr, runtimeDeps{version: "test"})
		want := "ipynb\t(priority 10)\n" +
			"docx\t(priority 12)\n" +
			"pptx\t(priority 13)\n" +
			"pdf\t(priority 14)\n" +
			"xlsx\t(priority 15)\n" +
			"xls\t(priority 16)\n" +
			"csv\t(priority 20)\n" +
			"epub\t(priority 25)\n" +
			"rss\t(priority 30)\n" +
			"zip\t(priority 35)\n" +
			"html\t(priority 40)\n" +
			"text\t(priority 100)\n"
		if code != 0 || stdout.String() != want || stderr.String() != "" {
			t.Fatalf("list formats = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
	})
}

func TestRunConvertRejectsInvalidTimeout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"convert", "--timeout", "not-a-duration"}, &stdout, &stderr, runtimeDeps{version: "test"})
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "invalid timeout") {
		t.Fatalf("expected invalid timeout error, got %q", stderr.String())
	}
}

func TestRunConversionReturnsContextError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	cancel()

	release := make(chan struct{})
	result, err := runConversion(ctx, func(context.Context) (inkbite.Result, error) {
		<-release
		return inkbite.Result{Markdown: "late"}, nil
	})
	close(release)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if result != (inkbite.Result{}) {
		t.Fatalf("cancelled runConversion() result = %+v, want zero Result", result)
	}
}

func TestRunConvertCancellationSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancelled.txt")
	if err := os.WriteFile(path, bytes.Repeat([]byte("cancel me\n"), 1024), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"--timeout", "1h", path}, &stdout, &stderr, runtimeDeps{
		version: "test",
		timeoutContext: func(parent context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
			return context.WithDeadline(parent, time.Unix(0, 0))
		},
	})
	if code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("run() stdout = %q, want empty", got)
	}
	if got, want := stderr.String(), "context deadline exceeded\n"; got != want {
		t.Fatalf("run() stderr = %q, want %q", got, want)
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
		helperSelfTest: func(helperPath string, provider string, backend string) error {
			if provider != "builtin" {
				t.Fatalf("expected builtin provider, got %q", provider)
			}
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
	if !strings.Contains(listOut.String(), "ocr\tprovider=builtin\tbackend=cpu\tversion=v0.1.0-test") {
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
			Provider  string `json:"provider"`
			Backend   string `json:"backend"`
			Component string `json:"component"`
			Version   string `json:"version"`
		} `json:"ocr"`
	}
	if err := json.Unmarshal(configOut.Bytes(), &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !cfg.OCR.Enabled || cfg.OCR.Provider != "builtin" || cfg.OCR.Backend != "cpu" || cfg.OCR.Version != "v0.1.0-test" {
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
		helperSelfTest: func(helperPath string, provider string, backend string) error { return nil },
	})
	if code == 0 {
		t.Fatal("expected non-zero exit code for unsupported backend")
	}
	if !strings.Contains(stderr.String(), `ocr backend "cuda" is not yet available`) {
		t.Fatalf("expected unsupported backend error, got %q", stderr.String())
	}
}

func TestInstallOCRRejectsUnknownProvider(t *testing.T) {
	baseDir := t.TempDir()
	executablePath := filepath.Join(t.TempDir(), "inkbite")
	if err := os.WriteFile(executablePath, []byte("fake executable"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"install", "ocr", "--dir", baseDir, "--provider", "mystery"}, &stdout, &stderr, runtimeDeps{
		version:        "test",
		executablePath: executablePath,
		helperSelfTest: func(helperPath string, provider string, backend string) error { return nil },
	})
	if code == 0 {
		t.Fatal("expected non-zero exit code for unknown provider")
	}
	if !strings.Contains(stderr.String(), `unknown ocr provider "mystery"`) {
		t.Fatalf("expected unknown provider error, got %q", stderr.String())
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
