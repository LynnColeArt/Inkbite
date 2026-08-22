package inkbite

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type pipelineProbeConverter struct {
	name       string
	priority   float64
	accept     bool
	legacy     Result
	detailed   DetailedConversion
	err        error
	mu         sync.Mutex
	events     []string
	legacyRuns int
	detailRuns int
}

func (c *pipelineProbeConverter) Name() string      { return c.name }
func (c *pipelineProbeConverter) Priority() float64 { return c.priority }
func (c *pipelineProbeConverter) Accepts(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions) bool {
	c.record("accept:" + readProbeByte(r))
	return c.accept
}
func (c *pipelineProbeConverter) Convert(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions) (Result, error) {
	c.mu.Lock()
	c.legacyRuns++
	c.mu.Unlock()
	c.record("legacy:" + readAllString(r))
	return c.legacy, c.err
}
func (c *pipelineProbeConverter) ConvertDetailed(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions, _ IngestionPolicy) (DetailedConversion, error) {
	c.mu.Lock()
	c.detailRuns++
	c.mu.Unlock()
	c.record("detailed:" + readAllString(r))
	return c.detailed, c.err
}
func (c *pipelineProbeConverter) record(event string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}
func (c *pipelineProbeConverter) snapshot() ([]string, int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.events), c.legacyRuns, c.detailRuns
}

type legacyPipelineProbe struct {
	name     string
	priority float64
	accept   bool
	result   Result
	err      error
	mu       sync.Mutex
	events   []string
}

func (c *legacyPipelineProbe) Name() string      { return c.name }
func (c *legacyPipelineProbe) Priority() float64 { return c.priority }
func (c *legacyPipelineProbe) Accepts(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions) bool {
	c.mu.Lock()
	c.events = append(c.events, "accept:"+readProbeByte(r))
	c.mu.Unlock()
	return c.accept
}
func (c *legacyPipelineProbe) Convert(_ context.Context, r io.ReadSeeker, _ StreamInfo, _ ConvertOptions) (Result, error) {
	c.mu.Lock()
	c.events = append(c.events, "convert:"+readAllString(r))
	c.mu.Unlock()
	return c.result, c.err
}

func readProbeByte(r io.Reader) string {
	buffer := make([]byte, 1)
	n, _ := r.Read(buffer)
	return string(buffer[:n])
}

