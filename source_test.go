package inkbite

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestFileURIToPath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "unix path",
			raw:  "file:///tmp/hello.txt",
			want: "/tmp/hello.txt",
		},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			raw  string
			want string
		}{
			name: "windows drive path",
			raw:  "file:///C:/Users/test/hello.txt",
			want: "C:/Users/test/hello.txt",
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := url.Parse(test.raw)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			got, err := fileURIToPath(parsed)
			if err != nil {
				t.Fatalf("fileURIToPath() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestResolveURIRemoteHTTPWithinLimit(t *testing.T) {
	engine := New(WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/hello.txt" {
				t.Fatalf("expected /hello.txt path, got %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
				Body:          io.NopCloser(strings.NewReader("hello remote")),
				ContentLength: int64(len("hello remote")),
			}, nil
		}),
	}))
	resolved, err := engine.resolveURI(context.Background(), "https://example.com/hello.txt", nil, ConvertOptions{
		EnableHTTP:   true,
		MaxHTTPBytes: 64,
	})
	if err != nil {
		t.Fatalf("resolveURI() error = %v", err)
	}

	data, err := io.ReadAll(resolved.reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "hello remote" {
		t.Fatalf("expected hello remote, got %q", string(data))
	}
	if resolved.info.MIMEType != "text/plain" {
		t.Fatalf("expected text/plain MIME type, got %q", resolved.info.MIMEType)
	}
	if resolved.info.Filename != "hello.txt" {
		t.Fatalf("expected hello.txt filename, got %q", resolved.info.Filename)
	}
}

func TestResolveURIRemoteHTTPRejectsOversizedBody(t *testing.T) {
	engine := New(WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/too-large.txt" {
				t.Fatalf("expected /too-large.txt path, got %q", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("a", 9))),
			}, nil
		}),
	}))
	_, err := engine.resolveURI(context.Background(), "https://example.com/too-large.txt", nil, ConvertOptions{
		EnableHTTP:   true,
		MaxHTTPBytes: 8,
	})
	if !errors.Is(err, ErrRemoteTooLarge) {
		t.Fatalf("expected ErrRemoteTooLarge, got %v", err)
	}
}
