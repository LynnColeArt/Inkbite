package xlsconv

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
)

func TestXLSConversionFixture(t *testing.T) {
	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(testutil.LoadFixture(t, filepath.Join("testdata", "simple.xls"))), inkbite.StreamInfo{
		Extension: ".xls",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	for _, fragment := range []string{
		"## Table",
		"| Code | Name | Description |",
		"| code1 | name1 | description1 |",
		"| code11 | name11 | description11 |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}

func TestXLSQualityFixturePreservesDatesAndNumbers(t *testing.T) {
	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(testutil.LoadFixture(t, filepath.Join("testdata", "quality.xls"))), inkbite.StreamInfo{
		Extension: ".xls",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	for _, fragment := range []string{
		"## Summary",
		"| Item | Value |",
		"| Revenue | 1234.5 |",
		"| As Of | 2024-05-06 |",
		"## Notes",
		"| Status | Ready |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}

	if strings.Contains(result.Markdown, "1653-06-08T00:25:26Z") {
		t.Fatalf("unexpected timestamp-like numeric rendering: %q", result.Markdown)
	}
}

func TestXLSRejectsMalformedWorkbook(t *testing.T) {
	converter := New()
	_, err := converter.Convert(context.Background(), bytes.NewReader([]byte("not an xls\x00")), inkbite.StreamInfo{
		Extension: ".xls",
	}, inkbite.ConvertOptions{})
	if err == nil {
		t.Fatal("expected malformed xls error")
	}
}
