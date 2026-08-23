package inkbite

import "encoding/json"

const (
	DefaultMaxSourceBytes         int64   = 32 << 20
	DefaultMaxPrimaryBytes        int64   = 32 << 20
	DefaultMaxArtifacts                   = 256
	DefaultMaxArtifactBytes       int64   = 8 << 20
	DefaultMaxTotalArtifactBytes  int64   = 32 << 20
	DefaultMaxContainerEntries            = 256
	DefaultMaxContainerEntryBytes int64   = 8 << 20
	DefaultMaxExpandedBytes       int64   = 32 << 20
	DefaultMaxContainerDepth              = 4
	DefaultMaxExpansionRatio      float64 = 1000
	V1MaxSourceBytes              int64   = 256 << 20
	V1MaxPrimaryBytes             int64   = 256 << 20
	V1MaxArtifactBytes            int64   = 32 << 20
)

// RemotePolicy records remote authority. It is disabled by default.
type RemotePolicy struct {
	Enabled bool
}

// IngestionPolicy is the fully materialized, immutable-by-convention boundary
// applied to one ingestion request.
type IngestionPolicy struct {
	MaxSourceBytes         int64
	MaxPrimaryBytes        int64
	MaxArtifacts           int
	MaxArtifactBytes       int64
	MaxTotalArtifactBytes  int64
	MaxContainerEntries    int
	MaxContainerEntryBytes int64
	MaxExpandedBytes       int64
	MaxContainerDepth      int
	MaxExpansionRatio      float64
	Remote                 RemotePolicy
	Component              string
}

// DefaultIngestionPolicy returns the strict, fully materialized v1 defaults.
func DefaultIngestionPolicy() IngestionPolicy {
	return IngestionPolicy{
		MaxSourceBytes:         DefaultMaxSourceBytes,
		MaxPrimaryBytes:        DefaultMaxPrimaryBytes,
		MaxArtifacts:           DefaultMaxArtifacts,
		MaxArtifactBytes:       DefaultMaxArtifactBytes,
		MaxTotalArtifactBytes:  DefaultMaxTotalArtifactBytes,
		MaxContainerEntries:    DefaultMaxContainerEntries,
		MaxContainerEntryBytes: DefaultMaxContainerEntryBytes,
		MaxExpandedBytes:       DefaultMaxExpandedBytes,
		MaxContainerDepth:      DefaultMaxContainerDepth,
		MaxExpansionRatio:      DefaultMaxExpansionRatio,
		Remote:                 RemotePolicy{Enabled: false},
	}
}

type ingestionPolicyJSON struct {
	MaxSourceBytes         int64   `json:"max_source_bytes"`
	MaxPrimaryBytes        int64   `json:"max_primary_bytes"`
	MaxArtifacts           int     `json:"max_artifacts"`
	MaxArtifactBytes       int64   `json:"max_artifact_bytes"`
	MaxTotalArtifactBytes  int64   `json:"max_total_artifact_bytes"`
	MaxContainerEntries    int     `json:"max_container_entries"`
	MaxContainerEntryBytes int64   `json:"max_container_entry_bytes"`
	MaxExpandedBytes       int64   `json:"max_expanded_bytes"`
	MaxContainerDepth      int     `json:"max_container_depth"`
	MaxExpansionRatio      float64 `json:"max_expansion_ratio"`
	RemoteEnabled          bool    `json:"remote_enabled"`
	Component              string  `json:"component,omitempty"`
}

// MarshalJSON preserves the flat v1 schema while retaining the ergonomic
// policy.Remote.Enabled Go API.
func (p IngestionPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(ingestionPolicyJSON{
		MaxSourceBytes:         p.MaxSourceBytes,
		MaxPrimaryBytes:        p.MaxPrimaryBytes,
		MaxArtifacts:           p.MaxArtifacts,
		MaxArtifactBytes:       p.MaxArtifactBytes,
		MaxTotalArtifactBytes:  p.MaxTotalArtifactBytes,
		MaxContainerEntries:    p.MaxContainerEntries,
		MaxContainerEntryBytes: p.MaxContainerEntryBytes,
		MaxExpandedBytes:       p.MaxExpandedBytes,
		MaxContainerDepth:      p.MaxContainerDepth,
		MaxExpansionRatio:      p.MaxExpansionRatio,
		RemoteEnabled:          p.Remote.Enabled,
		Component:              p.Component,
	})
}

// UnmarshalJSON restores the v1 wire policy into its public Go shape.
func (p *IngestionPolicy) UnmarshalJSON(data []byte) error {
	var decoded ingestionPolicyJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = IngestionPolicy{
		MaxSourceBytes:         decoded.MaxSourceBytes,
		MaxPrimaryBytes:        decoded.MaxPrimaryBytes,
		MaxArtifacts:           decoded.MaxArtifacts,
		MaxArtifactBytes:       decoded.MaxArtifactBytes,
		MaxTotalArtifactBytes:  decoded.MaxTotalArtifactBytes,
		MaxContainerEntries:    decoded.MaxContainerEntries,
		MaxContainerEntryBytes: decoded.MaxContainerEntryBytes,
		MaxExpandedBytes:       decoded.MaxExpandedBytes,
		MaxContainerDepth:      decoded.MaxContainerDepth,
		MaxExpansionRatio:      decoded.MaxExpansionRatio,
		Remote:                 RemotePolicy{Enabled: decoded.RemoteEnabled},
		Component:              decoded.Component,
	}
	return nil
}
