package inkbite_test

import (
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

type externalDetailedConverter struct {
	artifact []byte
}

func (*externalDetailedConverter) Name() string      { return "external-detailed" }
func (*externalDetailedConverter) Priority() float64 { return 1 }
func (*externalDetailedConverter) Accepts(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) bool {
	return true
}
func (*externalDetailedConverter) Convert(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) (inkbite.Result, error) {
	return inkbite.Result{Markdown: "legacy-only", Title: "wrong path"}, nil
}
func (c *externalDetailedConverter) ConvertDetailed(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions, inkbite.IngestionPolicy) (inkbite.DetailedConversion, error) {
	return inkbite.DetailedConversion{
		Result: inkbite.Result{Markdown: "# detailed", Title: "Detailed title"},
		Artifacts: []inkbite.DetailedArtifact{{
			Role:       inkbite.ArtifactRoleEmbeddedImage,
			Bytes:      c.artifact,
			MediaType:  "image/png",
			SafeName:   "figure.png",
			Occurrence: "page-1",
			Attributes: []inkbite.MetadataFact{},
		}},
		Warnings: []inkbite.WarningRecord{},
	}, nil
}

func TestPublicDetailedIngestionBoundaryAndLegacyProjection(t *testing.T) {
	source := []byte("external source")
	derivative := []byte("external artifact")
	engine := inkbite.New()
	engine.RegisterConverter(&externalDetailedConverter{artifact: derivative})

	envelope, err := engine.Ingest(context.Background(), source, &inkbite.StreamInfo{Filename: "brief.bin"}, inkbite.IngestOptions{})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if report := inkbite.VerifyEnvelope(envelope); !report.Valid {
		t.Fatalf("public envelope failed verification: %#v", report.Findings)
	}
	if string(envelope.Source.Bytes) != "external source" || string(envelope.Primary.Bytes) != "# detailed" ||
		len(envelope.Artifacts) != 1 || string(envelope.Artifacts[0].Bytes) != "external artifact" {
		t.Fatalf("public envelope values = %#v", envelope)
	}
	source[0] = 'X'
	derivative[0] = 'X'
	if string(envelope.Source.Bytes) != "external source" || string(envelope.Artifacts[0].Bytes) != "external artifact" {
		t.Fatal("public envelope aliases caller or converter storage")
	}

	legacy, err := engine.Convert(context.Background(), []byte("external source"), nil, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if legacy != (inkbite.Result{Markdown: "legacy-only", Title: "wrong path"}) {
		t.Fatalf("legacy projection = %#v", legacy)
	}
}

func TestConvertFileURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello from file uri"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}

	result, err := engine.ConvertURI(context.Background(), (&url.URL{Scheme: "file", Path: uriPath}).String(), nil, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("ConvertURI() error = %v", err)
	}
	if result.Markdown != "hello from file uri" {
		t.Fatalf("expected file contents, got %q", result.Markdown)
	}
}
