package epubconv

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

type epubEntry struct {
	name   string
	body   []byte
	mode   os.FileMode
	method uint16
}

func TestEPUBRejectsUnsafeArchiveMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra []epubEntry
	}{
		{name: "traversal", extra: []epubEntry{{name: "../secret.txt", body: []byte("secret")}}},
		{name: "backslash", extra: []epubEntry{{name: `OPS\secret.txt`, body: []byte("secret")}}},
		{name: "absolute", extra: []epubEntry{{name: "/secret.txt", body: []byte("secret")}}},
		{name: "nul", extra: []epubEntry{{name: "secret\x00.txt", body: []byte("secret")}}},
		{name: "duplicate", extra: []epubEntry{{name: "OPS/chapter.xhtml", body: []byte("duplicate")}}},
		{name: "portable case collision", extra: []epubEntry{{name: "OPS/CHAPTER.XHTML", body: []byte("duplicate")}}},
		{name: "symlink", extra: []epubEntry{{name: "OPS/link", body: []byte("target"), mode: os.ModeSymlink | 0o777}}},
		{name: "special file", extra: []epubEntry{{name: "OPS/pipe", mode: os.ModeNamedPipe | 0o600}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ingestEPUB(t, context.Background(), validEPUB(t, tc.extra...), inkbite.DefaultIngestionPolicy())
			if !errors.Is(err, inkbite.ErrMalformedInput) {
				t.Fatalf("Ingest() error = %v, want malformed input", err)
			}
		})
	}
}

func TestEPUBRejectsUnsafeInternalReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rootfile  string
		spineHref string
	}{
		{name: "traversal rootfile", rootfile: "../OPS/content.opf", spineHref: "chapter.xhtml"},
		{name: "absolute rootfile", rootfile: "/OPS/content.opf", spineHref: "chapter.xhtml"},
		{name: "backslash rootfile", rootfile: `OPS\content.opf`, spineHref: "chapter.xhtml"},
		{name: "traversal manifest href", rootfile: "OPS/content.opf", spineHref: "../../secret.xhtml"},
		{name: "absolute manifest href", rootfile: "OPS/content.opf", spineHref: "/secret.xhtml"},
		{name: "backslash manifest href", rootfile: "OPS/content.opf", spineHref: `folder\chapter.xhtml`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			archive := buildEPUB(t, []epubEntry{
				{name: "META-INF/container.xml", body: []byte(containerDocument(tc.rootfile))},
				{name: "OPS/content.opf", body: []byte(packageDocumentFixture(tc.spineHref, "chapter"))},
				{name: "OPS/chapter.xhtml", body: []byte("<h1>Chapter</h1>")},
			})
			_, err := ingestEPUB(t, context.Background(), archive, inkbite.DefaultIngestionPolicy())
			if !errors.Is(err, inkbite.ErrMalformedInput) {
				t.Fatalf("Ingest() error = %v, want malformed input", err)
			}
		})
	}
}

func TestEPUBPolicyBoundsEveryArchiveEntry(t *testing.T) {
	t.Parallel()

	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntries = 3
	if _, err := ingestEPUB(t, context.Background(), validEPUB(t), policy); err != nil {
		t.Fatalf("Ingest() at entry-count limit error = %v", err)
	}
	_, err := ingestEPUB(t, context.Background(), validEPUB(t, epubEntry{name: "OPS/unused.txt", body: []byte("unused")}), policy)
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("Ingest() count limit+1 error = %v, want limit exceeded", err)
	}
}

func TestEPUBBoundsContentReadsByActualBytes(t *testing.T) {
	t.Parallel()

	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntryBytes = 1024
	archive := buildEPUB(t, []epubEntry{
		{name: "META-INF/container.xml", body: []byte(containerDocument("OPS/content.opf"))},
		{name: "OPS/content.opf", body: []byte(packageDocumentFixture("chapter.xhtml", "chapter"))},
		{name: "OPS/chapter.xhtml", body: bytes.Repeat([]byte("x"), 1025)},
	})
	_, err := ingestEPUB(t, context.Background(), archive, policy)
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("Ingest() entry limit+1 error = %v, want limit exceeded", err)
	}
}

