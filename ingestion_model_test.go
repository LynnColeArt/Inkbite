package inkbite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"testing"
)

func independentIdentity(data []byte) ContentIdentity {
	sum := sha256.Sum256(data)
	return ContentIdentity("sha256:" + hex.EncodeToString(sum[:]))
}

func validEnvelopeFixture() IngestionEnvelope {
	sourceBytes := []byte("source bytes")
	primaryBytes := []byte("# normalized\n")
	imageBytes := []byte{0x89, 'P', 'N', 'G'}
	sourceIdentity := independentIdentity(sourceBytes)
	primaryIdentity := independentIdentity(primaryBytes)
	imageIdentity := independentIdentity(imageBytes)
	policy := DefaultIngestionPolicy()

	return IngestionEnvelope{
		ContractVersion: IngestionContractV1,
		Source: SourceArtifact{
			Bytes:      append([]byte(nil), sourceBytes...),
			Identity:   sourceIdentity,
			ByteLength: int64(len(sourceBytes)),
			MediaType:  "application/pdf",
			SourceKind: SourceKindBytes,
			SafeName:   "brief.pdf",
		},
		Primary: ContentArtifact{
			ArtifactID: "artifact-000000",
			Role:       ArtifactRolePrimaryMarkdown,
			Bytes:      append([]byte(nil), primaryBytes...),
			Identity:   primaryIdentity,
			ByteLength: int64(len(primaryBytes)),
			MediaType:  "text/markdown",
			SafeName:   "brief.md",
			Relations: []ArtifactRelation{{
				Kind:   RelationDerivedFrom,
				FromID: string(sourceIdentity),
				ToID:   "artifact-000000",
			}},
			Attributes: []MetadataFact{},
		},
		Artifacts: []ContentArtifact{{
			ArtifactID: "artifact-000001",
			Role:       ArtifactRoleEmbeddedImage,
			Bytes:      append([]byte(nil), imageBytes...),
			Identity:   imageIdentity,
			ByteLength: int64(len(imageBytes)),
			MediaType:  "image/png",
			SafeName:   "page-000001-image-000001.png",
			Relations: []ArtifactRelation{{
				Kind:       RelationDerivedFrom,
				FromID:     string(sourceIdentity),
				ToID:       "artifact-000001",
				Occurrence: "page-000001/object-000001",
			}},
			Attributes: []MetadataFact{{
				Kind: "page", Value: "1", Origin: MetadataOriginConverter,
			}},
		}},
		Provenance: ConversionProvenance{
			ContractVersion:  IngestionContractV1,
			SourceIdentity:   sourceIdentity,
			Converter:        "pdf",
			Backend:          "native",
			StreamFacts:      []MetadataFact{{Kind: "media_type", Value: "application/pdf", Origin: MetadataOriginSniff}},
			Policy:           policy,
			OutputIdentities: []ContentIdentity{primaryIdentity, imageIdentity},
			Attempts:         []ConversionAttempt{{Converter: "pdf", Outcome: AttemptSelected}},
		},
		Warnings: []WarningRecord{},
	}
}

func TestIngestionEnvelopeJSONGoldenRoundTrip(t *testing.T) {
	envelope := validEnvelopeFixture()
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const golden = `{"contract_version":"inkbite.ingestion/v1","source":{"bytes":"c291cmNlIGJ5dGVz","identity":"sha256:4d4823794cbed3c4ee0bbc684c8f66e1dfd5afa6f078d494ce254ec5a4671753","byte_length":12,"media_type":"application/pdf","source_kind":"bytes","safe_name":"brief.pdf"},"primary":{"artifact_id":"artifact-000000","role":"primary_markdown","bytes":"IyBub3JtYWxpemVkCg==","identity":"sha256:8e877c4ad1acbd6baba7da532c905381ea7b408ccbef084647deb2a3888bb1fa","byte_length":13,"media_type":"text/markdown","safe_name":"brief.md","relations":[{"kind":"derived_from","from_id":"sha256:4d4823794cbed3c4ee0bbc684c8f66e1dfd5afa6f078d494ce254ec5a4671753","to_id":"artifact-000000"}],"attributes":[]},"artifacts":[{"artifact_id":"artifact-000001","role":"embedded_image","bytes":"iVBORw==","identity":"sha256:0f4636c78f65d3639ece5a064b5ae753e3408614a14fb18ab4d7540d2c248543","byte_length":4,"media_type":"image/png","safe_name":"page-000001-image-000001.png","relations":[{"kind":"derived_from","from_id":"sha256:4d4823794cbed3c4ee0bbc684c8f66e1dfd5afa6f078d494ce254ec5a4671753","to_id":"artifact-000001","occurrence":"page-000001/object-000001"}],"attributes":[{"kind":"page","value":"1","origin":"converter"}]}],"provenance":{"contract_version":"inkbite.ingestion/v1","source_identity":"sha256:4d4823794cbed3c4ee0bbc684c8f66e1dfd5afa6f078d494ce254ec5a4671753","converter":"pdf","backend":"native","stream_facts":[{"kind":"media_type","value":"application/pdf","origin":"sniff"}],"policy":{"max_source_bytes":33554432,"max_primary_bytes":33554432,"max_artifacts":256,"max_artifact_bytes":8388608,"max_total_artifact_bytes":33554432,"max_container_entries":256,"max_container_entry_bytes":8388608,"max_expanded_bytes":33554432,"max_container_depth":4,"max_expansion_ratio":1000,"remote_enabled":false},"output_identities":["sha256:8e877c4ad1acbd6baba7da532c905381ea7b408ccbef084647deb2a3888bb1fa","sha256:0f4636c78f65d3639ece5a064b5ae753e3408614a14fb18ab4d7540d2c248543"],"attempts":[{"converter":"pdf","outcome":"selected"}]},"warnings":[]}`
	if string(encoded) != golden {
		t.Fatalf("JSON mismatch\n got: %s\nwant: %s", encoded, golden)
	}

	var decoded IngestionEnvelope
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, envelope) {
		t.Fatalf("round trip mismatch\n got: %#v\nwant: %#v", decoded, envelope)
	}
	if report := VerifyEnvelope(decoded); !report.Valid {
		t.Fatalf("VerifyEnvelope() = %#v", report)
	}
}

