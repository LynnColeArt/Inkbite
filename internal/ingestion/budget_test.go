package ingestion

import (
	"errors"
	"math"
	"sync"
	"testing"
)

func TestDefaultLimitsMatchPublicPolicy(t *testing.T) {
	t.Parallel()

	want := Limits{
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
	}
	if got := DefaultLimits(); got != want {
		t.Fatalf("DefaultLimits() = %#v, want public defaults %#v", got, want)
	}
}

func tinyLimits() Limits {
	return Limits{
		MaxSourceBytes:         8,
		MaxPrimaryBytes:        7,
		MaxArtifacts:           2,
		MaxArtifactBytes:       5,
		MaxTotalArtifactBytes:  8,
		MaxContainerEntries:    2,
		MaxContainerEntryBytes: 6,
		MaxExpandedBytes:       9,
		MaxContainerDepth:      2,
		MaxExpansionRatio:      3,
	}
}

func TestRequestBudgetExactBoundaryMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*RequestBudget) error
	}{
		{name: "source at limit", run: func(b *RequestBudget) error { return b.AdmitSource(8) }},
		{name: "source one over", run: func(b *RequestBudget) error { return b.AdmitSource(9) }},
		{name: "primary at limit", run: func(b *RequestBudget) error { return b.AdmitPrimary(7) }},
		{name: "primary one over", run: func(b *RequestBudget) error { return b.AdmitPrimary(8) }},
		{name: "artifact item at limit", run: func(b *RequestBudget) error { return b.AdmitArtifact(5) }},
		{name: "artifact item one over", run: func(b *RequestBudget) error { return b.AdmitArtifact(6) }},
		{name: "artifact count at limit", run: func(b *RequestBudget) error {
			if err := b.AdmitArtifact(1); err != nil {
				return err
			}
			return b.AdmitArtifact(1)
		}},
		{name: "artifact count one over", run: func(b *RequestBudget) error {
			for range 3 {
				if err := b.AdmitArtifact(1); err != nil {
					return err
				}
			}
			return nil
		}},
		{name: "artifact aggregate at limit", run: func(b *RequestBudget) error {
			if err := b.AdmitArtifact(4); err != nil {
				return err
			}
			return b.AdmitArtifact(4)
		}},
		{name: "artifact aggregate one over", run: func(b *RequestBudget) error {
			if err := b.AdmitArtifact(4); err != nil {
				return err
			}
			return b.AdmitArtifact(5)
		}},
		{name: "container entry and ratio at limit", run: func(b *RequestBudget) error {
			return b.AdmitContainerEntry(6, 2, 6)
		}},
		{name: "container actual one over", run: func(b *RequestBudget) error {
			return b.AdmitContainerEntry(1, 3, 7)
		}},
		{name: "container declared one over", run: func(b *RequestBudget) error {
			return b.AdmitContainerEntry(7, 3, 1)
		}},
		{name: "container ratio one over", run: func(b *RequestBudget) error {
			return b.AdmitContainerEntry(4, 1, 4)
		}},
		{name: "container count one over", run: func(b *RequestBudget) error {
			for range 3 {
				if err := b.AdmitContainerEntry(1, 1, 1); err != nil {
					return err
				}
			}
			return nil
		}},
		{name: "container aggregate one over", run: func(b *RequestBudget) error {
			if err := b.AdmitContainerEntry(5, 5, 5); err != nil {
				return err
			}
			return b.AdmitContainerEntry(4, 4, 5)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			budget, err := NewRequestBudget(tinyLimits())
			if err != nil {
				t.Fatal(err)
			}
			err = tc.run(budget)
			wantLimit := stringsContain(tc.name, "one over")
			if wantLimit != errors.Is(err, ErrLimitExceeded) {
				t.Fatalf("error = %v, wantLimit = %v", err, wantLimit)
			}
		})
	}
}

func stringsContain(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

func TestRequestBudgetDepthAndActualBytesAreAuthoritative(t *testing.T) {
	t.Parallel()

	budget, err := NewRequestBudget(tinyLimits())
	if err != nil {
		t.Fatal(err)
	}
	if err := budget.EnterContainer(); err != nil {
		t.Fatal(err)
	}
	if err := budget.EnterContainer(); err != nil {
		t.Fatal(err)
	}
	if err := budget.EnterContainer(); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("depth +1 error = %v, want limit", err)
	}
	if err := budget.LeaveContainer(); err != nil {
		t.Fatal(err)
	}
	if err := budget.LeaveContainer(); err != nil {
		t.Fatal(err)
	}
	if err := budget.LeaveContainer(); !errors.Is(err, ErrIntegrityFailure) {
		t.Fatalf("depth underflow error = %v, want integrity", err)
	}

	budget, _ = NewRequestBudget(tinyLimits())
	if err := budget.AdmitContainerEntry(1, 3, 6); err != nil {
		t.Fatalf("low declared hint overrode valid actual bytes: %v", err)
	}
	if got := budget.Snapshot().ExpandedBytes; got != 6 {
		t.Fatalf("expanded bytes = %d, want actual 6", got)
	}
	if got := budget.RemainingExpandedBytes(); got != 3 {
		t.Fatalf("remaining expanded bytes = %d, want 3", got)
	}
	if got := budget.Limits(); got != tinyLimits() {
		t.Fatalf("Limits() = %#v, want immutable input", got)
	}
}

