package pdfconv

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
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
		bytes.NewReader(testutil.LoadFixture(t, filepath.Join("testdata", "simple.pdf"))),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "auto"},
	)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !strings.Contains(result.Markdown, "Hello PDF") {
		if !strings.Contains(result.Markdown, "Fixture PDF") {
			t.Fatalf("expected extracted PDF text, got %q", result.Markdown)
		}
	}
}

func TestPDFConversionFixture(t *testing.T) {
	converter := New()
	result, err := converter.Convert(
		context.Background(),
		bytes.NewReader(testutil.LoadFixture(t, filepath.Join("testdata", "simple.pdf"))),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "purego"},
	)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !strings.Contains(result.Markdown, "Fixture PDF") {
		t.Fatalf("expected extracted PDF text, got %q", result.Markdown)
	}
}

func TestPDFConversionEmbedsImageXObjects(t *testing.T) {
	converter := New()
	result, err := converter.Convert(
		context.Background(),
		bytes.NewReader(makeImageXObjectPDF("Page with image")),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "purego", KeepDataURIs: true},
	)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	for _, want := range []string{
		"Page with image",
		"## PDF Images",
		"![PDF image page 1 Im1 1x1",
		"](data:image/png;base64,",
		"| 1 | Im1 |",
	} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("expected %q in markdown, got %q", want, result.Markdown)
		}
	}
}

func TestPDFConversionSkipsImagesWithoutKeepDataURIs(t *testing.T) {
	converter := New()
	result, err := converter.Convert(
		context.Background(),
		bytes.NewReader(makeImageXObjectPDF("Page with default image handling")),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "purego"},
	)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if strings.Contains(result.Markdown, "data:image/") {
		t.Fatalf("expected default conversion to avoid image data URIs, got %q", result.Markdown)
	}
	if !strings.Contains(result.Markdown, "Page with default image handling") {
		t.Fatalf("expected text extraction to remain intact, got %q", result.Markdown)
	}
}

func TestPDFConversionEmbedsImageMasks(t *testing.T) {
	converter := New()
	result, err := converter.Convert(
		context.Background(),
		bytes.NewReader(makeImageMaskPDF("Page with image mask")),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "purego", KeepDataURIs: true},
	)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	for _, want := range []string{
		"Page with image mask",
		"## PDF Images",
		"![PDF image page 1 ImMask 1x1 DeviceGray image mask",
		"](data:image/png;base64,",
		"| 1 | ImMask |",
	} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("expected %q in markdown, got %q", want, result.Markdown)
		}
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

func TestConvertRejectsMalformedPDF(t *testing.T) {
	converter := New()

	_, err := converter.Convert(
		context.Background(),
		bytes.NewReader([]byte("%PDF-1.4\nnot actually a PDF\n%%EOF")),
		inkbite.StreamInfo{Extension: ".pdf"},
		inkbite.ConvertOptions{PDFBackend: "auto"},
	)
	if err == nil {
		t.Fatal("expected malformed PDF error")
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

func makeImageXObjectPDF(text string) []byte {
	textStream := "BT\n/F1 24 Tf\n100 100 Td\n(" + escapePDFString(text) + ") Tj\nET\nq\n1 0 0 1 20 20 cm\n/Im1 Do\nQ"
	imageStream := "0"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> /XObject << /Im1 6 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(textStream), textStream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8 /Length %d >>\nstream\n%s\nendstream", len(imageStream), imageStream),
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

func makeImageMaskPDF(text string) []byte {
	textStream := "BT\n/F1 24 Tf\n100 100 Td\n(" + escapePDFString(text) + ") Tj\nET\nq\n1 0 0 1 20 20 cm\n/ImMask Do\nQ"
	imageStream := string([]byte{0x80})
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> /XObject << /ImMask 6 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(textStream), textStream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 1 /Height 1 /ImageMask true /BitsPerComponent 1 /Length %d >>\nstream\n%s\nendstream", len(imageStream), imageStream),
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
