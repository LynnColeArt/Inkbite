package zipconv_test

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

func TestZIPConversion(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeZipFile(t, zw, "notes.txt", "hello from zip")
	writeZipFile(t, zw, "data.csv", "name,age\nAda,30\n")

	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	result, err := engine.Convert(context.Background(), buf.Bytes(), &inkbite.StreamInfo{
		Extension: ".zip",
		Filename:  "bundle.zip",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	for _, fragment := range []string{
		"Content from zip file `bundle.zip`",
		"## File: notes.txt",
		"hello from zip",
		"## File: data.csv",
		"| name | age |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}

func writeZipFile(t *testing.T, zw *zip.Writer, name string, content string) {
	t.Helper()

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", name, err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("Write(%q) error = %v", name, err)
	}
}
