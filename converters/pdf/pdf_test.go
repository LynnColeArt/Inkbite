package pdfconv

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestLayoutToMarkdownConvertsTableBlocks(t *testing.T) {
	input := strings.Join([]string{
		"Inventory Summary",
		"",
		"Product Code    Location    Qty    Status",
		"SKU-1           A-01        10     OK",
		"SKU-2           B-07        5      HOLD",
		"",
		"Recommendations follow.",
	}, "\n")

	got := layoutToMarkdown(input)
	for _, fragment := range []string{
		"Inventory Summary",
		"| Product Code | Location | Qty | Status |",
		"| SKU-1 | A-01 | 10 | OK |",
		"| SKU-2 | B-07 | 5 | HOLD |",
		"Recommendations follow.",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, got)
		}
	}
}

func TestLayoutToMarkdownLeavesParagraphsAlone(t *testing.T) {
	input := "This is a paragraph\nwith wrapped lines\nand no tabular structure."

	got := layoutToMarkdown(input)
	if strings.Contains(got, "| --- |") {
		t.Fatalf("expected no markdown table, got %q", got)
	}
	if !strings.Contains(got, "This is a paragraph") {
		t.Fatalf("expected paragraph text, got %q", got)
	}
}

func TestConvertUsesPureGoBackendWithoutPATH(t *testing.T) {
	t.Setenv("PATH", "")

	converter := New()
	result, err := converter.Convert(
		context.Background(),
		bytes.NewReader(makeSimplePDF("Hello PDF")),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "auto"},
	)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !strings.Contains(result.Markdown, "Hello PDF") {
		t.Fatalf("expected extracted PDF text, got %q", result.Markdown)
	}
}

func TestChooseExtractorRejectsExternalBackendName(t *testing.T) {
	converter := New()

	_, err := converter.chooseExtractor("pdftotext")
	if err == nil {
		t.Fatal("expected error for unsupported external backend")
	}
	if !strings.Contains(err.Error(), "unknown PDF extractor") {
		t.Fatalf("expected unknown backend error, got %v", err)
	}
}

func makeSimplePDF(text string) []byte {
	stream := "BT\n/F1 24 Tf\n100 100 Td\n(" + escapePDFString(text) + ") Tj\nET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var doc bytes.Buffer
	doc.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects)+1)
	for idx, object := range objects {
		offsets[idx+1] = doc.Len()
		fmt.Fprintf(&doc, "%d 0 obj\n%s\nendobj\n", idx+1, object)
	}

	xrefOffset := doc.Len()
	fmt.Fprintf(&doc, "xref\n0 %d\n", len(objects)+1)
	doc.WriteString("0000000000 65535 f \n")
	for idx := 1; idx <= len(objects); idx++ {
		fmt.Fprintf(&doc, "%010d 00000 n \n", offsets[idx])
	}
	fmt.Fprintf(&doc, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)

	return doc.Bytes()
}

func escapePDFString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "(", `\(`)
	value = strings.ReplaceAll(value, ")", `\)`)
	return value
}
