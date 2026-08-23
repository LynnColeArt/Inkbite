package inkbite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func hasFinding(report VerificationReport, category VerificationCategory) bool {
	for _, finding := range report.Findings {
		if finding.Category == category {
			return true
		}
	}
	return false
}

func TestFailureErrorSupportsCategoryMatchingAndTypedInspection(t *testing.T) {
	categories := []struct {
		category FailureCategory
		target   error
	}{
		{FailureUnsupported, ErrUnsupportedFormat},
		{FailureMalformed, ErrMalformedInput},
		{FailureLimit, ErrLimitExceeded},
		{FailurePolicy, ErrPolicyViolation},
		{FailureIntegrity, ErrIntegrityFailure},
		{FailureCancellation, ErrCancellation},
		{FailureConverter, ErrConverterFailure},
	}
	for _, tc := range categories {
		t.Run(string(tc.category), func(t *testing.T) {
			err := &FailureError{Category: tc.category, Operation: "ingest", Cause: errors.New("SECRET backend trace")}
			if !errors.Is(err, tc.target) {
				t.Fatalf("errors.Is(%v, %v) = false", err, tc.target)
			}
			var typed *FailureError
			if !errors.As(err, &typed) || typed.Category != tc.category {
				t.Fatalf("errors.As() = %#v", typed)
			}
		})
	}
}

func TestPublicErrorsRedactSensitiveCauses(t *testing.T) {
	secret := "Bearer credential data:text/plain,TOPSECRET /home/user/private backend.Stack()"
	errorsToFormat := []error{
		&FailureError{Category: FailureConverter, Operation: "convert", Cause: errors.New(secret)},
		ConversionError{Converter: "pdf", Err: errors.New(secret)},
		FailedAttemptsError{Attempts: []ConversionError{{Converter: "pdf", Err: errors.New(secret)}}},
	}
	for _, err := range errorsToFormat {
		formatted := fmt.Sprint(err)
		if strings.Contains(formatted, secret) || strings.Contains(formatted, "TOPSECRET") || strings.Contains(formatted, "data:text/plain") {
			t.Fatalf("public error leaked sensitive cause: %q", formatted)
		}
	}
}

func TestConversionErrorRetainsCauseAndConverterCategory(t *testing.T) {
	cause := errors.New("internal")
	err := ConversionError{Converter: "pdf", Err: cause}
	if !errors.Is(err, cause) || !errors.Is(err, ErrConverterFailure) {
		t.Fatalf("ConversionError category/cause matching failed: %v", err)
	}
}

func TestVerifyEnvelopeRecomputesEveryIdentity(t *testing.T) {
	envelope := validEnvelopeFixture()
	report := VerifyEnvelope(envelope)
	if !report.Valid || len(report.Findings) != 0 {
		t.Fatalf("VerifyEnvelope() = %#v", report)
	}
	if report.VerifiedSourceIdentity != independentIdentity(envelope.Source.Bytes) {
		t.Fatalf("source identity = %q", report.VerifiedSourceIdentity)
	}
	want := []ContentIdentity{
		independentIdentity(envelope.Primary.Bytes),
		independentIdentity(envelope.Artifacts[0].Bytes),
	}
	if !reflectIdentities(report.VerifiedArtifactIdentities, want) {
		t.Fatalf("artifact identities = %#v, want %#v", report.VerifiedArtifactIdentities, want)
	}
}

func reflectIdentities(got, want []ContentIdentity) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestVerifyEnvelopeRejectsMutationsAndSubstitution(t *testing.T) {
	tests := map[string]func(*IngestionEnvelope){
		"source byte":   func(e *IngestionEnvelope) { e.Source.Bytes[0] ^= 1 },
		"primary byte":  func(e *IngestionEnvelope) { e.Primary.Bytes[0] ^= 1 },
		"artifact byte": func(e *IngestionEnvelope) { e.Artifacts[0].Bytes[0] ^= 1 },
		"source substitution": func(e *IngestionEnvelope) {
			e.Provenance.SourceIdentity = independentIdentity([]byte("other envelope"))
		},
		"output substitution": func(e *IngestionEnvelope) {
			e.Provenance.OutputIdentities[1] = independentIdentity([]byte("other artifact"))
		},
		"noncanonical identity": func(e *IngestionEnvelope) {
			e.Source.Identity = ContentIdentity("SHA256:10D8BD0BB12B7A1A9E3908151C2179D65996369B2D3F0AAABE98C2F0080A3CB8")
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			mutate(&envelope)
			report := VerifyEnvelope(envelope)
			if report.Valid || !hasFinding(report, VerificationIntegrity) {
				t.Fatalf("VerifyEnvelope() = %#v, want integrity finding", report)
			}
		})
	}
}

func TestVerifyEnvelopeRejectsMalformedObjectsOrderingAndReferences(t *testing.T) {
	tests := map[string]struct {
		mutate   func(*IngestionEnvelope)
		category VerificationCategory
	}{
		"unsupported version":   {func(e *IngestionEnvelope) { e.ContractVersion = "inkbite.ingestion/v2" }, VerificationContract},
		"missing primary":       {func(e *IngestionEnvelope) { e.Primary = ContentArtifact{} }, VerificationShape},
		"duplicate id":          {func(e *IngestionEnvelope) { e.Artifacts[0].ArtifactID = e.Primary.ArtifactID }, VerificationDuplicate},
		"invalid reference":     {func(e *IngestionEnvelope) { e.Artifacts[0].Relations[0].FromID = "artifact-999999" }, VerificationReference},
		"wrong relation target": {func(e *IngestionEnvelope) { e.Artifacts[0].Relations[0].ToID = e.Primary.ArtifactID }, VerificationRelationship},
		"out of order artifacts": {func(e *IngestionEnvelope) {
			second := e.Artifacts[0]
			second.ArtifactID = "artifact-000002"
			second.SafeName = "z.png"
			second.Relations = []ArtifactRelation{{Kind: RelationDerivedFrom, FromID: string(e.Source.Identity), ToID: second.ArtifactID, Occurrence: "page-000002"}}
			e.Artifacts = []ContentArtifact{second, e.Artifacts[0]}
		}, VerificationOrdering},
		"out of order facts": {func(e *IngestionEnvelope) {
			e.Provenance.StreamFacts = []MetadataFact{
				{Kind: "media_type", Value: "application/pdf", Origin: MetadataOriginSniff},
				{Kind: "charset", Value: "utf-8", Origin: MetadataOriginSource},
			}
		}, VerificationOrdering},
		"unsafe warning": {func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "fallback", Detail: "data:text/plain,SECRET"}}
		}, VerificationWarning},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			tc.mutate(&envelope)
			report := VerifyEnvelope(envelope)
			if report.Valid || !hasFinding(report, tc.category) {
				t.Fatalf("VerifyEnvelope() = %#v, want %q finding", report, tc.category)
			}
		})
	}
}

