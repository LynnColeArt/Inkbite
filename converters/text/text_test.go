package text

import (
	"bytes"
	"context"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestConverterAcceptsUTF8TextWithoutHints(t *testing.T) {
	converter := New()
	ok := converter.Accepts(context.Background(), bytes.NewReader([]byte("hello")), inkbite.StreamInfo{}, inkbite.ConvertOptions{})
	if !ok {
		t.Fatal("expected converter to accept plain UTF-8 text")
	}
}

func TestConverterDecodesHintedCharset(t *testing.T) {
	converter := New()
	reader := bytes.NewReader([]byte{0x63, 0x61, 0x66, 0xe9})

	result, err := converter.Convert(context.Background(), reader, inkbite.StreamInfo{
		Charset: "windows-1252",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if result.Markdown != "café" {
		t.Fatalf("expected café, got %q", result.Markdown)
	}
}
