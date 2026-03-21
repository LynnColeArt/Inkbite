package inkbite

import (
	"context"
	"io"
	"strings"
	"testing"
)

type stubConverter struct {
	name     string
	priority float64
	accepts  bool
	markdown string
}

func (s stubConverter) Name() string {
	return s.name
}

func (s stubConverter) Priority() float64 {
	return s.priority
}

func (s stubConverter) Accepts(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) bool {
	return s.accepts
}

func (s stubConverter) Convert(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) (Result, error) {
	return Result{Markdown: s.markdown}, nil
}

func TestEnginePrefersLowerPriorityValue(t *testing.T) {
	engine := New()
	engine.RegisterConverter(stubConverter{
		name:     "slow",
		priority: 50,
		accepts:  true,
		markdown: "slow",
	})
	engine.RegisterConverter(stubConverter{
		name:     "fast",
		priority: 10,
		accepts:  true,
		markdown: "fast",
	})

	result, err := engine.Convert(context.Background(), []byte("hello"), nil, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if result.Markdown != "fast" {
		t.Fatalf("expected lower priority converter result, got %q", result.Markdown)
	}
}

func TestEngineReturnsUnsupportedFormat(t *testing.T) {
	engine := New()

	_, err := engine.Convert(context.Background(), []byte("hello"), nil, ConvertOptions{})
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
}