func readAllString(r io.Reader) string {
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestSharedPipelineUsesDetailedCapabilityAndResetsIdentically(t *testing.T) {
	newEngine := func() (*Engine, *pipelineProbeConverter, *pipelineProbeConverter) {
		ignored := &pipelineProbeConverter{name: "ignored", priority: 1, accept: false}
		selected := &pipelineProbeConverter{
			name:     "selected",
			priority: 2,
			accept:   true,
			legacy:   Result{Markdown: "wrong legacy path", Title: "wrong"},
			detailed: DetailedConversion{Result: Result{Markdown: "  # shared\n\n\nbody  ", Title: "Shared title"}},
		}
		engine := New()
		engine.RegisterConverter(selected)
		engine.RegisterConverter(ignored)
		return engine, ignored, selected
	}

	detailedEngine, ignored, selected := newEngine()
	envelope, err := detailedEngine.Ingest(context.Background(), []byte("payload"), nil, IngestOptions{})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if got := string(envelope.Primary.Bytes); got != "# shared\n\nbody" {
		t.Fatalf("primary Markdown = %q", got)
	}
	if events, legacyRuns, detailRuns := selected.snapshot(); !slices.Equal(events, []string{"accept:p", "detailed:payload"}) || legacyRuns != 0 || detailRuns != 1 {
		t.Fatalf("detailed selection events/runs = %v/%d/%d", events, legacyRuns, detailRuns)
	}
	if events, _, _ := ignored.snapshot(); !slices.Equal(events, []string{"accept:p"}) {
		t.Fatalf("ignored detailed events = %v", events)
	}

	legacyEngine, ignored, selected := newEngine()
	legacy, err := legacyEngine.Convert(context.Background(), []byte("payload"), nil, ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if legacy != (Result{Markdown: "wrong legacy path", Title: "wrong"}) {
		t.Fatalf("legacy projection = %#v", legacy)
	}
	if events, legacyRuns, detailRuns := selected.snapshot(); !slices.Equal(events, []string{"accept:p", "legacy:payload"}) || legacyRuns != 1 || detailRuns != 0 {
		t.Fatalf("legacy selection events/runs = %v/%d/%d", events, legacyRuns, detailRuns)
	}
	if events, _, _ := ignored.snapshot(); !slices.Equal(events, []string{"accept:p"}) {
		t.Fatalf("ignored legacy events = %v", events)
	}
}

func TestIngestAdaptsLegacyConvertersAndRecordsOrderedFallback(t *testing.T) {
	unsupported := &legacyPipelineProbe{name: "unsupported", priority: 1, accept: false}
	failed := &legacyPipelineProbe{name: "failed", priority: 2, accept: true, err: errors.New("private backend detail")}
	winner := &legacyPipelineProbe{name: "winner", priority: 3, accept: true, result: Result{Markdown: "winner", Title: "title"}}
	engine := New()
	engine.RegisterConverter(winner)
	engine.RegisterConverter(failed)
	engine.RegisterConverter(unsupported)

	envelope, err := engine.Ingest(context.Background(), []byte("payload"), &StreamInfo{MIMEType: "text/plain"}, IngestOptions{})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	wantAttempts := []ConversionAttempt{
		{Converter: "unsupported", Outcome: AttemptUnsupported},
		{Converter: "failed", Outcome: AttemptFailed, Category: "converter"},
		{Converter: "winner", Outcome: AttemptSelected},
	}
	if !slices.Equal(envelope.Provenance.Attempts, wantAttempts) {
		t.Fatalf("attempts = %#v, want %#v", envelope.Provenance.Attempts, wantAttempts)
	}
	if len(envelope.Warnings) != 1 || envelope.Warnings[0].Category != "converter_fallback" || envelope.Warnings[0].Converter != "failed" || strings.Contains(fmt.Sprint(envelope.Warnings), "private") {
		t.Fatalf("fallback warnings = %#v", envelope.Warnings)
	}
	if report := VerifyEnvelope(envelope); !report.Valid {
		t.Fatalf("adapted legacy envelope failed verification: %#v", report.Findings)
	}
	if got := string(envelope.Primary.Bytes); got != "winner" || envelope.Provenance.Converter != "winner" {
		t.Fatalf("winner projection = %q/%q", got, envelope.Provenance.Converter)
	}
}

func TestIngestPreservesFactOriginsAndCallerPrecedence(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "source.bin")
	if err := os.WriteFile(path, []byte("plain text"), 0o600); err != nil {
		t.Fatal(err)
	}
	converter := &pipelineProbeConverter{
		name:     "facts",
		priority: 1,
		accept:   true,
		detailed: DetailedConversion{
			Result: Result{Markdown: "facts"},
			Facts:  []MetadataFact{{Kind: "charset", Value: "utf-16", Origin: MetadataOriginConverter}},
		},
	}
	engine := New()
	engine.RegisterConverter(converter)
	envelope, err := engine.Ingest(context.Background(), path, &StreamInfo{
		MIMEType: "application/json",
		Filename: "caller.json",
	}, IngestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []MetadataFact{
		{Kind: "charset", Value: "utf-16", Origin: MetadataOriginConverter},
		{Kind: "extension", Value: ".bin", Origin: MetadataOriginSource},
		{Kind: "filename", Value: "caller.json", Origin: MetadataOriginCaller},
		{Kind: "filename", Value: "source.bin", Origin: MetadataOriginSource},
		{Kind: "media_type", Value: "application/json", Origin: MetadataOriginCaller},
	}
	if !slices.Equal(envelope.Provenance.StreamFacts, want) {
		t.Fatalf("stream facts = %#v, want %#v", envelope.Provenance.StreamFacts, want)
	}
	sniffed, err := engine.Ingest(context.Background(), []byte("plain text"), nil, IngestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(sniffed.Provenance.StreamFacts, MetadataFact{Kind: "media_type", Value: "text/plain", Origin: MetadataOriginSniff}) ||
		!slices.Contains(sniffed.Provenance.StreamFacts, MetadataFact{Kind: "charset", Value: "utf-8", Origin: MetadataOriginSniff}) {
		t.Fatalf("sniff origins missing from %#v", sniffed.Provenance.StreamFacts)
	}
}

func TestIngestSealsExactOwnedArtifactsAndSelfVerifies(t *testing.T) {
	source := []byte("source")
	primaryBacking := []byte("primary")
	firstBacking := []byte("second")
	secondBacking := []byte("first")
	converter := &pipelineProbeConverter{
		name:     "detailed",
		priority: 1,
		accept:   true,
		detailed: DetailedConversion{
			Result: Result{Markdown: string(primaryBacking), Title: "title"},
			Artifacts: []DetailedArtifact{
				{Role: ArtifactRoleEmbeddedImage, Bytes: firstBacking, MediaType: "image/png", SafeName: "z.png", Occurrence: "page-2", Attributes: []MetadataFact{}},
				{Role: ArtifactRoleEmbeddedImage, Bytes: secondBacking, MediaType: "image/png", SafeName: "a.png", Occurrence: "page-1", Attributes: []MetadataFact{}},
			},
			Warnings: []WarningRecord{},
		},
	}
	engine := New()
	engine.RegisterConverter(converter)
	envelope, err := engine.Ingest(context.Background(), source, nil, IngestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source[0] = 'X'
	primaryBacking[0] = 'X'
	firstBacking[0] = 'X'
	secondBacking[0] = 'X'
	if string(envelope.Source.Bytes) != "source" || string(envelope.Primary.Bytes) != "primary" {
		t.Fatalf("source/primary alias leaked: %q/%q", envelope.Source.Bytes, envelope.Primary.Bytes)
	}
	if len(envelope.Artifacts) != 2 || string(envelope.Artifacts[0].Bytes) != "first" || string(envelope.Artifacts[1].Bytes) != "second" {
		t.Fatalf("canonical owned derivatives = %#v", envelope.Artifacts)
	}
	if envelope.Artifacts[0].ArtifactID != "artifact-000001" || envelope.Artifacts[1].ArtifactID != "artifact-000002" {
		t.Fatalf("artifact IDs = %q/%q", envelope.Artifacts[0].ArtifactID, envelope.Artifacts[1].ArtifactID)
	}
	if report := VerifyEnvelope(envelope); !report.Valid {
		t.Fatalf("returned envelope not self-verifying: %#v", report.Findings)
	}
	byteSlices := [][]byte{envelope.Source.Bytes, envelope.Primary.Bytes, envelope.Artifacts[0].Bytes, envelope.Artifacts[1].Bytes}
	for i := range byteSlices {
		for j := i + 1; j < len(byteSlices); j++ {
			if storageRangesOverlap(byteSlices[i], byteSlices[j]) {
				t.Fatalf("returned byte slices %d and %d share storage", i, j)
			}
		}
	}
}

func TestIngestSealsCapClippedOwnedByteMatrix(t *testing.T) {
	for _, size := range []int{0, 1, 5, 7} {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			source := make([]byte, size, size+9)
			derivative := make([]byte, size, size+11)
			for index := range source {
				source[index] = 's'
				derivative[index] = 'd'
			}
			primary := strings.Repeat("p", size)
			converter := &pipelineProbeConverter{
				name:     "capacity",
				priority: 1,
				accept:   true,
				detailed: DetailedConversion{
					Result: Result{Markdown: primary},
					Artifacts: []DetailedArtifact{{
						Role:       ArtifactRoleEmbeddedImage,
						Bytes:      derivative,
						MediaType:  "image/png",
						Occurrence: "page-1",
						Attributes: []MetadataFact{},
					}},
					Warnings: []WarningRecord{},
				},
			}
			policy := DefaultIngestionPolicy()
			limit := int64(size)
			if limit == 0 {
				limit = 1
			}
			policy.MaxSourceBytes = limit
			policy.MaxPrimaryBytes = limit
			policy.MaxArtifactBytes = limit
			policy.MaxTotalArtifactBytes = limit
			engine := New()
			engine.RegisterConverter(converter)
			envelope, err := engine.Ingest(context.Background(), source, nil, IngestOptions{Policy: policy})
			if err != nil {
				t.Fatal(err)
			}
			if report := VerifyEnvelope(envelope); !report.Valid {
				t.Fatalf("sealed matrix envelope failed verification: %#v", report.Findings)
			}
			objects := []struct {
				name     string
				bytes    []byte
				identity ContentIdentity
				length   int64
				wantByte byte
			}{
				{name: "source", bytes: envelope.Source.Bytes, identity: envelope.Source.Identity, length: envelope.Source.ByteLength, wantByte: 's'},
				{name: "primary", bytes: envelope.Primary.Bytes, identity: envelope.Primary.Identity, length: envelope.Primary.ByteLength, wantByte: 'p'},
				{name: "derivative", bytes: envelope.Artifacts[0].Bytes, identity: envelope.Artifacts[0].Identity, length: envelope.Artifacts[0].ByteLength, wantByte: 'd'},
			}
			for _, object := range objects {
				if object.bytes == nil || len(object.bytes) != size || cap(object.bytes) != len(object.bytes) ||
					object.length != int64(size) || object.identity != identityFor(object.bytes) {
					t.Fatalf("%s ownership/identity/length = len %d cap %d identity %q length %d", object.name, len(object.bytes), cap(object.bytes), object.identity, object.length)
				}
				if size > 0 && object.bytes[0] != object.wantByte {
					t.Fatalf("%s bytes = %q", object.name, object.bytes)
				}
			}
			if size > 0 {
				source[0] = 'X'
				derivative[0] = 'X'
				if envelope.Source.Bytes[0] != 's' || envelope.Artifacts[0].Bytes[0] != 'd' {
					t.Fatal("returned objects alias caller or converter buffers")
				}
			}
		})
	}
}

type alternatingArtifactConverter struct {
	calls atomic.Uint64
}

func (*alternatingArtifactConverter) Name() string      { return "alternating" }
func (*alternatingArtifactConverter) Priority() float64 { return 1 }
func (*alternatingArtifactConverter) Accepts(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) bool {
	return true
}
func (*alternatingArtifactConverter) Convert(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) (Result, error) {
	return Result{Markdown: "stable"}, nil
}
func (c *alternatingArtifactConverter) ConvertDetailed(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions, IngestionPolicy) (DetailedConversion, error) {
	artifacts := []DetailedArtifact{
		{
			Role:       ArtifactRoleEmbeddedImage,
			Bytes:      []byte("same bytes"),
			MediaType:  "image/png",
			SafeName:   "same.png",
			Occurrence: "page-1",
			Attributes: []MetadataFact{{Kind: "width", Value: "1", Origin: MetadataOriginConverter}},
		},
		{
			Role:       ArtifactRoleEmbeddedImage,
			Bytes:      []byte("same bytes"),
			MediaType:  "image/png",
			SafeName:   "same.png",
			Occurrence: "page-1",
			Attributes: []MetadataFact{{Kind: "width", Value: "2", Origin: MetadataOriginConverter}},
		},
	}
	if c.calls.Add(1)%2 == 0 {
		slices.Reverse(artifacts)
	}
	return DetailedConversion{Result: Result{Markdown: "stable"}, Artifacts: artifacts, Warnings: []WarningRecord{}}, nil
}

func TestIngestCanonicalArtifactOrderIsIDIndependent(t *testing.T) {
	engine := New()
	engine.RegisterConverter(&alternatingArtifactConverter{})
	var want []byte
	var canonical IngestionEnvelope
	for run := 0; run < 10; run++ {
		envelope, err := engine.Ingest(context.Background(), []byte("source"), nil, IngestOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if report := VerifyEnvelope(envelope); !report.Valid {
			t.Fatalf("run %d did not verify: %#v", run, report.Findings)
		}
		encoded, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			want = encoded
			canonical = envelope
		} else if !bytes.Equal(encoded, want) {
			t.Fatalf("run %d emitted converter-order-dependent envelope", run)
		}
	}

	reordered := canonical
	reordered.Artifacts = slices.Clone(canonical.Artifacts)
	slices.Reverse(reordered.Artifacts)
	for index := range reordered.Artifacts {
		artifactID := fmt.Sprintf("artifact-%06d", index+1)
		reordered.Artifacts[index].ArtifactID = artifactID
		reordered.Artifacts[index].Relations = slices.Clone(reordered.Artifacts[index].Relations)
		for relationIndex := range reordered.Artifacts[index].Relations {
			reordered.Artifacts[index].Relations[relationIndex].ToID = artifactID
		}
	}
	reordered.Provenance.OutputIdentities = []ContentIdentity{
		reordered.Primary.Identity,
		reordered.Artifacts[0].Identity,
		reordered.Artifacts[1].Identity,
	}
	report := VerifyEnvelope(reordered)
	if report.Valid || !slices.ContainsFunc(report.Findings, func(finding VerificationFinding) bool {
		return finding.Category == VerificationOrdering && finding.Path == "artifacts"
	}) {
		t.Fatalf("ID-renumbered noncanonical artifacts were accepted: %#v", report.Findings)
	}
}

func TestIngestRejectsSemanticallyAmbiguousArtifacts(t *testing.T) {
	duplicate := DetailedArtifact{
		Role:       ArtifactRoleEmbeddedImage,
		Bytes:      []byte("duplicate"),
		MediaType:  "image/png",
		SafeName:   "same.png",
		Occurrence: "page-1",
		Attributes: []MetadataFact{},
	}
	engine := New()
	engine.RegisterConverter(&pipelineProbeConverter{
		name:     "ambiguous",
		priority: 1,
		accept:   true,
		detailed: DetailedConversion{Result: Result{Markdown: "stable"}, Artifacts: []DetailedArtifact{duplicate, duplicate}},
	})
	envelope, err := engine.Ingest(context.Background(), []byte("source"), nil, IngestOptions{})
	if !errors.Is(err, ErrIntegrityFailure) || !reflect.DeepEqual(envelope, IngestionEnvelope{}) {
		t.Fatalf("ambiguous artifacts result/error = %#v/%v", envelope, err)
	}
}

func TestIngestPolicyBoundariesAndFailureReturnsZeroEnvelope(t *testing.T) {
	basePolicy := DefaultIngestionPolicy()
	basePolicy.MaxSourceBytes = 8
	basePolicy.MaxPrimaryBytes = 7
	basePolicy.MaxArtifacts = 1
	basePolicy.MaxArtifactBytes = 5
	basePolicy.MaxTotalArtifactBytes = 5
	conversion := func(primary, derivative string) *pipelineProbeConverter {
		return &pipelineProbeConverter{
			name:     "bounded",
			priority: 1,
			accept:   true,
			detailed: DetailedConversion{
				Result:    Result{Markdown: primary},
				Artifacts: []DetailedArtifact{{Role: ArtifactRoleEmbeddedImage, Bytes: []byte(derivative), MediaType: "image/png", Attributes: []MetadataFact{}}},
			},
		}
	}
	tests := []struct {
		name       string
		source     []byte
		primary    string
		derivative string
		policy     IngestionPolicy
		wantErr    error
	}{
		{name: "all at limit", source: []byte("12345678"), primary: "1234567", derivative: "12345", policy: basePolicy},
		{name: "source plus one", source: []byte("123456789"), primary: "ok", derivative: "ok", policy: basePolicy, wantErr: ErrLimitExceeded},
		{name: "primary plus one", source: []byte("ok"), primary: "12345678", derivative: "ok", policy: basePolicy, wantErr: ErrLimitExceeded},
		{name: "artifact plus one", source: []byte("ok"), primary: "ok", derivative: "123456", policy: basePolicy, wantErr: ErrLimitExceeded},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := New()
			engine.RegisterConverter(conversion(tc.primary, tc.derivative))
			got, err := engine.Ingest(context.Background(), tc.source, nil, IngestOptions{Policy: tc.policy})
			if tc.wantErr == nil {
				if err != nil || !VerifyEnvelope(got).Valid {
					t.Fatalf("at-limit result/error = %#v/%v", got, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, IngestionEnvelope{}) {
				t.Fatalf("failure returned envelope: %#v", got)
			}
		})
	}

	invalidPolicy := basePolicy
	invalidPolicy.MaxPrimaryBytes = 0
	engine := New()
	engine.RegisterConverter(conversion("ok", "ok"))
	got, err := engine.Ingest(context.Background(), []byte("ok"), nil, IngestOptions{Policy: invalidPolicy})
	if !errors.Is(err, ErrPolicyViolation) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
		t.Fatalf("invalid policy result/error = %#v/%v", got, err)
	}
	unsafePolicy := basePolicy
	unsafePolicy.Component = "https://component.invalid/?secret=SENSITIVE"
	got, err = engine.Ingest(context.Background(), []byte("ok"), nil, IngestOptions{Policy: unsafePolicy})
	if !errors.Is(err, ErrPolicyViolation) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
		t.Fatalf("unsafe component policy result/error = %#v/%v", got, err)
	}
}

