package pdfconv

import (
	"strings"
	"testing"
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
