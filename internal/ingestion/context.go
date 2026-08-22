package ingestion

import (
	"context"
	"errors"
)

// ErrCancellation classifies work stopped at a cooperative context boundary.
var ErrCancellation = errors.New("ingestion cancelled")

type requestBudgetContextKey struct{}

type classifiedError struct {
	category error
	cause    error
}

func (e *classifiedError) Error() string { return e.category.Error() }

func (e *classifiedError) Unwrap() error { return e.cause }

func (e *classifiedError) Is(target error) bool { return target == e.category }

func newClassifiedError(category, cause error) error {
	return &classifiedError{category: category, cause: cause}
}

// Checkpoint cheaply observes cancellation without exposing its possibly
// sensitive cause. A nil context is treated as an uncancelled context.
func Checkpoint(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return newClassifiedError(ErrCancellation, ctx.Err())
	default:
		return nil
	}
}

// WithRequestBudget attaches the request's single accounting authority to the
// conversion context. Reattaching the same ledger is harmless; replacing it
// would split accounting and therefore fails closed.
func WithRequestBudget(ctx context.Context, budget *RequestBudget) (context.Context, error) {
	if budget == nil {
		return nil, newClassifiedError(ErrPolicyViolation, nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if current, ok := RequestBudgetFromContext(ctx); ok {
		if current != budget {
			return nil, newClassifiedError(ErrIntegrityFailure, nil)
		}
		return ctx, nil
	}
	return context.WithValue(ctx, requestBudgetContextKey{}, budget), nil
}

// RequestBudgetFromContext returns the exact request ledger attached by the
// ingestion pipeline. It never allocates, substitutes, or resets a budget.
func RequestBudgetFromContext(ctx context.Context) (*RequestBudget, bool) {
	if ctx == nil {
		return nil, false
	}
	budget, ok := ctx.Value(requestBudgetContextKey{}).(*RequestBudget)
	return budget, ok && budget != nil
}
