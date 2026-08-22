package pdfconv

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

type artifactAttributeProbe struct {
	attributes     []inkbite.MetadataFact
	detailedResult string
	legacyResult   string
}

type artifactMarkdownPDFProbe struct {
	*Converter
	markdown string
}

func (p *artifactMarkdownPDFProbe) ConvertDetailed(
	ctx context.Context,
	reader io.ReadSeeker,
	info inkbite.StreamInfo,
	options inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (inkbite.DetailedConversion, error) {
	conversion, err := p.Converter.ConvertDetailed(ctx, reader, info, options, policy)
	if err != nil {
		return inkbite.DetailedConversion{}, err
	}
	conversion.Result.Markdown = p.markdown
	return conversion, nil
}

func (*artifactAttributeProbe) Name() string      { return "artifact-attribute-probe" }
func (*artifactAttributeProbe) Priority() float64 { return 1 }
func (*artifactAttributeProbe) Accepts(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) bool {
	return true
}
func (p *artifactAttributeProbe) Convert(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) (inkbite.Result, error) {
	markdown := p.legacyResult
	if markdown == "" {
		markdown = "legacy"
	}
	return inkbite.Result{Markdown: markdown}, nil
}
func (p *artifactAttributeProbe) ConvertDetailed(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions, inkbite.IngestionPolicy) (inkbite.DetailedConversion, error) {
	markdown := p.detailedResult
	if markdown == "" {
		markdown = "detailed"
	}
	return inkbite.DetailedConversion{
		Result: inkbite.Result{Markdown: markdown},
		Artifacts: []inkbite.DetailedArtifact{{
			Role:       inkbite.ArtifactRoleEmbeddedImage,
			Bytes:      []byte("artifact"),
			MediaType:  "image/png",
			SafeName:   "artifact.png",
			Occurrence: "page-000001/object-000001",
			Attributes: p.attributes,
		}},
	}, nil
}

func TestDetailedArtifactReferencesFailClosedWithoutChangingLegacyLiterals(t *testing.T) {
	const legacyLiteral = "legacy inkbite-artifact:artifact-999999"
	for _, tc := range []struct {
		name       string
		markdown   string
		wantAccept bool
	}{
		{name: "exact end", markdown: "inkbite-artifact:artifact-000001", wantAccept: true},
		{name: "generated close paren", markdown: "inkbite-artifact:artifact-000001) trailing", wantAccept: true},
		{name: "close bracket", markdown: "inkbite-artifact:artifact-000001] trailing", wantAccept: true},
		{name: "close angle", markdown: "inkbite-artifact:artifact-000001> trailing", wantAccept: true},
		{name: "double quote", markdown: "inkbite-artifact:artifact-000001\" trailing", wantAccept: true},
		{name: "single quote", markdown: "inkbite-artifact:artifact-000001' trailing", wantAccept: true},
		{name: "space", markdown: "inkbite-artifact:artifact-000001 trailing", wantAccept: true},
		{name: "newline", markdown: "inkbite-artifact:artifact-000001\ntrailing", wantAccept: true},
		{name: "short", markdown: "inkbite-artifact:artifact-00000"},
		{name: "wrong prefix", markdown: "inkbite-artifact:invalid-000001"},
		{name: "alphanumeric continuation", markdown: "inkbite-artifact:artifact-000001x"},
		{name: "path", markdown: "inkbite-artifact:artifact-000001/extra"},
		{name: "query", markdown: "inkbite-artifact:artifact-000001?query"},
		{name: "fragment", markdown: "inkbite-artifact:artifact-000001#fragment"},
		{name: "percent encoding", markdown: "inkbite-artifact:artifact-000001%2fextra"},
		{name: "colon", markdown: "inkbite-artifact:artifact-000001:extra"},
		{name: "backslash", markdown: "inkbite-artifact:artifact-000001\\extra"},
		{name: "authority path", markdown: "inkbite-artifact:artifact-000001//example.invalid"},
		{name: "authority userinfo", markdown: "inkbite-artifact:artifact-000001@example.invalid"},
		{name: "punctuation continuation", markdown: "inkbite-artifact:artifact-000001.extra"},
		{name: "unicode continuation", markdown: "inkbite-artifact:artifact-000001é"},
		{name: "zero", markdown: "inkbite-artifact:artifact-000000"},
		{name: "out of range", markdown: "inkbite-artifact:artifact-000002"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := inkbite.New()
			engine.RegisterConverter(&artifactAttributeProbe{detailedResult: tc.markdown, legacyResult: legacyLiteral})
			envelope, err := engine.Ingest(context.Background(), []byte("source"), nil, inkbite.IngestOptions{})
			if tc.wantAccept {
				if err != nil {
					t.Fatalf("Ingest(%q) error = %v", tc.markdown, err)
				}
				if envelope.ContractVersion == "" || len(envelope.Primary.Bytes) == 0 || len(envelope.Artifacts) != 1 {
					t.Fatalf("Ingest(%q) envelope = %#v", tc.markdown, envelope)
				}
			} else {
				if !errors.Is(err, inkbite.ErrIntegrityFailure) {
					t.Fatalf("Ingest(%q) error = %v, want integrity category", tc.markdown, err)
				}
				if envelope.ContractVersion != "" || len(envelope.Primary.Bytes) != 0 || len(envelope.Artifacts) != 0 {
					t.Fatalf("Ingest(%q) returned partial envelope: %#v", tc.markdown, envelope)
				}
			}
			legacy, err := engine.Convert(context.Background(), []byte("source"), nil, inkbite.ConvertOptions{})
			if err != nil {
				t.Fatalf("Convert(%q) error = %v", tc.markdown, err)
			}
			if legacy.Markdown != legacyLiteral {
				t.Fatalf("Convert(%q) Markdown = %q, want exact legacy literal", tc.markdown, legacy.Markdown)
			}
		})
	}
}

