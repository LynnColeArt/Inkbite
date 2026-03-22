package docxconv

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

func TestDOCXConversionFixture(t *testing.T) {
	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))), inkbite.StreamInfo{
		Extension: ".docx",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if result.Title != "Sample Doc" {
		t.Fatalf("expected title Sample Doc, got %q", result.Title)
	}

	for _, fragment := range []string{
		"# Sample Doc",
		"Hello [world](https://example.com)",
		"| Name | Age |",
		"| Ada | 30 |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}

func TestDOCXRejectsMissingDocumentXML(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile(t, zw, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`)
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	converter := New()
	_, err := converter.Convert(context.Background(), bytes.NewReader(buf.Bytes()), inkbite.StreamInfo{
		Extension: ".docx",
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