func TestVerifyEnvelopeDoesNoNetworkOrMutation(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	envelope := validEnvelopeFixture()
	envelope.Warnings = []WarningRecord{{Category: "remote_reference", Location: server.URL}}
	before, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	report := VerifyEnvelope(envelope)
	after, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Fatalf("VerifyEnvelope() = %#v", report)
	}
	if requests.Load() != 0 {
		t.Fatalf("verification issued %d network requests", requests.Load())
	}
	if !bytes.Equal(before, after) {
		t.Fatal("verification mutated its input")
	}
}

func TestVerifyEnvelopeConcurrentDeterminism(t *testing.T) {
	want := VerifyEnvelope(validEnvelopeFixture())
	const workers = 100
	results := make(chan VerificationReport, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- VerifyEnvelope(validEnvelopeFixture())
		}()
	}
	wg.Wait()
	close(results)
	for got := range results {
		if !got.Valid || !reflectIdentities(got.VerifiedArtifactIdentities, want.VerifiedArtifactIdentities) {
			t.Fatalf("concurrent report = %#v, want %#v", got, want)
		}
	}
}

func TestVerifyEnvelopeRejectsInvalidV1Shapes(t *testing.T) {
	tests := map[string]func(*IngestionEnvelope){
		"null collections": func(e *IngestionEnvelope) { e.Artifacts, e.Warnings = nil, nil },
		"source length and limit": func(e *IngestionEnvelope) {
			e.Source.ByteLength++
			e.Provenance.Policy.MaxSourceBytes = 1
		},
		"source kind":       func(e *IngestionEnvelope) { e.Source.SourceKind = "socket" },
		"source media type": func(e *IngestionEnvelope) { e.Source.MediaType = "Application/PDF; secret=x" },
		"source safe name":  func(e *IngestionEnvelope) { e.Source.SafeName = "../secret.pdf" },
		"primary shape": func(e *IngestionEnvelope) {
			e.Primary.ArtifactID = "primary"
			e.Primary.Role = "binary"
			e.Primary.MediaType = "text/plain"
		},
		"artifact shape": func(e *IngestionEnvelope) {
			e.Artifacts[0].ArtifactID = "artifact-x"
			e.Artifacts[0].Role = ArtifactRolePrimaryMarkdown
			e.Artifacts[0].MediaType = "IMAGE/PNG"
			e.Artifacts[0].SafeName = "/private/image.png"
			e.Artifacts[0].ByteLength++
		},
		"artifact policy": func(e *IngestionEnvelope) {
			e.Provenance.Policy.MaxArtifactBytes = 1
			e.Provenance.Policy.MaxTotalArtifactBytes = 1
		},
		"null artifact details": func(e *IngestionEnvelope) {
			e.Artifacts[0].Relations = nil
			e.Artifacts[0].Attributes = nil
		},
		"duplicate invalid relationship": func(e *IngestionEnvelope) {
			relation := e.Artifacts[0].Relations[0]
			relation.Kind = "copied_by"
			relation.Occurrence = "data:text/plain,secret"
			e.Artifacts[0].Relations = []ArtifactRelation{relation, relation}
		},
		"provenance identities": func(e *IngestionEnvelope) {
			e.Provenance.Converter = "PDF Converter"
			e.Provenance.Backend = "Native Backend"
			e.Provenance.Component = "data:text/plain,secret"
			e.Provenance.Policy.Component = "other"
		},
		"null provenance details": func(e *IngestionEnvelope) {
			e.Provenance.StreamFacts = nil
			e.Provenance.OutputIdentities = nil
			e.Provenance.Attempts = nil
		},
		"invalid attempts": func(e *IngestionEnvelope) {
			e.Provenance.Attempts = []ConversionAttempt{
				{Converter: "bad converter", Outcome: "unknown", Category: "data:text/plain,secret"},
				{Converter: "pdf", Outcome: AttemptSelected},
				{Converter: "pdf", Outcome: AttemptSelected},
			}
		},
		"duplicate invalid facts": func(e *IngestionEnvelope) {
			fact := MetadataFact{Kind: "bad kind", Value: "data:text/plain,secret", Origin: "host"}
			e.Provenance.StreamFacts = []MetadataFact{fact, fact}
		},
		"warning order and converter": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{
				{Category: "z", Converter: "bad converter"},
				{Category: "a"},
			}
		},
		"shared storage": func(e *IngestionEnvelope) { e.Primary.Bytes = e.Source.Bytes },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			mutate(&envelope)
			if report := VerifyEnvelope(envelope); report.Valid {
				t.Fatalf("VerifyEnvelope() unexpectedly valid: %#v", report)
			}
		})
	}
}