func TestDetailedArtifactReferenceTokenStartAndCanonicalMapping(t *testing.T) {
	t.Setenv("PATH", "")
	const (
		firstReference  = "inkbite-artifact:artifact-000001"
		secondReference = "inkbite-artifact:artifact-000002"
		finalFirst      = "inkbite-artifact:artifact-000002"
		finalSecond     = "inkbite-artifact:artifact-000001"
	)
	for _, tc := range []struct {
		name     string
		markdown string
		want     string
	}{
		{name: "embedded identifier", markdown: "literal-prefix" + firstReference, want: "literal-prefix" + firstReference},
		{name: "longer URI", markdown: "https://example.invalid/" + firstReference, want: "https://example.invalid/" + firstReference},
		{name: "longer path", markdown: "/var/tmp/" + firstReference, want: "/var/tmp/" + firstReference},
		{name: "adjacent prose", markdown: "before" + firstReference + "after", want: "before" + firstReference + "after"},
		{name: "generated destination", markdown: "![PDF image](" + firstReference + ")", want: "![PDF image](" + finalFirst + ")"},
		{name: "exact EOF", markdown: firstReference, want: finalFirst},
		{name: "multiple references", markdown: firstReference + " " + secondReference, want: finalFirst + " " + finalSecond},
		{name: "space opener", markdown: "before " + firstReference + " after", want: "before " + finalFirst + " after"},
		{name: "tab opener", markdown: "before\t" + firstReference + "\tafter", want: "before\t" + finalFirst + "\tafter"},
		{name: "line opener", markdown: "before\n" + firstReference + "\nafter", want: "before\n" + finalFirst + "\nafter"},
		{name: "carriage return opener", markdown: "before\r" + firstReference + "\rafter", want: "before\n" + finalFirst + "\nafter"},
		{name: "bracket opener", markdown: "before[" + firstReference + "]after", want: "before[" + finalFirst + "]after"},
		{name: "angle opener", markdown: "before<" + firstReference + ">after", want: "before<" + finalFirst + ">after"},
		{name: "single quote opener", markdown: "before'" + firstReference + "'after", want: "before'" + finalFirst + "'after"},
		{name: "double quote opener", markdown: "before\"" + firstReference + "\"after", want: "before\"" + finalFirst + "\"after"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := inkbite.New()
			engine.RegisterConverter(&artifactMarkdownPDFProbe{Converter: New(), markdown: tc.markdown})
			envelope, err := engine.Ingest(
				context.Background(),
				makeTwoPageImagePDF(t, false),
				imagePDFInfo(),
				inkbite.IngestOptions{ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"}},
			)
			if err != nil {
				t.Fatalf("Ingest(%q) error = %v", tc.markdown, err)
			}
			if got := string(envelope.Primary.Bytes); got != tc.want {
				t.Fatalf("Ingest(%q) Markdown = %q, want %q", tc.markdown, got, tc.want)
			}
			if len(envelope.Artifacts) != 2 || envelope.Artifacts[0].MediaType != "image/png" || envelope.Artifacts[1].MediaType != "image/jpeg" {
				t.Fatalf("fixture did not reverse canonical order: %#v", envelope.Artifacts)
			}
		})
	}

	for _, tc := range []struct {
		name      string
		reference string
	}{
		{name: "short", reference: "inkbite-artifact:artifact-00000"},
		{name: "wrong prefix", reference: "inkbite-artifact:invalid-000001"},
		{name: "zero", reference: "inkbite-artifact:artifact-000000"},
		{name: "out of range", reference: "inkbite-artifact:artifact-000003"},
		{name: "alphanumeric continuation", reference: firstReference + "x"},
		{name: "path continuation", reference: firstReference + "/extra"},
		{name: "query continuation", reference: firstReference + "?query"},
		{name: "fragment continuation", reference: firstReference + "#fragment"},
		{name: "percent continuation", reference: firstReference + "%2fextra"},
		{name: "colon continuation", reference: firstReference + ":extra"},
		{name: "backslash continuation", reference: firstReference + "\\extra"},
		{name: "authority continuation", reference: firstReference + "//example.invalid"},
		{name: "userinfo continuation", reference: firstReference + "@example.invalid"},
		{name: "punctuation continuation", reference: firstReference + ".extra"},
		{name: "unicode continuation", reference: firstReference + "é"},
	} {
		t.Run("malformed "+tc.name, func(t *testing.T) {
			engine := inkbite.New()
			engine.RegisterConverter(&artifactMarkdownPDFProbe{Converter: New(), markdown: "before (" + tc.reference + ") after"})
			envelope, err := engine.Ingest(
				context.Background(),
				makeTwoPageImagePDF(t, false),
				imagePDFInfo(),
				inkbite.IngestOptions{ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"}},
			)
			if !errors.Is(err, inkbite.ErrIntegrityFailure) {
				t.Fatalf("Ingest(%q) error = %v, want integrity category", tc.reference, err)
			}
			if envelope.ContractVersion != "" || len(envelope.Primary.Bytes) != 0 || len(envelope.Artifacts) != 0 {
				t.Fatalf("Ingest(%q) returned partial envelope: %#v", tc.reference, envelope)
			}
		})
	}
}

func TestPDFDetailedArtifactAndPrimaryBoundaries(t *testing.T) {
	source := makeImageXObjectPDF("PDF artifact boundaries")
	baseline := ingestImagePDF(t, pdfEngine(), source, inkbite.IngestOptions{})
	if len(baseline.Artifacts) != 1 {
		t.Fatalf("baseline artifacts = %d, want 1", len(baseline.Artifacts))
	}
	artifactBytes := baseline.Artifacts[0].ByteLength
	primaryBytes := baseline.Primary.ByteLength

	atLimit := inkbite.DefaultIngestionPolicy()
	atLimit.MaxArtifacts = 1
	atLimit.MaxArtifactBytes = artifactBytes
	atLimit.MaxTotalArtifactBytes = artifactBytes
	atLimit.MaxPrimaryBytes = primaryBytes
	if envelope, err := pdfEngine().Ingest(context.Background(), source, imagePDFInfo(), inkbite.IngestOptions{Policy: atLimit}); err != nil {
		t.Fatalf("at-limit Ingest() error = %v", err)
	} else if len(envelope.Artifacts) != 1 {
		t.Fatalf("at-limit artifacts = %d", len(envelope.Artifacts))
	}

	tests := []struct {
		name   string
		policy inkbite.IngestionPolicy
	}{
		{name: "artifact count", policy: func() inkbite.IngestionPolicy { p := atLimit; p.MaxArtifacts = 0; return p }()},
		{name: "artifact item", policy: func() inkbite.IngestionPolicy { p := atLimit; p.MaxArtifactBytes--; return p }()},
		{name: "artifact aggregate", policy: func() inkbite.IngestionPolicy { p := atLimit; p.MaxTotalArtifactBytes--; return p }()},
		{name: "primary", policy: func() inkbite.IngestionPolicy { p := atLimit; p.MaxPrimaryBytes--; return p }()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envelope, err := pdfEngine().Ingest(context.Background(), source, imagePDFInfo(), inkbite.IngestOptions{Policy: tc.policy})
			if !errors.Is(err, inkbite.ErrLimitExceeded) {
				t.Fatalf("Ingest() error = %v, want limit category", err)
			}
			if envelope.ContractVersion != "" || len(envelope.Artifacts) != 0 || len(envelope.Primary.Bytes) != 0 {
				t.Fatalf("limit failure returned partial envelope: %#v", envelope)
			}
		})
	}
}