func TestIngestPolicyIsTheOnlyRemoteAuthority(t *testing.T) {
	var calls atomic.Int64
	engine := New(WithHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, errors.New("unexpected remote call")
	})}))
	engine.RegisterConverter(&legacyPipelineProbe{name: "unused", priority: 1, accept: true, result: Result{Markdown: "unexpected"}})
	got, err := engine.Ingest(context.Background(), "https://example.com/brief.txt", nil, IngestOptions{
		ConvertOptions: ConvertOptions{EnableHTTP: true},
	})
	if !errors.Is(err, ErrRemoteDisabled) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
		t.Fatalf("disabled detailed remote result/error = %#v/%v", got, err)
	}
	if calls.Load() != 0 {
		t.Fatalf("converter options bypassed policy and made %d remote calls", calls.Load())
	}
}

func TestIngestRejectsInvalidDetailedValuesBeforeSuccess(t *testing.T) {
	tests := []struct {
		name       string
		conversion DetailedConversion
	}{
		{name: "unsafe warning", conversion: DetailedConversion{Result: Result{Markdown: "ok"}, Warnings: []WarningRecord{{Category: "bad\nwarning"}}}},
		{name: "invalid media type", conversion: DetailedConversion{Result: Result{Markdown: "ok"}, Artifacts: []DetailedArtifact{{Role: ArtifactRoleEmbeddedImage, Bytes: []byte("x"), MediaType: "IMAGE/PNG", Attributes: []MetadataFact{}}}}},
		{name: "invalid fact origin", conversion: DetailedConversion{Result: Result{Markdown: "ok"}, Facts: []MetadataFact{{Kind: "charset", Value: "utf-8", Origin: MetadataOriginCaller}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := New()
			engine.RegisterConverter(&pipelineProbeConverter{name: "invalid", priority: 1, accept: true, detailed: tc.conversion})
			got, err := engine.Ingest(context.Background(), []byte("source"), nil, IngestOptions{})
			if !errors.Is(err, ErrIntegrityFailure) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
				t.Fatalf("invalid detailed result/error = %#v/%v", got, err)
			}
		})
	}
}