func TestEPUBRejectsExpansionRatioLimitPlusOne(t *testing.T) {
	t.Parallel()

	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxExpansionRatio = 1
	archive := buildEPUB(t, []epubEntry{
		{name: "META-INF/container.xml", body: []byte(containerDocument("OPS/content.opf"))},
		{name: "OPS/content.opf", body: []byte(packageDocumentFixture("chapter.xhtml", "chapter"))},
		{name: "OPS/chapter.xhtml", body: bytes.Repeat([]byte("a"), 4096), method: zip.Deflate},
	})
	_, err := ingestEPUB(t, context.Background(), archive, policy)
	if !errors.Is(err, inkbite.ErrLimitExceeded) {
		t.Fatalf("Ingest() ratio limit+1 error = %v, want limit exceeded", err)
	}
}

func TestEPUBReadsContentThroughEOFForChecksum(t *testing.T) {
	t.Parallel()

	archive := validEPUB(t)
	payload := []byte("<html><body><h1>Chapter</h1></body></html>")
	index := bytes.Index(archive, payload)
	if index < 0 {
		t.Fatal("stored chapter payload not found")
	}
	archive[index] ^= 0xff
	_, err := ingestEPUB(t, context.Background(), archive, inkbite.DefaultIngestionPolicy())
	if !errors.Is(err, inkbite.ErrIntegrityFailure) {
		t.Fatalf("Ingest() error = %v, want integrity failure", err)
	}
}

func TestEPUBDetailedWarningsExposeOptionalSpineSkips(t *testing.T) {
	t.Parallel()

	archive := buildEPUB(t, []epubEntry{
		{name: "META-INF/container.xml", body: []byte(containerDocument("OPS/content.opf"))},
		{name: "OPS/content.opf", body: []byte(`
<package xmlns="http://www.idpf.org/2007/opf">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Warnings</dc:title></metadata>
  <manifest><item id="missing-file" href="missing.xhtml"/></manifest>
  <spine><itemref idref="missing-manifest"/><itemref idref="missing-file"/></spine>
</package>`)},
	})
	envelope, err := ingestEPUB(t, context.Background(), archive, inkbite.DefaultIngestionPolicy())
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	want := []inkbite.WarningRecord{
		{Category: "missing_manifest_item", Converter: "epub", Location: "missing-manifest"},
		{Category: "missing_spine_content", Converter: "epub", Location: "OPS/missing.xhtml"},
	}
	if !reflect.DeepEqual(envelope.Warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", envelope.Warnings, want)
	}
}

func TestEPUBCancellationReturnsNoEnvelope(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	envelope, err := ingestEPUB(t, ctx, validEPUB(t), inkbite.DefaultIngestionPolicy())
	if !errors.Is(err, inkbite.ErrCancellation) {
		t.Fatalf("Ingest() error = %v, want cancellation", err)
	}
	if !reflect.DeepEqual(envelope, inkbite.IngestionEnvelope{}) {
		t.Fatalf("Ingest() envelope = %#v, want zero", envelope)
	}
}

func ingestEPUB(t *testing.T, ctx context.Context, archive []byte, policy inkbite.IngestionPolicy) (inkbite.IngestionEnvelope, error) {
	t.Helper()
	engine := inkbite.New()
	engine.RegisterConverter(New())
	return engine.Ingest(ctx, archive, &inkbite.StreamInfo{Extension: ".epub", Filename: "book.epub"}, inkbite.IngestOptions{Policy: policy})
}

func validEPUB(t *testing.T, extra ...epubEntry) []byte {
	t.Helper()
	entries := []epubEntry{
		{name: "META-INF/container.xml", body: []byte(containerDocument("OPS/content.opf"))},
		{name: "OPS/content.opf", body: []byte(packageDocumentFixture("chapter.xhtml", "chapter"))},
		{name: "OPS/chapter.xhtml", body: []byte("<html><body><h1>Chapter</h1></body></html>")},
	}
	return buildEPUB(t, append(entries, extra...))
}

func buildEPUB(t *testing.T, entries []epubEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, source := range entries {
		header := &zip.FileHeader{Name: source.name, Method: source.method}
		if source.mode != 0 {
			header.SetMode(source.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", source.name, err)
		}
		if _, err := entry.Write(source.body); err != nil {
			t.Fatalf("Write(%q) error = %v", source.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return bytes.Clone(buffer.Bytes())
}

func containerDocument(rootfile string) string {
	return `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="` + rootfile + `"/></rootfiles></container>`
}

func packageDocumentFixture(href, id string) string {
	return `<package xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Book</dc:title></metadata><manifest><item id="` + id + `" href="` + href + `"/></manifest><spine><itemref idref="` + id + `"/></spine></package>`
}