func TestPDFDetailedCancellationReturnsNoEnvelope(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	envelope, err := pdfEngine().Ingest(ctx, makeImageXObjectPDF("cancelled"), imagePDFInfo(), inkbite.IngestOptions{})
	if !errors.Is(err, inkbite.ErrCancellation) {
		t.Fatalf("Ingest() error = %v, want cancellation", err)
	}
	if envelope.ContractVersion != "" || len(envelope.Artifacts) != 0 {
		t.Fatalf("cancellation returned partial envelope: %#v", envelope)
	}
}

func TestPDFDetailedOptionalExtractionFailureIsVisible(t *testing.T) {
	source := makeImageXObjectPDFWithDictionary("Visible image degradation", "/Filter /BogusDecode")
	envelope, err := pdfEngine().Ingest(context.Background(), source, imagePDFInfo(), inkbite.IngestOptions{})
	if err != nil {
		t.Fatalf("optional extraction Ingest() error = %v", err)
	}
	if len(envelope.Artifacts) != 0 {
		t.Fatalf("optional extraction artifacts = %#v", envelope.Artifacts)
	}
	if len(envelope.Warnings) != 1 || envelope.Warnings[0].Category != "artifact_extraction_failed" ||
		envelope.Warnings[0].Converter != "pdf" || strings.Contains(envelope.Warnings[0].Detail, "BogusDecode") {
		t.Fatalf("optional extraction warnings = %#v", envelope.Warnings)
	}
}