func TestVerifyEnvelopeRejectsSensitivePublicMetadataWithoutEchoingIt(t *testing.T) {
	const secret = "TOPSECRET"
	tests := map[string]func(*IngestionEnvelope){
		"source name query": func(e *IngestionEnvelope) {
			e.Source.SafeName = "brief.pdf?token=" + secret
		},
		"artifact name fragment": func(e *IngestionEnvelope) {
			e.Artifacts[0].SafeName = "image.png#" + secret
		},
		"warning location query": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "remote_reference", Location: "https://example.test/document?token=" + secret}}
		},
		"warning detail fragment": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "degraded", Detail: "detail#" + secret}}
		},
		"fact value userinfo": func(e *IngestionEnvelope) {
			e.Provenance.StreamFacts = []MetadataFact{{Kind: "location", Value: "https://user:" + secret + "@example.test/document", Origin: MetadataOriginSource}}
		},
		"fact value data uri": func(e *IngestionEnvelope) {
			e.Provenance.StreamFacts = []MetadataFact{{Kind: "payload", Value: "data:text/plain," + secret, Origin: MetadataOriginSource}}
		},
		"warning authorization": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "remote", Detail: "Authorization: Bearer " + secret}}
		},
		"warning control character": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "remote", Location: "member\n" + secret}}
		},
		"relation occurrence query": func(e *IngestionEnvelope) {
			e.Artifacts[0].Relations[0].Occurrence = "page-000001?token=" + secret
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			mutate(&envelope)
			report := VerifyEnvelope(envelope)
			if report.Valid {
				t.Fatalf("VerifyEnvelope() unexpectedly accepted sensitive metadata: %#v", report)
			}
			formatted := fmt.Sprint(report.Findings)
			if strings.Contains(formatted, secret) {
				t.Fatalf("verification finding echoed sensitive metadata: %q", formatted)
			}
		})
	}
}

func TestVerifyEnvelopeRejectsEveryNonEmptyStorageOverlap(t *testing.T) {
	backing := []byte("abcdefgh")
	tests := map[string]struct {
		source  []byte
		primary []byte
		valid   bool
	}{
		"exact alias":               {source: backing[:4], primary: backing[:4]},
		"prefix suffix overlap":     {source: backing[:5], primary: backing[3:]},
		"interior overlap":          {source: backing[:], primary: backing[2:5]},
		"clipped adjacent ranges":   {source: backing[:4:4], primary: backing[4:8:8], valid: true},
		"independent equal content": {source: []byte("same"), primary: []byte("same"), valid: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			setEnvelopeSourceAndPrimaryBytes(&envelope, tc.source, tc.primary)
			report := VerifyEnvelope(envelope)
			if tc.valid {
				if !report.Valid {
					t.Fatalf("VerifyEnvelope() rejected disjoint storage: %#v", report)
				}
				return
			}
			if report.Valid || !hasFinding(report, VerificationOwnership) {
				t.Fatalf("VerifyEnvelope() = %#v, want ownership finding", report)
			}
		})
	}
}

func setEnvelopeSourceAndPrimaryBytes(envelope *IngestionEnvelope, source, primary []byte) {
	oldSourceIdentity := envelope.Source.Identity
	envelope.Source.Bytes = source
	envelope.Source.ByteLength = int64(len(source))
	envelope.Source.Identity = independentIdentity(source)
	envelope.Primary.Bytes = primary
	envelope.Primary.ByteLength = int64(len(primary))
	envelope.Primary.Identity = independentIdentity(primary)
	envelope.Provenance.SourceIdentity = envelope.Source.Identity
	envelope.Provenance.OutputIdentities[0] = envelope.Primary.Identity
	for i := range envelope.Primary.Relations {
		if envelope.Primary.Relations[i].FromID == string(oldSourceIdentity) {
			envelope.Primary.Relations[i].FromID = string(envelope.Source.Identity)
		}
	}
	for i := range envelope.Artifacts {
		for j := range envelope.Artifacts[i].Relations {
			if envelope.Artifacts[i].Relations[j].FromID == string(oldSourceIdentity) {
				envelope.Artifacts[i].Relations[j].FromID = string(envelope.Source.Identity)
			}
		}
	}
}

func TestVerifyEnvelopeEnforcesAbsoluteV1ArtifactCeiling(t *testing.T) {
	for _, count := range []int{DefaultMaxArtifacts, DefaultMaxArtifacts + 1} {
		t.Run(fmt.Sprintf("count_%d", count), func(t *testing.T) {
			envelope := envelopeWithArtifactCount(count)
			// A request may carry a stricter effective limit, but it cannot widen
			// the closed v1 wire shape beyond the schema's absolute ceiling.
			envelope.Provenance.Policy.MaxArtifacts = DefaultMaxArtifacts + 1
			report := VerifyEnvelope(envelope)
			if count == DefaultMaxArtifacts {
				if !report.Valid {
					t.Fatalf("VerifyEnvelope() rejected v1 ceiling: %#v", report)
				}
				return
			}
			if report.Valid || !hasFinding(report, VerificationPolicy) {
				t.Fatalf("VerifyEnvelope() = %#v, want absolute v1 ceiling finding", report)
			}
		})
	}
}

func TestVerifyEnvelopeRejectsSemanticallyDuplicateArtifactsDespiteDistinctIDs(t *testing.T) {
	envelope := validEnvelopeFixture()
	duplicate := envelope.Artifacts[0]
	duplicate.ArtifactID = "artifact-000002"
	duplicate.Bytes = append([]byte(nil), duplicate.Bytes...)
	duplicate.Relations = append([]ArtifactRelation(nil), duplicate.Relations...)
	duplicate.Relations[0].ToID = duplicate.ArtifactID
	duplicate.Attributes = append([]MetadataFact(nil), duplicate.Attributes...)
	envelope.Artifacts = append(envelope.Artifacts, duplicate)
	envelope.Provenance.OutputIdentities = append(envelope.Provenance.OutputIdentities, duplicate.Identity)

	report := VerifyEnvelope(envelope)
	if report.Valid || !hasFindingAtPath(report, VerificationDuplicate, "artifacts") {
		t.Fatalf("VerifyEnvelope() = %#v, want positional-ID-independent duplicate finding", report)
	}
}