func TestSharedPipelineFailureAndCancellationNeverReturnSuccess(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "policy", err: ErrPolicyViolation, want: ErrPolicyViolation},
		{name: "integrity", err: ErrIntegrityFailure, want: ErrIntegrityFailure},
		{name: "cancellation", err: context.Canceled, want: ErrCancellation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := New()
			engine.RegisterConverter(&pipelineProbeConverter{name: "terminal", priority: 1, accept: true, err: tc.err})
			got, err := engine.Ingest(context.Background(), []byte("source"), nil, IngestOptions{})
			if !errors.Is(err, tc.want) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
				t.Fatalf("Ingest result/error = %#v/%v", got, err)
			}
			legacy, legacyErr := engine.Convert(context.Background(), []byte("source"), nil, ConvertOptions{})
			if !errors.Is(legacyErr, tc.want) || legacy != (Result{}) {
				t.Fatalf("Convert result/error = %#v/%v", legacy, legacyErr)
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := New()
	engine.RegisterConverter(&pipelineProbeConverter{name: "unused", priority: 1, accept: true})
	got, err := engine.Ingest(ctx, []byte("source"), nil, IngestOptions{})
	if !errors.Is(err, ErrCancellation) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
		t.Fatalf("pre-cancel result/error = %#v/%v", got, err)
	}
}

