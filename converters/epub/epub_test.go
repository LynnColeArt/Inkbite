package epubconv

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
)

func TestEPUBConversionFixture(t *testing.T) {
	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))), inkbite.StreamInfo{
		Extension: ".epub",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	for _, fragment := range []string{
		"**Title:** Sample Book",
		"**Authors:** Test Author",
		"# Chapter 1",
		"Hello **EPUB**",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}

func TestEPUBRejectsMissingContainerXML(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile(t, zw, "OPS/content.opf", `<?xml version="1.0" encoding="UTF-8"?><package/>`)
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	converter := New()
	_, err := converter.Convert(context.Background(), bytes.NewReader(buf.Bytes()), inkbite.StreamInfo{
		Extension: ".epub",
	}, inkbite.ConvertOptions{})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	var unsupported inkbite.UnsupportedFormatError
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected unsupported format error, got %v", err)
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
