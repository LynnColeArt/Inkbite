package ingestion

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestFactOriginsMirrorV1Schema(t *testing.T) {
	t.Parallel()

	origins := factOrigins()
	encoded, err := json.Marshal(origins[:])
	if err != nil {
		t.Fatal(err)
	}
	const schemaLiteral = `["caller","source","sniff","converter"]`
	if string(encoded) != schemaLiteral {
		t.Fatalf("ordered FactOrigin vocabulary = %s, want schema literal %s", encoded, schemaLiteral)
	}
	for _, origin := range origins {
		if !validOrigin(origin) {
			t.Fatalf("enumerated origin %q is not accepted", origin)
		}
	}
}

func TestCanonicalLogicalNameAndArchivePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		archive bool
		want    string
		ok      bool
	}{
		{name: "logical name", input: " Brief.PDF ", want: "Brief.PDF", ok: true},
		{name: "archive path", input: "word/media/image-01.png", archive: true, want: "word/media/image-01.png", ok: true},
		{name: "nested safe encoding", input: "chapter%20one.txt", archive: true, want: "chapter%20one.txt", ok: true},
		{name: "empty", input: ""},
		{name: "nul", input: "safe\x00name"},
		{name: "control", input: "safe\nname"},
		{name: "outer control", input: "\nname\n"},
		{name: "logical slash", input: "dir/name.txt"},
		{name: "logical colon", input: "name:stream"},
		{name: "traversal", input: "../secret", archive: true},
		{name: "encoded traversal", input: "%252e%252e/secret", archive: true},
		{name: "encoded separator", input: "dir%252fname", archive: true},
		{name: "absolute", input: "/etc/passwd", archive: true},
		{name: "drive", input: "C:/secret", archive: true},
		{name: "UNC", input: "//server/share", archive: true},
		{name: "backslash", input: `dir\secret`, archive: true},
		{name: "empty segment", input: "dir//name", archive: true},
		{name: "dot segment", input: "dir/./name", archive: true},
		{name: "trailing-space segment", input: "dir/.. /name", archive: true},
		{name: "trailing-dot segment", input: "dir/name./child", archive: true},
		{name: "data payload", input: "data:text/plain,SENSITIVE"},
		{name: "authorization", input: "Authorization: Bearer SENSITIVE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			var err error
			if tc.archive {
				got, err = CanonicalArchivePath(tc.input)
			} else {
				got, err = CanonicalLogicalName(tc.input)
			}
			if (err == nil) != tc.ok {
				t.Fatalf("canonicalize(%q) = %q, %v; ok=%v", tc.input, got, err, tc.ok)
			}
			if tc.ok && got != tc.want {
				t.Fatalf("canonicalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
			if err != nil && (!errors.Is(err, ErrUnsafeMetadata) || strings.Contains(err.Error(), "SENSITIVE")) {
				t.Fatalf("unsafe error = %q", err)
			}
		})
	}
}

func TestCanonicalURLDisplayRedactsAuthorityPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "canonical https origin", input: "HTTPS://Example.COM:8443/path?q=SENSITIVE#fragment", want: "https://example.com:8443", ok: true},
		{name: "canonical default port", input: "HTTPS://Example.COM:0443/path", want: "https://example.com", ok: true},
		{name: "ipv6 origin", input: "http://[2001:db8::1]/secret", want: "http://[2001:db8::1]", ok: true},
		{name: "userinfo", input: "https://user:SENSITIVE@example.com/path"},
		{name: "non http", input: "file:///SENSITIVE/path"},
		{name: "missing host", input: "https:///path"},
		{name: "bad port", input: "https://example.com:notaport/path"},
		{name: "port out of range", input: "https://example.com:65536/path"},
		{name: "unicode hostname", input: "https://éxample.com/path"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CanonicalURLDisplay(tc.input)
			if (err == nil) != tc.ok {
				t.Fatalf("CanonicalURLDisplay() = %q, %v; ok=%v", got, err, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("CanonicalURLDisplay() = %q, want %q", got, tc.want)
			}
			if strings.Contains(got, "SENSITIVE") || err != nil && strings.Contains(err.Error(), "SENSITIVE") {
				t.Fatalf("URL display leaked payload: got=%q err=%v", got, err)
			}
		})
	}
}

func TestCanonicalFactsAndOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   string
		value  string
		origin FactOrigin
		want   Fact
		ok     bool
	}{
		{name: "media", kind: "media_type", value: " Application/PDF ", origin: OriginSniff, want: Fact{Kind: "media_type", Value: "application/pdf", Origin: OriginSniff}, ok: true},
		{name: "extension", kind: "extension", value: "PDF", origin: OriginCaller, want: Fact{Kind: "extension", Value: ".pdf", Origin: OriginCaller}, ok: true},
		{name: "filename", kind: "filename", value: " brief.PDF ", origin: OriginSource, want: Fact{Kind: "filename", Value: "brief.PDF", Origin: OriginSource}, ok: true},
		{name: "charset", kind: "charset", value: " UTF-8 ", origin: OriginConverter, want: Fact{Kind: "charset", Value: "utf-8", Origin: OriginConverter}, ok: true},
		{name: "unknown fact", kind: "SENSITIVE", value: "value", origin: OriginCaller},
		{name: "invalid origin", kind: "extension", value: "pdf", origin: "SENSITIVE"},
		{name: "media params", kind: "media_type", value: "text/plain; token=SENSITIVE", origin: OriginSource},
		{name: "bad extension", kind: "extension", value: "../SENSITIVE", origin: OriginCaller},
		{name: "bad charset", kind: "charset", value: "utf-8\nSENSITIVE", origin: OriginSource},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewFact(tc.kind, tc.value, tc.origin)
			if (err == nil) != tc.ok {
				t.Fatalf("NewFact() = %#v, %v; ok=%v", got, err, tc.ok)
			}
			if got != tc.want {
				t.Fatalf("NewFact() = %#v, want %#v", got, tc.want)
			}
			if err != nil && strings.Contains(err.Error(), "SENSITIVE") {
				t.Fatalf("NewFact() leaked payload: %q", err)
			}
		})
	}
}

func TestCanonicalExtensionAndMediaTypeGuardTable(t *testing.T) {
	t.Parallel()

	extensionCases := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: ".TXT", want: ".txt", ok: true},
		{input: "tar.gz", want: ".tar.gz", ok: true},
		{input: ""},
		{input: "\nPDF\n"},
		{input: "."},
		{input: "a/b"},
		{input: `a\b`},
		{input: "a?token=SENSITIVE"},
	}
	for _, tc := range extensionCases {
		got, err := CanonicalExtension(tc.input)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("CanonicalExtension(%q) = %q, %v; want %q ok=%v", tc.input, got, err, tc.want, tc.ok)
		}
	}

	mediaCases := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "IMAGE/PNG", want: "image/png", ok: true},
		{input: " application/vnd.openxmlformats-officedocument.wordprocessingml.document ", want: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", ok: true},
		{input: ""},
		{input: "\nimage/png\n"},
		{input: "text/plain; charset=utf-8"},
		{input: "not a type"},
	}
	for _, tc := range mediaCases {
		got, err := CanonicalMediaType(tc.input)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("CanonicalMediaType(%q) = %q, %v; want %q ok=%v", tc.input, got, err, tc.want, tc.ok)
		}
	}
}
