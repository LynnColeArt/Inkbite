package components

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunCommandStreamsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash-based command streaming test is not applicable on windows")
	}

	var progress bytes.Buffer
	err := runCommand(&progress, "bash", "-lc", "printf 'hello stdout\\n'; printf 'hello stderr\\n' >&2")
	if err != nil {
		t.Fatalf("runCommand() error = %v", err)
	}

	got := progress.String()
	if !strings.Contains(got, "hello stdout") {
		t.Fatalf("expected stdout in streamed output, got %q", got)
	}
	if !strings.Contains(got, "hello stderr") {
		t.Fatalf("expected stderr in streamed output, got %q", got)
	}
}

func TestWritePaddleWrapperConfiguresQuietSelfTestEnvironment(t *testing.T) {
	dir := t.TempDir()
	helperPath := filepath.Join(dir, "inkbite-ocr-helper")
	scriptPath := filepath.Join(dir, "helper.py")

	if err := os.WriteFile(scriptPath, []byte("print('ok')\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := writePaddleWrapper(helperPath, "/tmp/python", scriptPath, "/tmp/cache", "/tmp/home")
	if err != nil {
		t.Fatalf("writePaddleWrapper() error = %v", err)
	}

	data, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, `export PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK="True"`) {
		t.Fatalf("expected wrapper to disable paddle model source checks, got %q", got)
	}
	if !strings.Contains(got, `export PADDLE_PDX_MODEL_SOURCE="BOS"`) {
		t.Fatalf("expected wrapper to pin paddle model source, got %q", got)
	}
}

func TestPaddleHelperScriptSilencesInitializationNoise(t *testing.T) {
	if !strings.Contains(paddleHelperScript, "contextlib.redirect_stdout") {
		t.Fatalf("expected helper script to redirect stdout during self-test")
	}
	if !strings.Contains(paddleHelperScript, "contextlib.redirect_stderr") {
		t.Fatalf("expected helper script to redirect stderr during self-test")
	}
	if !strings.Contains(paddleHelperScript, "capture_native_output") {
		t.Fatalf("expected helper script to capture native output during self-test")
	}
	if !strings.Contains(paddleHelperScript, "logging.disable(logging.CRITICAL)") {
		t.Fatalf("expected helper script to suppress paddle logging during self-test")
	}
	if !strings.Contains(paddleHelperScript, `logging.getLogger("paddlex")`) {
		t.Fatalf("expected helper script to target the paddlex logger during self-test")
	}
	if !strings.Contains(paddleHelperScript, "paddlex_logging.warning = lambda *args, **kwargs: None") {
		t.Fatalf("expected helper script to patch paddlex warning logging during self-test")
	}
}