func TestEnvelopeSerializationIsDeterministic(t *testing.T) {
	envelope := validEnvelopeFixture()
	want, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		got, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("serialization %d differs", i)
		}
	}
}

func TestV1ClosedEnumsMatchSchemaExactlyAndInOrder(t *testing.T) {
	tests := []struct {
		name string
		got  any
		want string
	}{
		{
			name: "source kinds",
			got:  []SourceKind{SourceKindBytes, SourceKindReader, SourceKindFile, SourceKindDataURI, SourceKindRemote},
			want: `["bytes","reader","file","data_uri","remote"]`,
		},
		{
			name: "relation kinds",
			got:  []RelationKind{RelationDerivedFrom, RelationEmbeddedIn, RelationReferencedBy},
			want: `["derived_from","embedded_in","referenced_by"]`,
		},
		{
			name: "metadata origins",
			got:  []MetadataOrigin{MetadataOriginCaller, MetadataOriginSource, MetadataOriginSniff, MetadataOriginConverter},
			want: `["caller","source","sniff","converter"]`,
		},
		{
			name: "attempt outcomes",
			got:  []AttemptOutcome{AttemptUnsupported, AttemptFailed, AttemptSelected},
			want: `["unsupported","failed","selected"]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.got)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("runtime v1 values = %s, schema order = %s", got, tc.want)
			}
		})
	}
}

type legacyOnlyConverter struct{}

func (legacyOnlyConverter) Name() string      { return "legacy" }
func (legacyOnlyConverter) Priority() float64 { return 1 }
func (legacyOnlyConverter) Accepts(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) bool {
	return true
}
func (legacyOnlyConverter) Convert(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) (Result, error) {
	return Result{"markdown", "title"}, nil
}

type detailedTestConverter struct{ legacyOnlyConverter }

func (detailedTestConverter) ConvertDetailed(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions, IngestionPolicy) (DetailedConversion, error) {
	return DetailedConversion{Result: Result{"markdown", "title"}}, nil
}

func TestDetailedConverterIsOptionalAndLegacyResultRemainsComparable(t *testing.T) {
	var legacy Converter = legacyOnlyConverter{}
	if _, ok := legacy.(DetailedConverter); ok {
		t.Fatal("legacy converter unexpectedly implements detailed capability")
	}
	var _ DetailedConverter = detailedTestConverter{}

	legacyResult := Result{"markdown", "title"}
	if legacyResult != (Result{"markdown", "title"}) {
		t.Fatal("legacy Result equality changed")
	}
	keyed := map[Result]string{legacyResult: "retained"}
	if keyed[legacyResult] != "retained" {
		t.Fatal("legacy Result is no longer usable as a map key")
	}
}

type resetCheckingLegacyConverter struct{}

func (resetCheckingLegacyConverter) Name() string      { return "reset-check" }
func (resetCheckingLegacyConverter) Priority() float64 { return 1 }
func (resetCheckingLegacyConverter) Accepts(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions) bool {
	buffer := make([]byte, 1)
	_, _ = r.Read(buffer)
	return true
}
func (resetCheckingLegacyConverter) Convert(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions) (Result, error) {
	data, err := io.ReadAll(r)
	return Result{Markdown: string(data)}, err
}

func TestLegacyConverterDispatchStillResetsReader(t *testing.T) {
	engine := New()
	engine.RegisterConverter(resetCheckingLegacyConverter{})
	result, err := engine.Convert(context.Background(), []byte("legacy bytes"), nil, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if result.Markdown != "legacy bytes" {
		t.Fatalf("legacy converter read %q after Accepts consumed input", result.Markdown)
	}
}
