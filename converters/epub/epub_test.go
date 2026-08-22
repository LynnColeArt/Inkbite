package epubconv

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
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
	want := "**Title:** Sample Book\n**Authors:** Test Author\n**Language:** en\n\n# Chapter 1\n\nHello **EPUB**"
	if result.Markdown != want {
		t.Fatalf("Convert() markdown = %q, want exact fixture %q", result.Markdown, want)
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

func TestEPUBConverterSurfaceAndTypedSeekFailure(t *testing.T) {
	t.Parallel()

	converter := New()
	if converter.Priority() != priority {
		t.Fatalf("Priority() = %v, want %v", converter.Priority(), priority)
	}
	for _, info := range []inkbite.StreamInfo{
		{Extension: ".epub"},
		{MIMEType: "application/epub+zip"},
	} {
		if !converter.Accepts(context.Background(), bytes.NewReader(nil), info, inkbite.ConvertOptions{}) {
			t.Fatalf("Accepts(%#v) = false", info)
		}
	}
	if converter.Accepts(context.Background(), bytes.NewReader(nil), inkbite.StreamInfo{Extension: ".txt"}, inkbite.ConvertOptions{}) {
		t.Fatal("Accepts(.txt) = true")
	}
	_, err := converter.Convert(context.Background(), failingEPUBReadSeeker{}, inkbite.StreamInfo{Extension: ".epub"}, inkbite.ConvertOptions{})
	if !errors.Is(err, inkbite.ErrIntegrityFailure) {
		t.Fatalf("Convert() seek error = %v, want integrity failure", err)
	}
}

type failingEPUBReadSeeker struct{}

func (failingEPUBReadSeeker) Read([]byte) (int, error) { return 0, io.EOF }
func (failingEPUBReadSeeker) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek failed")
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
