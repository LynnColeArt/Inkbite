package xlsxconv

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/LynnColeArt/Inkbite"
)

func TestXLSXConversion(t *testing.T) {
	workbook := excelize.NewFile()
	defer func() {
		_ = workbook.Close()
	}()

	workbook.SetSheetName("Sheet1", "People")
	if err := workbook.SetSheetRow("People", "A1", &[]string{"name", "age"}); err != nil {
		t.Fatalf("SetSheetRow() error = %v", err)
	}
	if err := workbook.SetSheetRow("People", "A2", &[]string{"Ada", "30"}); err != nil {
		t.Fatalf("SetSheetRow() error = %v", err)
	}

	var buf bytes.Buffer
	if err := workbook.Write(&buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(buf.Bytes()), inkbite.StreamInfo{
		Extension: ".xlsx",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	for _, fragment := range []string{
		"## People",
		"| name | age |",
		"| Ada | 30 |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}
