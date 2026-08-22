package ingestion

import (
	"math"
	"math/big"
	"sync"
)

const (
	defaultMaxSourceBytes         int64   = 32 << 20
	defaultMaxPrimaryBytes        int64   = 32 << 20
	defaultMaxArtifacts                   = 256
	defaultMaxArtifactBytes       int64   = 8 << 20
	defaultMaxTotalArtifactBytes  int64   = 32 << 20
	defaultMaxContainerEntries            = 256
	defaultMaxContainerEntryBytes int64   = 8 << 20
	defaultMaxExpandedBytes       int64   = 32 << 20
	defaultMaxContainerDepth              = 4
	defaultMaxExpansionRatio      float64 = 1000
)

// Limits is the internal effective resource boundary for one request. The root
// package explicitly translates the public IngestionPolicy into this type.
type Limits struct {
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
}

// DefaultLimits returns values exactly matching the public v1 default policy.
func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes:         defaultMaxSourceBytes,
		MaxPrimaryBytes:        defaultMaxPrimaryBytes,
		MaxArtifacts:           defaultMaxArtifacts,
		MaxArtifactBytes:       defaultMaxArtifactBytes,
		MaxTotalArtifactBytes:  defaultMaxTotalArtifactBytes,
		MaxContainerEntries:    defaultMaxContainerEntries,
		MaxContainerEntryBytes: defaultMaxContainerEntryBytes,
		MaxExpandedBytes:       defaultMaxExpandedBytes,
		MaxContainerDepth:      defaultMaxContainerDepth,
		MaxExpansionRatio:      defaultMaxExpansionRatio,
	}
}

// BudgetSnapshot is an atomic value copy of request-local accounting.
type BudgetSnapshot struct {
	SourceBytes      int64
	PrimaryBytes     int64
	ArtifactCount    int
	ArtifactBytes    int64
	ContainerEntries int
	ExpandedBytes    int64
	ContainerDepth   int
}

// RequestBudget owns all mutable accounting for exactly one ingestion attempt.
// It is safe to use from cooperating converter goroutines, but is never shared
// between requests.
type RequestBudget struct {
	limits Limits

	mu       sync.Mutex
	snapshot BudgetSnapshot
}

// NewRequestBudget validates and captures an immutable limits value.
func NewRequestBudget(limits Limits) (*RequestBudget, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	return &RequestBudget{limits: limits}, nil
}

// Limits returns the effective limits by value.
func (b *RequestBudget) Limits() Limits { return b.limits }

// Snapshot returns a consistent accounting snapshot by value.
func (b *RequestBudget) Snapshot() BudgetSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.snapshot
}

// AdmitSource accounts actual acquired source bytes.
func (b *RequestBudget) AdmitSource(actual int64) error {
	return b.admitBytes(&b.snapshot.SourceBytes, actual, b.limits.MaxSourceBytes)
}

// AdmitPrimary accounts actual normalized primary-output bytes.
func (b *RequestBudget) AdmitPrimary(actual int64) error {
	return b.admitBytes(&b.snapshot.PrimaryBytes, actual, b.limits.MaxPrimaryBytes)
}

// AdmitArtifact atomically accounts one derivative and its actual bytes.
func (b *RequestBudget) AdmitArtifact(actual int64) error {
	if actual < 0 {
		return newClassifiedError(ErrIntegrityFailure, nil)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if actual > b.limits.MaxArtifactBytes || b.snapshot.ArtifactCount >= b.limits.MaxArtifacts {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	next, ok := checkedAdd(b.snapshot.ArtifactBytes, actual)
	if !ok || next > b.limits.MaxTotalArtifactBytes {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	b.snapshot.ArtifactCount++
	b.snapshot.ArtifactBytes = next
	return nil
}

// EnterContainer accounts a recursive container level. A failed entry does
// not mutate the active depth.
func (b *RequestBudget) EnterContainer() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshot.ContainerDepth >= b.limits.MaxContainerDepth {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	b.snapshot.ContainerDepth++
	return nil
}

// LeaveContainer closes one active container level.
func (b *RequestBudget) LeaveContainer() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshot.ContainerDepth == 0 {
		return newClassifiedError(ErrIntegrityFailure, nil)
	}
	b.snapshot.ContainerDepth--
	return nil
}

// AdmitContainerEntry accounts one member using its declared size as an early
// rejection hint while enforcing every final limit against actual bytes.
// declared may be -1 when the format has no size claim.
func (b *RequestBudget) AdmitContainerEntry(declared, compressed, actual int64) error {
	if declared < -1 || compressed < 0 || actual < 0 {
		return newClassifiedError(ErrIntegrityFailure, nil)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshot.ContainerEntries >= b.limits.MaxContainerEntries ||
		declared > b.limits.MaxContainerEntryBytes || actual > b.limits.MaxContainerEntryBytes {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	if exceedsRatio(actual, compressed, b.limits.MaxExpansionRatio) {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	next, ok := checkedAdd(b.snapshot.ExpandedBytes, actual)
	if !ok || next > b.limits.MaxExpandedBytes {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	if declared >= 0 {
		claimedTotal, ok := checkedAdd(b.snapshot.ExpandedBytes, declared)
		if !ok || claimedTotal > b.limits.MaxExpandedBytes {
			return newClassifiedError(ErrLimitExceeded, nil)
		}
	}
	b.snapshot.ContainerEntries++
	b.snapshot.ExpandedBytes = next
	return nil
}

// RemainingExpandedBytes reports the actual-byte capacity left for bounded
// container reads. Callers still commit the exact result after EOF.
func (b *RequestBudget) RemainingExpandedBytes() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.limits.MaxExpandedBytes - b.snapshot.ExpandedBytes
}

func (b *RequestBudget) admitBytes(counter *int64, actual, limit int64) error {
	if actual < 0 {
		return newClassifiedError(ErrIntegrityFailure, nil)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	next, ok := checkedAdd(*counter, actual)
	if !ok || next > limit {
		return newClassifiedError(ErrLimitExceeded, nil)
	}
	*counter = next
	return nil
}

func validateLimits(limits Limits) error {
	if limits.MaxSourceBytes <= 0 || limits.MaxPrimaryBytes <= 0 || limits.MaxArtifacts < 0 ||
		limits.MaxArtifactBytes <= 0 || limits.MaxTotalArtifactBytes <= 0 ||
		limits.MaxContainerEntries <= 0 || limits.MaxContainerEntryBytes <= 0 ||
		limits.MaxExpandedBytes <= 0 || limits.MaxContainerDepth < 0 ||
		limits.MaxExpansionRatio < 1 || math.IsNaN(limits.MaxExpansionRatio) || math.IsInf(limits.MaxExpansionRatio, 0) {
		return newClassifiedError(ErrPolicyViolation, nil)
	}
	return nil
}

func checkedAdd(current, amount int64) (int64, bool) {
	if amount < 0 || current > math.MaxInt64-amount {
		return 0, false
	}
	return current + amount, true
}

func exceedsRatio(actual, compressed int64, limit float64) bool {
	if actual == 0 {
		return false
	}
	if compressed == 0 {
		return true
	}
	actualRatio := new(big.Rat).SetFrac(big.NewInt(actual), big.NewInt(compressed))
	limitRatio := new(big.Rat)
	limitRatio.SetFloat64(limit)
	return actualRatio.Cmp(limitRatio) > 0
}