type cooperativeDetailedConverter struct {
	started chan struct{}
	done    chan struct{}
}

func (*cooperativeDetailedConverter) Name() string      { return "cooperative" }
func (*cooperativeDetailedConverter) Priority() float64 { return 1 }
func (*cooperativeDetailedConverter) Accepts(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) bool {
	return true
}
func (*cooperativeDetailedConverter) Convert(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) (Result, error) {
	panic("legacy conversion path used for detailed converter")
}
func (c *cooperativeDetailedConverter) ConvertDetailed(ctx context.Context, _ io.ReadSeeker, _ StreamInfo, _ ConvertOptions, _ IngestionPolicy) (DetailedConversion, error) {
	close(c.started)
	defer close(c.done)
	<-ctx.Done()
	return DetailedConversion{}, ctx.Err()
}

func TestIngestJoinsCooperativeConverterCancellation(t *testing.T) {
	converter := &cooperativeDetailedConverter{started: make(chan struct{}), done: make(chan struct{})}
	engine := New()
	engine.RegisterConverter(converter)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	var envelope IngestionEnvelope
	var ingestErr error
	go func() {
		envelope, ingestErr = engine.Ingest(ctx, []byte("source"), nil, IngestOptions{})
		close(returned)
	}()
	<-converter.started
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("cooperative converter cancellation exceeded one second")
	}
	select {
	case <-converter.done:
	default:
		t.Fatal("Ingest returned before cooperative converter exited")
	}
	if !errors.Is(ingestErr, ErrCancellation) || !errors.Is(ingestErr, context.Canceled) || !reflect.DeepEqual(envelope, IngestionEnvelope{}) {
		t.Fatalf("cancelled converter result/error = %#v/%v", envelope, ingestErr)
	}
}

