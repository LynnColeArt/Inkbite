package inkbite

import (
	"context"
	"errors"
	"testing"
)

func TestParseDataURIBase64(t *testing.T) {
	mediaType, attributes, data, err := parseDataURI("data:text/plain;charset=utf-8;base64,SGVsbG8=")
	if err != nil {
		t.Fatalf("parseDataURI() error = %v", err)
	}
	if mediaType != "text/plain" {
		t.Fatalf("expected text/plain, got %q", mediaType)
	}
	if attributes["charset"] != "utf-8" {
		t.Fatalf("expected utf-8 charset, got %q", attributes["charset"])
	}
	if string(data) != "Hello" {
		t.Fatalf("expected Hello, got %q", string(data))
	}
}

func TestRemoteHTTPDisabledByDefault(t *testing.T) {
	engine := New()

	_, err := engine.ConvertURI(context.Background(), "https://example.com", nil, ConvertOptions{})
	if !errors.Is(err, ErrRemoteDisabled) {
		t.Fatalf("expected ErrRemoteDisabled, got %v", err)
	}
}
