package inkbite_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

func TestConvertFileURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello from file uri"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	result, err := engine.ConvertURI(context.Background(), "file://"+path, nil, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("ConvertURI() error = %v", err)
	}
	if result.Markdown != "hello from file uri" {
		t.Fatalf("expected file contents, got %q", result.Markdown)
	}
}