func TestAllLegacyEntryPointsProjectExactSharedValues(t *testing.T) {
	newEngine := func() *Engine {
		engine := New()
		engine.RegisterConverter(&legacyPipelineProbe{name: "legacy", priority: 1, accept: true, result: Result{Markdown: "  legacy\n\n\nbody  ", Title: "Exact title"}})
		return engine
	}
	temp := t.TempDir()
	path := filepath.Join(temp, "source.txt")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := Result{Markdown: "legacy\n\nbody", Title: "Exact title"}
	tests := []struct {
		name string
		run  func(*Engine) (Result, error)
	}{
		{name: "convert", run: func(e *Engine) (Result, error) {
			return e.Convert(context.Background(), []byte("payload"), nil, ConvertOptions{})
		}},
		{name: "path", run: func(e *Engine) (Result, error) {
			return e.ConvertPath(context.Background(), path, nil, ConvertOptions{})
		}},
		{name: "reader", run: func(e *Engine) (Result, error) {
			return e.ConvertReader(context.Background(), bytes.NewBufferString("payload"), nil, ConvertOptions{})
		}},
		{name: "URI", run: func(e *Engine) (Result, error) {
			return e.ConvertURI(context.Background(), "data:text/plain,payload", nil, ConvertOptions{})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.run(newEngine())
			if err != nil || got != want {
				t.Fatalf("legacy projection = %#v/%v, want %#v", got, err, want)
			}
		})
	}
}

