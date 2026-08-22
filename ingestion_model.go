package inkbite

// IngestionContractV1 identifies the first detailed-ingestion wire contract.
const IngestionContractV1 = "inkbite.ingestion/v1"

// ContentIdentity is a canonical digest over exact bytes. Identity proves byte
// integrity; it does not prove origin, authorship, or authority.
type ContentIdentity string

// SourceKind describes how source bytes entered the ingestion boundary without
// exposing a path, URL payload, or other authority-bearing locator.
type SourceKind string

const (
	SourceKindBytes   SourceKind = "bytes"
	SourceKindReader  SourceKind = "reader"
	SourceKindFile    SourceKind = "file"
	SourceKindDataURI SourceKind = "data_uri"
	SourceKindRemote  SourceKind = "remote"
)

// ArtifactRole gives a retained artifact stable contract meaning.
type ArtifactRole string

const (
	ArtifactRolePrimaryMarkdown ArtifactRole = "primary_markdown"
	ArtifactRoleEmbeddedImage   ArtifactRole = "embedded_image"
)

// RelationKind describes an envelope-local relationship between retained
// values. A relationship never grants filesystem or network authority.
type RelationKind string

const (
	RelationDerivedFrom  RelationKind = "derived_from"
	RelationEmbeddedIn   RelationKind = "embedded_in"
	RelationReferencedBy RelationKind = "referenced_by"
)

// MetadataOrigin records where a canonical fact came from.
type MetadataOrigin string

const (
	MetadataOriginCaller    MetadataOrigin = "caller"
	MetadataOriginSource    MetadataOrigin = "source"
	MetadataOriginSniff     MetadataOrigin = "sniff"
	MetadataOriginConverter MetadataOrigin = "converter"
)

// MetadataFact is one ordered, non-sensitive scalar fact.
type MetadataFact struct {
	Kind   string         `json:"kind"`
	Value  string         `json:"value"`
	Origin MetadataOrigin `json:"origin"`
}

// ArtifactRelation binds an artifact to the source or to another artifact in
// the same envelope.
type ArtifactRelation struct {
	Kind       RelationKind `json:"kind"`
	FromID     string       `json:"from_id"`
	ToID       string       `json:"to_id"`
	Occurrence string       `json:"occurrence,omitempty"`
}

// SourceArtifact owns the exact acquired source bytes.
type SourceArtifact struct {
	Bytes      []byte          `json:"bytes"`
	Identity   ContentIdentity `json:"identity"`
	ByteLength int64           `json:"byte_length"`
	MediaType  string          `json:"media_type,omitempty"`
	SourceKind SourceKind      `json:"source_kind"`
	SafeName   string          `json:"safe_name,omitempty"`
}

// ContentArtifact owns one independently retainable output.
type ContentArtifact struct {
	ArtifactID string             `json:"artifact_id"`
	Role       ArtifactRole       `json:"role"`
	Bytes      []byte             `json:"bytes"`
	Identity   ContentIdentity    `json:"identity"`
	ByteLength int64              `json:"byte_length"`
	MediaType  string             `json:"media_type"`
	SafeName   string             `json:"safe_name,omitempty"`
	Relations  []ArtifactRelation `json:"relations"`
	Attributes []MetadataFact     `json:"attributes"`
}

// AttemptOutcome is a stable public conversion-attempt outcome.
type AttemptOutcome string

const (
	AttemptUnsupported AttemptOutcome = "unsupported"
	AttemptFailed      AttemptOutcome = "failed"
	AttemptSelected    AttemptOutcome = "selected"
)

// ConversionAttempt is safe dispatch provenance. It intentionally excludes
// backend error text.
type ConversionAttempt struct {
	Converter string         `json:"converter"`
	Outcome   AttemptOutcome `json:"outcome"`
	Category  string         `json:"category,omitempty"`
}

// WarningRecord makes permitted degradation visible without exposing payloads
// or backend diagnostic detail.
type WarningRecord struct {
	Category  string `json:"category"`
	Converter string `json:"converter,omitempty"`
	Location  string `json:"location,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// ConversionProvenance is deterministic evidence binding the exact source,
// effective policy, selected converter, and ordered outputs.
type ConversionProvenance struct {
	ContractVersion  string              `json:"contract_version"`
	SourceIdentity   ContentIdentity     `json:"source_identity"`
	Converter        string              `json:"converter"`
	Backend          string              `json:"backend,omitempty"`
	Component        string              `json:"component,omitempty"`
	StreamFacts      []MetadataFact      `json:"stream_facts"`
	Policy           IngestionPolicy     `json:"policy"`
	OutputIdentities []ContentIdentity   `json:"output_identities"`
	Attempts         []ConversionAttempt `json:"attempts"`
}

// IngestionEnvelope is the complete successful value for contract v1. All
// slices are ordered contract data; no behavior-bearing maps are present.
type IngestionEnvelope struct {
	ContractVersion string               `json:"contract_version"`
	Source          SourceArtifact       `json:"source"`
	Primary         ContentArtifact      `json:"primary"`
	Artifacts       []ContentArtifact    `json:"artifacts"`
	Provenance      ConversionProvenance `json:"provenance"`
	Warnings        []WarningRecord      `json:"warnings"`
}

// DetailedArtifact is raw, converter-supplied derivative material. The engine
// assigns IDs, identities, lengths, and final relationships when it seals the
// envelope.
type DetailedArtifact struct {
	Role       ArtifactRole   `json:"role"`
	Bytes      []byte         `json:"bytes"`
	MediaType  string         `json:"media_type"`
	SafeName   string         `json:"safe_name,omitempty"`
	Occurrence string         `json:"occurrence,omitempty"`
	Attributes []MetadataFact `json:"attributes"`
}

// DetailedConversion is the optional richer converter result. The engine,
// rather than the converter, remains the identity and validation authority.
type DetailedConversion struct {
	Result    Result             `json:"result"`
	Artifacts []DetailedArtifact `json:"artifacts"`
	Warnings  []WarningRecord    `json:"warnings"`
	Backend   string             `json:"backend,omitempty"`
	Component string             `json:"component,omitempty"`
	Facts     []MetadataFact     `json:"facts"`
}