func TestVerifyEnvelopeRejectsPortableAbsolutePathsAcrossPublicMetadata(t *testing.T) {
	const secret = "PRIVATE_PATH_SENTINEL"
	tests := map[string]func(*IngestionEnvelope){
		"source safe name": func(e *IngestionEnvelope) {
			e.Source.SafeName = "/home/user/" + secret + ".pdf"
		},
		"artifact safe name": func(e *IngestionEnvelope) {
			e.Artifacts[0].SafeName = "C:/Users/user/" + secret + ".png"
		},
		"artifact role": func(e *IngestionEnvelope) {
			e.Artifacts[0].Role = ArtifactRole("/private/" + secret)
		},
		"relation occurrence": func(e *IngestionEnvelope) {
			e.Artifacts[0].Relations[0].Occurrence = `C:\Users\user\` + secret
		},
		"metadata fact": func(e *IngestionEnvelope) {
			e.Provenance.StreamFacts = []MetadataFact{{Kind: "location", Value: "/var/private/" + secret, Origin: MetadataOriginSource}}
		},
		"policy and provenance component": func(e *IngestionEnvelope) {
			e.Provenance.Component = "C:/components/" + secret
			e.Provenance.Policy.Component = e.Provenance.Component
		},
		"attempt category": func(e *IngestionEnvelope) {
			e.Provenance.Attempts[0].Category = `\\server\share\` + secret
		},
		"warning category": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "/private/" + secret}}
		},
		"warning location": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "source", Location: "/home/user/" + secret}}
		},
		"warning detail containing path": func(e *IngestionEnvelope) {
			e.Warnings = []WarningRecord{{Category: "source", Detail: "failed at /home/user/" + secret}}
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			mutate(&envelope)
			report := VerifyEnvelope(envelope)
			if report.Valid {
				t.Fatalf("VerifyEnvelope() unexpectedly accepted absolute metadata: %#v", report)
			}
			if formatted := fmt.Sprint(report.Findings); strings.Contains(formatted, secret) {
				t.Fatalf("verification finding echoed absolute metadata: %q", formatted)
			}
		})
	}
}

func TestVerifyEnvelopeAllowsRelativeLogicalLocations(t *testing.T) {
	envelope := validEnvelopeFixture()
	envelope.Source.SafeName = "briefs/final.pdf"
	envelope.Artifacts[0].SafeName = "images/page-000001.png"
	envelope.Artifacts[0].Relations[0].Occurrence = "pages/page-000001/object-000001"
	envelope.Provenance.StreamFacts = []MetadataFact{{Kind: "location", Value: "members/chapter-000001.xhtml", Origin: MetadataOriginSource}}
	envelope.Provenance.Attempts[0].Category = "fallback/unsupported"
	envelope.Warnings = []WarningRecord{{Category: "degraded", Location: "members/chapter-000001.xhtml", Detail: "relative/logical/location"}}

	if report := VerifyEnvelope(envelope); !report.Valid {
		t.Fatalf("VerifyEnvelope() rejected relative logical metadata: %#v", report)
	}
}

func TestVerifyEnvelopeEnforcesAbsoluteV1ByteCeilings(t *testing.T) {
	const (
		qualifiedLargeCeiling  = int64(256 << 20)
		derivedArtifactCeiling = int64(32 << 20)
	)
	tests := []struct {
		name    string
		path    string
		ceiling int64
		set     func(*IngestionEnvelope, []byte)
	}{
		{
			name:    "source",
			path:    "source.byte_length",
			ceiling: qualifiedLargeCeiling,
			set: func(e *IngestionEnvelope, data []byte) {
				setEnvelopeSourceAndPrimaryBytes(e, data, e.Primary.Bytes)
			},
		},
		{
			name:    "primary",
			path:    "primary.byte_length",
			ceiling: qualifiedLargeCeiling,
			set: func(e *IngestionEnvelope, data []byte) {
				setEnvelopeSourceAndPrimaryBytes(e, e.Source.Bytes, data)
			},
		},
		{
			name:    "derivative",
			path:    "artifacts[0].byte_length",
			ceiling: derivedArtifactCeiling,
			set: func(e *IngestionEnvelope, data []byte) {
				e.Artifacts[0].Bytes = data
				e.Artifacts[0].ByteLength = int64(len(data))
				e.Artifacts[0].Identity = independentIdentity(data)
				e.Provenance.OutputIdentities[1] = e.Artifacts[0].Identity
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			overLimit := bytes.Repeat([]byte("a"), int(tc.ceiling+1))
			atLimit := overLimit[:tc.ceiling:tc.ceiling]
			for _, boundary := range []struct {
				name  string
				bytes []byte
				valid bool
			}{
				{name: "at_limit", bytes: atLimit, valid: true},
				{name: "limit_plus_one", bytes: overLimit},
			} {
				t.Run(boundary.name, func(t *testing.T) {
					t.Logf("actual_bytes=%d sha256=%s", len(boundary.bytes), independentIdentity(boundary.bytes))
					envelope := validEnvelopeFixture()
					envelope.Provenance.Policy.MaxSourceBytes = qualifiedLargeCeiling + 1
					envelope.Provenance.Policy.MaxPrimaryBytes = qualifiedLargeCeiling + 1
					envelope.Provenance.Policy.MaxArtifactBytes = derivedArtifactCeiling + 1
					envelope.Provenance.Policy.MaxTotalArtifactBytes = derivedArtifactCeiling + 1
					tc.set(&envelope, boundary.bytes)

					report := VerifyEnvelope(envelope)
					if boundary.valid {
						if !report.Valid {
							t.Fatalf("VerifyEnvelope() rejected v1 byte ceiling: %#v", report)
						}
						return
					}
					if report.Valid || !hasFindingAtPath(report, VerificationPolicy, tc.path) {
						t.Fatalf("VerifyEnvelope() = %#v, want absolute v1 byte ceiling at %s", report, tc.path)
					}
				})
			}
		})
	}
}

func TestVerifyEnvelopeExplicitLargePolicyCrossesStandardByteCeiling(t *testing.T) {
	const standardCeiling = DefaultMaxSourceBytes
	const explicitLargeCeiling = 256 << 20
	overStandard := bytes.Repeat([]byte("a"), int(standardCeiling+1))
	t.Logf("actual_bytes=%d sha256=%s", len(overStandard), independentIdentity(overStandard))

	for _, tc := range []struct {
		name string
		set  func(*IngestionEnvelope, []byte)
	}{
		{
			name: "source",
			set: func(envelope *IngestionEnvelope, data []byte) {
				setEnvelopeSourceAndPrimaryBytes(envelope, data, envelope.Primary.Bytes)
			},
		},
		{
			name: "primary",
			set: func(envelope *IngestionEnvelope, data []byte) {
				setEnvelopeSourceAndPrimaryBytes(envelope, envelope.Source.Bytes, data)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			envelope.Provenance.Policy.MaxSourceBytes = explicitLargeCeiling
			envelope.Provenance.Policy.MaxPrimaryBytes = explicitLargeCeiling
			tc.set(&envelope, overStandard)

			if report := VerifyEnvelope(envelope); !report.Valid {
				t.Fatalf("VerifyEnvelope() rejected explicit large policy at 32 MiB + 1: %#v", report)
			}
		})
	}

	t.Run("standard_policy_still_rejects_limit_plus_one", func(t *testing.T) {
		envelope := validEnvelopeFixture()
		setEnvelopeSourceAndPrimaryBytes(&envelope, overStandard, envelope.Primary.Bytes)
		report := VerifyEnvelope(envelope)
		if report.Valid || !hasFindingAtPath(report, VerificationPolicy, "source.byte_length") {
			t.Fatalf("VerifyEnvelope() = %#v, want standard policy rejection", report)
		}
	})

	t.Run("contract_version_unchanged", func(t *testing.T) {
		if IngestionContractV1 != "inkbite.ingestion/v1" {
			t.Fatalf("IngestionContractV1 = %q", IngestionContractV1)
		}
	})
}

func TestVerifyEnvelopeRequiresCanonicalUTF8Primary(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
		valid bool
	}{
		{name: "valid multibyte", bytes: []byte("# café 🐱\n"), valid: true},
		{name: "invalid byte sequence", bytes: []byte{0xff, 0xfe}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			setEnvelopeSourceAndPrimaryBytes(&envelope, envelope.Source.Bytes, tc.bytes)
			report := VerifyEnvelope(envelope)
			if report.Valid != tc.valid {
				t.Fatalf("VerifyEnvelope().Valid = %v, want %v; findings: %#v", report.Valid, tc.valid, report.Findings)
			}
			if !tc.valid && !hasFindingAtPath(report, VerificationShape, "primary.bytes") {
				t.Fatalf("VerifyEnvelope() = %#v, want primary UTF-8 shape finding", report)
			}
		})
	}
}

func TestVerifyEnvelopeRequiresValidNonSelfRelationships(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*IngestionEnvelope)
		valid  bool
	}{
		{name: "canonical source relationships", mutate: func(*IngestionEnvelope) {}, valid: true},
		{name: "empty primary relations", mutate: func(e *IngestionEnvelope) { e.Primary.Relations = []ArtifactRelation{} }},
		{name: "empty derivative relations", mutate: func(e *IngestionEnvelope) { e.Artifacts[0].Relations = []ArtifactRelation{} }},
		{name: "self primary relation", mutate: func(e *IngestionEnvelope) {
			e.Primary.Relations = []ArtifactRelation{{Kind: RelationDerivedFrom, FromID: e.Primary.ArtifactID, ToID: e.Primary.ArtifactID}}
		}},
		{name: "self derivative relation", mutate: func(e *IngestionEnvelope) {
			id := e.Artifacts[0].ArtifactID
			e.Artifacts[0].Relations = []ArtifactRelation{{Kind: RelationDerivedFrom, FromID: id, ToID: id}}
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			tc.mutate(&envelope)
			report := VerifyEnvelope(envelope)
			if report.Valid != tc.valid {
				t.Fatalf("VerifyEnvelope().Valid = %v, want %v; findings: %#v", report.Valid, tc.valid, report.Findings)
			}
			if !tc.valid && !hasFinding(report, VerificationRelationship) {
				t.Fatalf("VerifyEnvelope() = %#v, want relationship finding", report)
			}
		})
	}
}

func TestVerifyEnvelopeRejectsRecursivelyEncodedUnsafeNames(t *testing.T) {
	tests := []struct {
		name     string
		safeName string
		valid    bool
	}{
		{name: "encoded dot segment", safeName: "%2e%2e/secret.pdf"},
		{name: "encoded forward separator", safeName: "folder%2fsecret.pdf"},
		{name: "encoded backslash separator", safeName: "folder%5csecret.pdf"},
		{name: "recursive traversal and separator", safeName: "%252e%252e%252fsecret.pdf"},
		{name: "encoded space", safeName: "report%20final.pdf", valid: true},
		{name: "encoded non-segment dot", safeName: "chapter%2Eone.txt", valid: true},
		{name: "recursive literal percent", safeName: "100%2525-complete.pdf", valid: true},
		{name: "relative nested name", safeName: "chapters/chapter-000001.txt", valid: true},
	}

	for _, surface := range []struct {
		name string
		set  func(*IngestionEnvelope, string)
	}{
		{name: "source", set: func(e *IngestionEnvelope, value string) { e.Source.SafeName = value }},
		{name: "artifact", set: func(e *IngestionEnvelope, value string) { e.Artifacts[0].SafeName = value }},
	} {
		t.Run(surface.name, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					envelope := validEnvelopeFixture()
					surface.set(&envelope, tc.safeName)
					report := VerifyEnvelope(envelope)
					if report.Valid != tc.valid {
						t.Fatalf("VerifyEnvelope().Valid = %v, want %v for safe name %q; findings: %#v", report.Valid, tc.valid, tc.safeName, report.Findings)
					}
					if !tc.valid && strings.Contains(fmt.Sprint(report.Findings), tc.safeName) {
						t.Fatalf("verification finding echoed rejected name: %#v", report.Findings)
					}
				})
			}
		})
	}
}

func TestVerifyEnvelopeDistinguishesAbsentAndPresentEmptyBytes(t *testing.T) {
	tests := []struct {
		name string
		path string
		set  func(*IngestionEnvelope, []byte)
	}{
		{
			name: "source",
			path: "source.bytes",
			set: func(envelope *IngestionEnvelope, data []byte) {
				setEnvelopeSourceAndPrimaryBytes(envelope, data, envelope.Primary.Bytes)
			},
		},
		{
			name: "primary",
			path: "primary.bytes",
			set: func(envelope *IngestionEnvelope, data []byte) {
				setEnvelopeSourceAndPrimaryBytes(envelope, envelope.Source.Bytes, data)
			},
		},
		{
			name: "derivative",
			path: "artifacts[0].bytes",
			set: func(envelope *IngestionEnvelope, data []byte) {
				setEnvelopeArtifactBytes(envelope, 0, data)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("go nil is absent", func(t *testing.T) {
				envelope := validEnvelopeFixture()
				tc.set(&envelope, nil)
				assertRejectedAtPath(t, VerifyEnvelope(envelope), VerificationShape, tc.path)
			})

			t.Run("go present empty", func(t *testing.T) {
				envelope := validEnvelopeFixture()
				tc.set(&envelope, []byte{})
				if report := VerifyEnvelope(envelope); !report.Valid {
					t.Fatalf("VerifyEnvelope() rejected present empty bytes: %#v", report)
				}
			})

			for _, jsonCase := range []struct {
				name  string
				value any
				omit  bool
			}{
				{name: "json missing", omit: true},
				{name: "json null", value: nil},
				{name: "json present empty", value: ""},
			} {
				t.Run(jsonCase.name, func(t *testing.T) {
					envelope := validEnvelopeFixture()
					tc.set(&envelope, []byte{})
					decoded := envelopeWithJSONByteValue(t, envelope, tc.name, jsonCase.value, jsonCase.omit)
					report := VerifyEnvelope(decoded)
					if jsonCase.name == "json present empty" {
						if !report.Valid {
							t.Fatalf("VerifyEnvelope() rejected JSON present-empty bytes: %#v", report)
						}
						return
					}
					assertRejectedAtPath(t, report, VerificationShape, tc.path)
				})
			}
		})
	}
}

func TestVerifyEnvelopeRejectsDecodedAbsolutePathsAcrossPublicMetadata(t *testing.T) {
	const secret = "TERMINAL_PATH_SENTINEL"
	forbidden := []struct {
		name  string
		value string
	}{
		{name: "posix", value: "/home/user/" + secret},
		{name: "windows drive", value: `C:\Users\user\` + secret},
		{name: "unc", value: `\\server\share\` + secret},
		{name: "encoded posix", value: "%2Fhome%2Fuser%2F" + secret},
		{name: "encoded windows drive", value: "C:%5CUsers%5Cuser%5C" + secret},
		{name: "encoded unc", value: "%5C%5Cserver%5Cshare%5C" + secret},
		{name: "recursively encoded posix", value: "%252Fhome%252Fuser%252F" + secret},
		{name: "labeled posix", value: "path=/home/user/" + secret},
		{name: "labeled windows drive", value: `path:C:\Users\user\` + secret},
		{name: "labeled unc", value: `location(\\server\share\` + secret + `)`},
		{name: "encoded traversal", value: "members%252F..%252F" + secret},
		{name: "recursively revealed query", value: "https://example.test/document%253Ftoken%253D" + secret},
		{name: "malformed candidate escape", value: "path%2G" + secret},
	}

	for _, surface := range terminalMetadataSurfaces() {
		t.Run(surface.name, func(t *testing.T) {
			for _, tc := range forbidden {
				t.Run(tc.name, func(t *testing.T) {
					envelope := validEnvelopeFixture()
					surface.set(&envelope, tc.value)
					report := VerifyEnvelope(envelope)
					if report.Valid {
						t.Fatalf("VerifyEnvelope() accepted unsafe metadata")
					}
					for _, finding := range surface.findings {
						if !hasFindingAtPath(report, finding.category, finding.path) {
							t.Fatalf("VerifyEnvelope() = %#v, want %s finding at %s", report, finding.category, finding.path)
						}
					}
					formatted := fmt.Sprint(report.Findings)
					if strings.Contains(formatted, secret) || strings.Contains(formatted, tc.value) {
						t.Fatalf("verification finding exposed rejected metadata: %q", formatted)
					}
				})
			}
		})
	}
}

func TestVerifyEnvelopeAllowsBoundedDecodedPublicMetadataControls(t *testing.T) {
	controls := []struct {
		name  string
		value string
	}{
		{name: "relative logical location", value: "members/chapter-000001.xhtml"},
		{name: "safe http locator", value: "http://example.test/public/document"},
		{name: "safe https locator", value: "https://example.test/public/document"},
		{name: "escaped space", value: "report%20final.pdf"},
		{name: "escaped literal percent", value: "100%2525-complete"},
		{name: "raw literal percent", value: "100%-complete"},
		{name: "depth 16", value: recursivelyPercentEncode("report final.pdf", 16)},
	}

	for _, surface := range terminalMetadataSurfaces() {
		t.Run(surface.name, func(t *testing.T) {
			for _, tc := range controls {
				t.Run(tc.name, func(t *testing.T) {
					envelope := validEnvelopeFixture()
					surface.set(&envelope, tc.value)
					if report := VerifyEnvelope(envelope); !report.Valid {
						t.Fatalf("VerifyEnvelope() rejected safe metadata: %#v", report)
					}
				})
			}

			t.Run("unresolved depth 17", func(t *testing.T) {
				envelope := validEnvelopeFixture()
				surface.set(&envelope, recursivelyPercentEncode("report final.pdf", 17))
				report := VerifyEnvelope(envelope)
				if report.Valid {
					t.Fatal("VerifyEnvelope() accepted percent encoding beyond the public limit")
				}
				for _, finding := range surface.findings {
					if !hasFindingAtPath(report, finding.category, finding.path) {
						t.Fatalf("VerifyEnvelope() = %#v, want %s finding at %s", report, finding.category, finding.path)
					}
				}
			})
		})
	}
}

func TestVerifyEnvelopeRejectsAccessibleCapacityAliases(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*IngestionEnvelope) func() bool
	}{
		{
			name: "source primary",
			setup: func(envelope *IngestionEnvelope) func() bool {
				backing := []byte("abcd")
				setEnvelopeSourceAndPrimaryBytes(envelope, backing[:2], backing[2:4:4])
				before := envelope.Primary.Bytes[0]
				return func() bool { envelope.Source.Bytes[:3][2] ^= 1; return envelope.Primary.Bytes[0] != before }
			},
		},
		{
			name: "source derivative",
			setup: func(envelope *IngestionEnvelope) func() bool {
				backing := []byte("abcd")
				setEnvelopeSourceAndPrimaryBytes(envelope, backing[:2], envelope.Primary.Bytes)
				setEnvelopeArtifactBytes(envelope, 0, backing[2:4:4])
				before := envelope.Artifacts[0].Bytes[0]
				return func() bool { envelope.Source.Bytes[:3][2] ^= 1; return envelope.Artifacts[0].Bytes[0] != before }
			},
		},
		{
			name: "primary derivative",
			setup: func(envelope *IngestionEnvelope) func() bool {
				backing := []byte("abcd")
				setEnvelopeSourceAndPrimaryBytes(envelope, envelope.Source.Bytes, backing[:2])
				setEnvelopeArtifactBytes(envelope, 0, backing[2:4:4])
				before := envelope.Artifacts[0].Bytes[0]
				return func() bool { envelope.Primary.Bytes[:3][2] ^= 1; return envelope.Artifacts[0].Bytes[0] != before }
			},
		},
		{
			name: "derivative siblings",
			setup: func(envelope *IngestionEnvelope) func() bool {
				addSecondArtifact(envelope)
				backing := []byte("abcd")
				setEnvelopeArtifactBytes(envelope, 0, backing[:2])
				setEnvelopeArtifactBytes(envelope, 1, backing[2:4:4])
				before := envelope.Artifacts[1].Bytes[0]
				return func() bool { envelope.Artifacts[0].Bytes[:3][2] ^= 1; return envelope.Artifacts[1].Bytes[0] != before }
			},
		},
		{
			name: "adjacent lengths overlapping capacity",
			setup: func(envelope *IngestionEnvelope) func() bool {
				backing := []byte("abcd")
				setEnvelopeSourceAndPrimaryBytes(envelope, backing[:2], backing[2:4])
				before := envelope.Primary.Bytes[0]
				return func() bool { envelope.Source.Bytes[:3][2] ^= 1; return envelope.Primary.Bytes[0] != before }
			},
		},
		{
			name: "zero length positive capacity",
			setup: func(envelope *IngestionEnvelope) func() bool {
				backing := []byte("ab")
				setEnvelopeSourceAndPrimaryBytes(envelope, backing[:0], backing[:2:2])
				before := envelope.Primary.Bytes[0]
				return func() bool { envelope.Source.Bytes[:1][0] ^= 1; return envelope.Primary.Bytes[0] != before }
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			crossesBoundary := tc.setup(&envelope)
			report := VerifyEnvelope(envelope)
			if report.Valid || !hasFindingAtPath(report, VerificationOwnership, "bytes") {
				t.Fatalf("VerifyEnvelope() = %#v, want ownership finding", report)
			}
			if !crossesBoundary() {
				t.Fatal("post-verification reslice mutation did not cross the object boundary")
			}
		})
	}
}

func TestVerifyEnvelopeAllowsIndependentAndClippedCapacityControls(t *testing.T) {
	controls := []struct {
		name    string
		source  []byte
		primary []byte
	}{
		{
			name:    "independent spare capacity",
			source:  append(make([]byte, 0, 16), "same"...),
			primary: append(make([]byte, 0, 16), "data"...),
		},
		{name: "independent equal content", source: []byte("same"), primary: []byte("same")},
	}
	backing := []byte("abcdefgh")
	controls = append(controls, struct {
		name    string
		source  []byte
		primary []byte
	}{name: "full slice clipped adjacency", source: backing[:4:4], primary: backing[4:8:8]})

	for _, tc := range controls {
		t.Run(tc.name, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			setEnvelopeSourceAndPrimaryBytes(&envelope, tc.source, tc.primary)
			if report := VerifyEnvelope(envelope); !report.Valid {
				t.Fatalf("VerifyEnvelope() rejected independent capacity: %#v", report)
			}
		})
	}

	for _, surface := range []string{"source", "primary", "derivative"} {
		t.Run("non-nil zero-capacity empty "+surface, func(t *testing.T) {
			envelope := validEnvelopeFixture()
			setEnvelopeBytesBySurface(&envelope, surface, make([]byte, 0))
			if report := VerifyEnvelope(envelope); !report.Valid {
				t.Fatalf("VerifyEnvelope() rejected present zero-capacity empty bytes: %#v", report)
			}
		})
	}
}

type terminalFinding struct {
	category VerificationCategory
	path     string
}

type terminalMetadataSurface struct {
	name     string
	set      func(*IngestionEnvelope, string)
	findings []terminalFinding
}

func terminalMetadataSurfaces() []terminalMetadataSurface {
	return []terminalMetadataSurface{
		{
			name: "warning category",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Warnings = []WarningRecord{{Category: value}}
			},
			findings: []terminalFinding{{VerificationWarning, "warnings[0]"}},
		},
		{
			name: "warning location",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Warnings = []WarningRecord{{Category: "source", Location: value}}
			},
			findings: []terminalFinding{{VerificationWarning, "warnings[0]"}},
		},
		{
			name: "warning detail",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Warnings = []WarningRecord{{Category: "source", Detail: value}}
			},
			findings: []terminalFinding{{VerificationWarning, "warnings[0]"}},
		},
		{
			name: "metadata fact value",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Provenance.StreamFacts = []MetadataFact{{Kind: "location", Value: value, Origin: MetadataOriginSource}}
			},
			findings: []terminalFinding{{VerificationShape, "provenance.stream_facts[0]"}},
		},
		{
			name: "relation occurrence",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Artifacts[0].Relations[0].Occurrence = value
			},
			findings: []terminalFinding{{VerificationRelationship, "artifacts[0].relations[0].occurrence"}},
		},
		{
			name: "provenance and policy component",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Provenance.Component = value
				envelope.Provenance.Policy.Component = value
			},
			findings: []terminalFinding{
				{VerificationPolicy, "provenance.policy.component"},
				{VerificationProvenance, "provenance.component"},
			},
		},
		{
			name: "attempt category",
			set: func(envelope *IngestionEnvelope, value string) {
				envelope.Provenance.Attempts[0].Category = value
			},
			findings: []terminalFinding{{VerificationProvenance, "provenance.attempts[0]"}},
		},
	}
}

func setEnvelopeBytesBySurface(envelope *IngestionEnvelope, surface string, data []byte) {
	switch surface {
	case "source":
		setEnvelopeSourceAndPrimaryBytes(envelope, data, envelope.Primary.Bytes)
	case "primary":
		setEnvelopeSourceAndPrimaryBytes(envelope, envelope.Source.Bytes, data)
	case "derivative":
		setEnvelopeArtifactBytes(envelope, 0, data)
	default:
		panic("unknown byte surface: " + surface)
	}
}

func setEnvelopeArtifactBytes(envelope *IngestionEnvelope, index int, data []byte) {
	envelope.Artifacts[index].Bytes = data
	envelope.Artifacts[index].ByteLength = int64(len(data))
	envelope.Artifacts[index].Identity = independentIdentity(data)
	envelope.Provenance.OutputIdentities[index+1] = envelope.Artifacts[index].Identity
}

func addSecondArtifact(envelope *IngestionEnvelope) {
	artifact := envelope.Artifacts[0]
	artifact.ArtifactID = "artifact-000002"
	artifact.Bytes = []byte("second artifact")
	artifact.Identity = independentIdentity(artifact.Bytes)
	artifact.ByteLength = int64(len(artifact.Bytes))
	artifact.SafeName = "page-000001-image-000002.png"
	artifact.Relations = append([]ArtifactRelation(nil), artifact.Relations...)
	artifact.Relations[0].ToID = artifact.ArtifactID
	artifact.Attributes = append([]MetadataFact(nil), artifact.Attributes...)
	envelope.Artifacts = append(envelope.Artifacts, artifact)
	envelope.Provenance.OutputIdentities = append(envelope.Provenance.OutputIdentities, artifact.Identity)
}

func envelopeWithJSONByteValue(t *testing.T, envelope IngestionEnvelope, surface string, value any, omit bool) IngestionEnvelope {
	t.Helper()
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatal(err)
	}
	var bytesObject map[string]any
	switch surface {
	case "source", "primary":
		bytesObject = object[surface].(map[string]any)
	case "derivative":
		bytesObject = object["artifacts"].([]any)[0].(map[string]any)
	default:
		t.Fatalf("unknown byte surface %q", surface)
	}
	if omit {
		delete(bytesObject, "bytes")
	} else {
		bytesObject["bytes"] = value
	}
	encoded, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var decoded IngestionEnvelope
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func recursivelyPercentEncode(value string, depth int) string {
	for range depth {
		var encoded strings.Builder
		for i := range len(value) {
			c := value[i]
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", rune(c)) {
				encoded.WriteByte(c)
				continue
			}
			fmt.Fprintf(&encoded, "%%%02X", c)
		}
		value = encoded.String()
	}
	return value
}

func assertRejectedAtPath(t *testing.T, report VerificationReport, category VerificationCategory, path string) {
	t.Helper()
	if report.Valid || !hasFindingAtPath(report, category, path) {
		t.Fatalf("VerifyEnvelope() = %#v, want %s finding at %s", report, category, path)
	}
}

func hasFindingAtPath(report VerificationReport, category VerificationCategory, path string) bool {
	for _, finding := range report.Findings {
		if finding.Category == category && finding.Path == path {
			return true
		}
	}
	return false
}

func envelopeWithArtifactCount(count int) IngestionEnvelope {
	envelope := validEnvelopeFixture()
	envelope.Artifacts = make([]ContentArtifact, count)
	envelope.Provenance.OutputIdentities = []ContentIdentity{envelope.Primary.Identity}
	emptyIdentity := independentIdentity(nil)
	for i := range envelope.Artifacts {
		id := fmt.Sprintf("artifact-%06d", i+1)
		safeName := fmt.Sprintf("empty-%06d.png", i+1)
		occurrence := fmt.Sprintf("artifact-%06d", i+1)
		envelope.Artifacts[i] = ContentArtifact{
			ArtifactID: id,
			Role:       ArtifactRoleEmbeddedImage,
			Bytes:      []byte{},
			Identity:   emptyIdentity,
			ByteLength: 0,
			MediaType:  "image/png",
			SafeName:   safeName,
			Relations: []ArtifactRelation{{
				Kind:       RelationDerivedFrom,
				FromID:     string(envelope.Source.Identity),
				ToID:       id,
				Occurrence: occurrence,
			}},
			Attributes: []MetadataFact{},
		}
		envelope.Provenance.OutputIdentities = append(envelope.Provenance.OutputIdentities, emptyIdentity)
	}
	return envelope
}