func TestDetailedArtifactAttributesFailClosed(t *testing.T) {
	converterFact := func(kind, value string) inkbite.MetadataFact {
		return inkbite.MetadataFact{Kind: kind, Value: value, Origin: inkbite.MetadataOriginConverter}
	}
	tests := []struct {
		name       string
		attributes []inkbite.MetadataFact
	}{
		{name: "unknown kind", attributes: []inkbite.MetadataFact{converterFact("backend", "private")}},
		{name: "empty integer", attributes: []inkbite.MetadataFact{converterFact("width", "")}},
		{name: "negative integer", attributes: []inkbite.MetadataFact{converterFact("width", "-1")}},
		{name: "signed integer", attributes: []inkbite.MetadataFact{converterFact("width", "+1")}},
		{name: "leading zero", attributes: []inkbite.MetadataFact{converterFact("width", "01")}},
		{name: "integer overflow", attributes: []inkbite.MetadataFact{converterFact("width", "18446744073709551616")}},
		{name: "uppercase boolean", attributes: []inkbite.MetadataFact{converterFact("image_mask", "TRUE")}},
		{name: "wrong origin", attributes: []inkbite.MetadataFact{{Kind: "width", Value: "1", Origin: inkbite.MetadataOriginSource}}},
		{name: "duplicate kind", attributes: []inkbite.MetadataFact{converterFact("width", "1"), converterFact("width", "2")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := inkbite.New()
			engine.RegisterConverter(&artifactAttributeProbe{attributes: tc.attributes})
			envelope, err := engine.Ingest(context.Background(), []byte("source"), nil, inkbite.IngestOptions{})
			if !errors.Is(err, inkbite.ErrIntegrityFailure) {
				t.Fatalf("Ingest() error = %v, want integrity category", err)
			}
			if envelope.ContractVersion != "" || len(envelope.Source.Bytes) != 0 || len(envelope.Primary.Bytes) != 0 || len(envelope.Artifacts) != 0 {
				t.Fatalf("invalid attributes returned partial envelope: %#v", envelope)
			}
		})
	}
}

func TestDetailedImageArtifactsPreserveExtractionOrderingAndValidation(t *testing.T) {
	images := []extractedImage{
		{Page: 4, Object: 4, MediaType: "image/webp", Data: []byte("webp")},
		{Page: 3, Object: 3, MediaType: "image/tiff", Data: []byte("tiff")},
		{Page: 2, Object: 2, MediaType: "image/png", Data: []byte("png")},
		{Page: 1, Object: 1, MediaType: "image/jpeg", Data: []byte("jpeg")},
	}
	policy := inkbite.DefaultIngestionPolicy()
	artifacts, err := detailedImageArtifacts(context.Background(), images, policy)
	if err != nil {
		t.Fatal(err)
	}
	wantMediaTypes := []string{"image/webp", "image/tiff", "image/png", "image/jpeg"}
	for index, artifact := range artifacts {
		if artifact.MediaType != wantMediaTypes[index] {
			t.Fatalf("artifact %d media type = %q, want %q", index, artifact.MediaType, wantMediaTypes[index])
		}
	}
	images[0].Data[0] = 'X'
	if string(artifacts[0].Bytes) != "webp" {
		t.Fatal("detailed artifact aliases extracted image storage")
	}

	invalid := []extractedImage{
		{Page: 1, Object: 1, MediaType: "image/gif", Data: []byte("gif")},
		{Page: 0, Object: 1, MediaType: "image/png", Data: []byte("png")},
		{Page: 1, Object: 0, MediaType: "image/png", Data: []byte("png")},
		{Page: 1, Object: 1, Width: -1, MediaType: "image/png", Data: []byte("png")},
		{Page: 1, Object: 1, Height: -1, MediaType: "image/png", Data: []byte("png")},
		{Page: 1, Object: 1, Bpc: -1, MediaType: "image/png", Data: []byte("png")},
	}
	for _, image := range invalid {
		if _, err := detailedImageArtifacts(context.Background(), []extractedImage{image}, policy); !errors.Is(err, inkbite.ErrIntegrityFailure) {
			t.Fatalf("detailedImageArtifacts(%#v) error = %v, want integrity category", image, err)
		}
	}
}