func TestIngestIsDeterministicAndIsolatedForRepeatedConcurrentRequests(t *testing.T) {
	failed := &legacyPipelineProbe{name: "fallback", priority: 1, accept: true, err: errors.New("private failure")}
	converter := &pipelineProbeConverter{
		name:     "stable",
		priority: 2,
		accept:   true,
		detailed: DetailedConversion{
			Result: Result{Markdown: "stable", Title: "title"},
			Artifacts: []DetailedArtifact{{
				Role:       ArtifactRoleEmbeddedImage,
				Bytes:      []byte("artifact"),
				MediaType:  "image/png",
				Occurrence: "page-1",
				Attributes: []MetadataFact{},
			}},
			Warnings: []WarningRecord{},
		},
	}
	engine := New()
	engine.RegisterConverter(failed)
	engine.RegisterConverter(converter)

	baseline, err := engine.Ingest(context.Background(), []byte("source-a"), nil, IngestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	for run := 0; run < 100; run++ {
		got, err := engine.Ingest(context.Background(), []byte("source-a"), nil, IngestOptions{})
		if err != nil {
			t.Fatalf("repeat %d: %v", run, err)
		}
		encoded, err := json.Marshal(got)
		if err != nil || !bytes.Equal(encoded, want) {
			t.Fatalf("repeat %d not byte-identical: %v", run, err)
		}
	}

	tightPolicy := DefaultIngestionPolicy()
	tightPolicy.MaxSourceBytes = 8
	otherOptions := IngestOptions{Policy: tightPolicy}
	other, err := engine.Ingest(context.Background(), []byte("source-b"), nil, otherOptions)
	if err != nil {
		t.Fatal(err)
	}
	wantOther, err := json.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}

	const requests = 100
	errorsSeen := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			ctx := context.Background()
			source := []byte("source-a")
			options := IngestOptions{}
			wantEnvelope := want
			if index%3 == 1 {
				source = []byte("source-b")
				options = otherOptions
				wantEnvelope = wantOther
			}
			if index%3 == 2 {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			got, err := engine.Ingest(ctx, source, nil, options)
			if index%3 == 2 {
				if !errors.Is(err, ErrCancellation) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
					errorsSeen <- fmt.Errorf("cancelled mixed request returned %#v/%v", got, err)
				}
				return
			}
			if err != nil {
				errorsSeen <- err
				return
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				errorsSeen <- err
				return
			}
			if !bytes.Equal(encoded, wantEnvelope) {
				errorsSeen <- fmt.Errorf("nondeterministic envelope")
				return
			}
			got.Source.Bytes[0] ^= 0xff
			if baseline.Source.Bytes[0] != 's' {
				errorsSeen <- fmt.Errorf("request source alias")
			}
		}(i)
	}
	wg.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Error(err)
	}
}

func TestIngestConverterFailureAggregateIsTypedAndRedacted(t *testing.T) {
	engine := New()
	engine.RegisterConverter(&legacyPipelineProbe{name: "failed", priority: 1, accept: true, err: errors.New("SENSITIVE-BACKEND-STACK")})
	got, err := engine.Ingest(context.Background(), []byte("source"), nil, IngestOptions{})
	if !errors.Is(err, ErrConverterFailure) || !reflect.DeepEqual(got, IngestionEnvelope{}) {
		t.Fatalf("failed aggregate result/error = %#v/%v", got, err)
	}
	if strings.Contains(err.Error(), "SENSITIVE") {
		t.Fatalf("converter failure leaked detail: %q", err)
	}
}
