package htmlconv

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestHTMLConversion(t *testing.T) {
	input := `<html><head><title>Example</title><style>body{}</style><script>alert(1)</script></head><body><h1>Hello</h1><p>World</p></body></html>`

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader([]byte(input)), inkbite.StreamInfo{
		MIMEType: "text/html",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if result.Title != "Example" {
		t.Fatalf("expected title Example, got %q", result.Title)
	}
	if !strings.Contains(result.Markdown, "# Hello") {
		t.Fatalf("expected heading in markdown, got %q", result.Markdown)
	}
	if strings.Contains(result.Markdown, "alert") {
		t.Fatalf("expected script content to be removed, got %q", result.Markdown)
	}
}
