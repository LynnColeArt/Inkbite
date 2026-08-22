package zipconv_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
	zipconv "github.com/LynnColeArt/Inkbite/converters/zip"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
)

func TestZIPConversionFixture(t *testing.T) {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	result, err := engine.Convert(context.Background(), testutil.BuildZipFixture(t, filepath.Join("testdata", "simple")), &inkbite.StreamInfo{
		Extension: ".zip",
		Filename:  "bundle.zip",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	want := "Content from zip file `bundle.zip`\n\n" +
		"## File: data.csv\n\n| name | age |\n| --- | --- |\n| Ada | 30 |\n\n" +
		"## File: notes.txt\n\nhello from zip"
	if result.Markdown != want {
		t.Fatalf("Convert() markdown = %q, want exact fixture %q", result.Markdown, want)
	}

	for _, fragment := range []string{
		"Content from zip file `bundle.zip`",
		"## File: notes.txt",
		"hello from zip",
		"## File: data.csv",
		"| name | age |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}

func BenchmarkZIPConversionFixture(b *testing.B) {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	archive := testutil.BuildZipFixture(b, filepath.Join("testdata", "simple"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Convert(context.Background(), archive, &inkbite.StreamInfo{
			Extension: ".zip",
			Filename:  "bundle.zip",
		}, inkbite.ConvertOptions{}); err != nil {
			b.Fatalf("Convert() error = %v", err)
		}
	}
}

func TestZIPRejectsTooManyEntries(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 300; i++ {
		writeZipFile(t, zw, fmt.Sprintf("note-%03d.txt", i), "hello")
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	_, err := engine.Convert(context.Background(), buf.Bytes(), &inkbite.StreamInfo{
		Extension: ".zip",
		Filename:  "many.zip",
	}, inkbite.ConvertOptions{})
	if err == nil {
		t.Fatal("expected archive entry limit error")
	}
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("expected entry limit error, got %v", err)
	}
}

func TestZIPRejectsInvalidArchive(t *testing.T) {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	_, err := engine.Convert(context.Background(), []byte("not a zip archive\x00"), &inkbite.StreamInfo{
		Extension: ".zip",
		Filename:  "broken.zip",
	}, inkbite.ConvertOptions{})
	if err == nil {
		t.Fatal("expected invalid archive error")
	}
}

func TestZIPRejectsDeeplyNestedArchives(t *testing.T) {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	archive := nestedArchive(t, 6)
	_, err := engine.Convert(context.Background(), archive, &inkbite.StreamInfo{
		Extension: ".zip",
		Filename:  "nested.zip",
	}, inkbite.ConvertOptions{})
	if err == nil {
		t.Fatal("expected recursion depth error")
	}
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("expected recursion depth error, got %v", err)
	}
}

func TestZIPDirectLegacyConversionUsesBoundedAuthorityAndLabels(t *testing.T) {
	t.Parallel()

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	converter := zipconv.New(engine)
	archive := buildZIP(t, []zipMember{{name: "note.txt", body: []byte("direct")}})
	tests := []struct {
		name string
		info inkbite.StreamInfo
		want string
	}{
		{name: "URL", info: inkbite.StreamInfo{Extension: ".zip", URL: "https://example.test/bundle.zip"}, want: "https://example.test/bundle.zip"},
		{name: "local path", info: inkbite.StreamInfo{Extension: ".zip", LocalPath: "fixtures/bundle.zip"}, want: "fixtures/bundle.zip"},
		{name: "default", info: inkbite.StreamInfo{Extension: ".zip"}, want: "archive.zip"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := converter.Convert(context.Background(), bytes.NewReader(archive), tc.info, inkbite.ConvertOptions{})
			if err != nil {
				t.Fatalf("Convert() error = %v", err)
			}
			if !strings.Contains(result.Markdown, "`"+tc.want+"`") || !strings.Contains(result.Markdown, "direct") {
				t.Fatalf("Convert() markdown = %q, want label %q and member", result.Markdown, tc.want)
			}
		})
	}

	_, err := converter.Convert(context.Background(), failingZIPReadSeeker{}, inkbite.StreamInfo{Extension: ".zip"}, inkbite.ConvertOptions{})
	if !errors.Is(err, inkbite.ErrIntegrityFailure) {
		t.Fatalf("Convert() seek error = %v, want integrity failure", err)
	}
}

type failingZIPReadSeeker struct{}

func (failingZIPReadSeeker) Read([]byte) (int, error) { return 0, io.EOF }
func (failingZIPReadSeeker) Seek(int64, int) (int64, error) {
	return 0, errors.New("seek failed")
}

func nestedArchive(t *testing.T, depth int) []byte {
	t.Helper()

	if depth <= 1 {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		writeZipFile(t, zw, "leaf.txt", "bottom")
		if err := zw.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		return buf.Bytes()
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writer, err := zw.Create(fmt.Sprintf("nested-%d.zip", depth-1))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := writer.Write(nestedArchive(t, depth-1)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}

func writeZipFile(t *testing.T, zw *zip.Writer, name string, content string) {
	t.Helper()

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", name, err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("Write(%q) error = %v", name, err)
	}
}
