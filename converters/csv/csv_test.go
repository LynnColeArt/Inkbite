package csvconv

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestCSVConversion(t *testing.T) {
	input := "name,age\nAda,30\nGrace,31\n"

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader([]byte(input)), inkbite.StreamInfo{
		Extension: ".csv",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	for _, fragment := range []string{
		"| name | age |",
		"| --- | --- |",
		"| Ada | 30 |",
		"| Grace | 31 |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}