func TestRequestBudgetZeroBoundariesAndUnknownClaims(t *testing.T) {
	t.Parallel()

	limits := tinyLimits()
	limits.MaxArtifacts = 0
	limits.MaxContainerDepth = 0
	budget, err := NewRequestBudget(limits)
	if err != nil {
		t.Fatalf("zero count/depth policy rejected: %v", err)
	}
	if err := budget.AdmitArtifact(0); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("zero artifact policy error = %v, want limit", err)
	}
	if err := budget.EnterContainer(); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("zero depth policy error = %v, want limit", err)
	}

	budget, _ = NewRequestBudget(tinyLimits())
	if err := budget.AdmitContainerEntry(-1, 0, 0); err != nil {
		t.Fatalf("unknown declaration with empty entry: %v", err)
	}
	if err := budget.AdmitContainerEntry(-1, 0, 1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("nonempty zero-compressed ratio error = %v, want limit", err)
	}
}

func TestRejectedBudgetAdmissionsDoNotMutateState(t *testing.T) {
	t.Parallel()

	budget, _ := NewRequestBudget(tinyLimits())
	before := budget.Snapshot()
	rejections := []func() error{
		func() error { return budget.AdmitSource(9) },
		func() error { return budget.AdmitPrimary(8) },
		func() error { return budget.AdmitArtifact(6) },
		func() error { return budget.AdmitContainerEntry(7, 1, 7) },
	}
	for i, reject := range rejections {
		if err := reject(); !errors.Is(err, ErrLimitExceeded) {
			t.Fatalf("rejection %d error = %v, want limit", i, err)
		}
		if got := budget.Snapshot(); got != before {
			t.Fatalf("rejection %d mutated budget: got %#v want %#v", i, got, before)
		}
	}
}

func TestRequestBudgetCheckedArithmeticAndInvalidValues(t *testing.T) {
	t.Parallel()

	limits := tinyLimits()
	limits.MaxSourceBytes = math.MaxInt64
	budget, err := NewRequestBudget(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := budget.AdmitSource(math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	if err := budget.AdmitSource(1); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("overflow error = %v, want limit", err)
	}

	invalid := []func(*RequestBudget) error{
		func(b *RequestBudget) error { return b.AdmitSource(-1) },
		func(b *RequestBudget) error { return b.AdmitPrimary(-1) },
		func(b *RequestBudget) error { return b.AdmitArtifact(-1) },
		func(b *RequestBudget) error { return b.AdmitContainerEntry(-2, 1, 1) },
		func(b *RequestBudget) error { return b.AdmitContainerEntry(1, -1, 1) },
		func(b *RequestBudget) error { return b.AdmitContainerEntry(1, 1, -1) },
	}
	for i, run := range invalid {
		budget, _ := NewRequestBudget(tinyLimits())
		if err := run(budget); !errors.Is(err, ErrIntegrityFailure) {
			t.Fatalf("invalid case %d error = %v, want integrity", i, err)
		}
	}
}

func TestNewRequestBudgetRejectsInvalidPolicy(t *testing.T) {
	t.Parallel()

	tests := []Limits{
		{},
		func() Limits { v := tinyLimits(); v.MaxSourceBytes = -1; return v }(),
		func() Limits { v := tinyLimits(); v.MaxArtifacts = -1; return v }(),
		func() Limits { v := tinyLimits(); v.MaxContainerDepth = -1; return v }(),
		func() Limits { v := tinyLimits(); v.MaxExpansionRatio = math.Inf(1); return v }(),
		func() Limits { v := tinyLimits(); v.MaxExpansionRatio = math.NaN(); return v }(),
	}
	for i, limits := range tests {
		if _, err := NewRequestBudget(limits); !errors.Is(err, ErrPolicyViolation) {
			t.Fatalf("case %d error = %v, want policy", i, err)
		}
	}
}

func TestRequestBudgetsAreIsolatedUnderRace(t *testing.T) {
	t.Parallel()

	const requests = 64
	budgets := make([]*RequestBudget, requests)
	for i := range budgets {
		budgets[i], _ = NewRequestBudget(tinyLimits())
	}
	var wg sync.WaitGroup
	for _, budget := range budgets {
		budget := budget
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := budget.AdmitSource(8); err != nil {
				t.Errorf("AdmitSource: %v", err)
			}
			if err := budget.AdmitPrimary(7); err != nil {
				t.Errorf("AdmitPrimary: %v", err)
			}
			if err := budget.AdmitArtifact(5); err != nil {
				t.Errorf("AdmitArtifact: %v", err)
			}
			if err := budget.AdmitContainerEntry(6, 2, 6); err != nil {
				t.Errorf("AdmitContainerEntry: %v", err)
			}
		}()
	}
	wg.Wait()
	for i, budget := range budgets {
		want := BudgetSnapshot{SourceBytes: 8, PrimaryBytes: 7, ArtifactCount: 1, ArtifactBytes: 5, ContainerEntries: 1, ExpandedBytes: 6}
		if got := budget.Snapshot(); got != want {
			t.Fatalf("budget %d snapshot = %#v, want %#v", i, got, want)
		}
	}
}

func TestOneRequestBudgetIsAtomicUnderRace(t *testing.T) {
	t.Parallel()

	limits := tinyLimits()
	limits.MaxArtifacts = 64
	limits.MaxTotalArtifactBytes = 64
	budget, _ := NewRequestBudget(limits)
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := budget.AdmitArtifact(1); err != nil {
				t.Errorf("AdmitArtifact: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := budget.Snapshot(); got.ArtifactCount != 64 || got.ArtifactBytes != 64 {
		t.Fatalf("concurrent snapshot = %#v", got)
	}
}
