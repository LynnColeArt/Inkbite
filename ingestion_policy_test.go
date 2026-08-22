package inkbite

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDefaultIngestionPolicyIsStrictAndFullyMaterialized(t *testing.T) {
	got := DefaultIngestionPolicy()
	want := IngestionPolicy{
		MaxSourceBytes:         32 << 20,
		MaxPrimaryBytes:        32 << 20,
		MaxArtifacts:           256,
		MaxArtifactBytes:       8 << 20,
		MaxTotalArtifactBytes:  32 << 20,
		MaxContainerEntries:    256,
		MaxContainerEntryBytes: 8 << 20,
		MaxExpandedBytes:       32 << 20,
		MaxContainerDepth:      4,
		MaxExpansionRatio:      1000,
		Remote:                 RemotePolicy{Enabled: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultIngestionPolicy() = %#v, want %#v", got, want)
	}
}

func TestIngestionPolicyJSONUsesContractShape(t *testing.T) {
	policy := DefaultIngestionPolicy()
	policy.Remote.Enabled = true
	policy.Component = "ocr@1"

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"max_source_bytes":33554432,"max_primary_bytes":33554432,"max_artifacts":256,"max_artifact_bytes":8388608,"max_total_artifact_bytes":33554432,"max_container_entries":256,"max_container_entry_bytes":8388608,"max_expanded_bytes":33554432,"max_container_depth":4,"max_expansion_ratio":1000,"remote_enabled":true,"component":"ocr@1"}`
	if string(data) != want {
		t.Fatalf("Marshal() = %s, want %s", data, want)
	}

	var decoded IngestionPolicy
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, policy) {
		t.Fatalf("round trip = %#v, want %#v", decoded, policy)
	}
}

func TestIngestionPolicyRejectsInvalidJSON(t *testing.T) {
	var policy IngestionPolicy
	if err := json.Unmarshal([]byte(`{"max_source_bytes":`), &policy); err == nil {
		t.Fatal("expected malformed policy JSON to fail")
	}
}

func TestVerifierRejectsInvalidEffectivePolicy(t *testing.T) {
	envelope := validEnvelopeFixture()
	envelope.Provenance.Policy.MaxSourceBytes = 0
	report := VerifyEnvelope(envelope)
	if report.Valid || !hasFinding(report, VerificationPolicy) {
		t.Fatalf("VerifyEnvelope() = %#v, want policy finding", report)
	}
}
