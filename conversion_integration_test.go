package inkbite_test

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}

	result, err := engine.ConvertURI(context.Background(), (&url.URL{Scheme: "file", Path: uriPath}).String(), nil, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("ConvertURI() error = %v", err)
	}
	if result.Markdown != "hello from file uri" {
		t.Fatalf("expected file contents, got %q", result.Markdown)
	}
}
