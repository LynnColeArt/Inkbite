package inkbite

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported format")
	ErrInvalidSource     = errors.New("invalid source")
	ErrRemoteDisabled    = errors.New("remote fetching is disabled")
	ErrRemoteTooLarge    = errors.New("remote response exceeds size limit")
	ErrMalformedInput    = errors.New("malformed input")
	ErrLimitExceeded     = errors.New("ingestion limit exceeded")
	ErrPolicyViolation   = errors.New("ingestion policy violation")
	ErrIntegrityFailure  = errors.New("ingestion integrity failure")
	ErrCancellation      = errors.New("ingestion cancelled")
	ErrConverterFailure  = errors.New("converter failure")
)

// FailureCategory is the stable public class of a failed ingestion outcome.
type FailureCategory string

const (
	FailureUnsupported  FailureCategory = "unsupported"
	FailureMalformed    FailureCategory = "malformed"
	FailureLimit        FailureCategory = "limit"
	FailurePolicy       FailureCategory = "policy"
	FailureIntegrity    FailureCategory = "integrity"
	FailureCancellation FailureCategory = "cancellation"
	FailureConverter    FailureCategory = "converter"
)

// FailureError exposes a typed failure category while retaining a trusted
// internal cause for programmatic inspection. Error deliberately omits Cause.
type FailureError struct {
	Category  FailureCategory
	Operation string
	Cause     error
}

func (e *FailureError) Error() string {
	if e == nil {
		return "ingestion failed"
	}
	if isSafeLabel(e.Operation) {
		return fmt.Sprintf("%s: %s", e.Operation, failureSentinel(e.Category))
	}
	return failureSentinel(e.Category).Error()
}

func (e *FailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *FailureError) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == failureSentinel(e.Category)
}

func failureSentinel(category FailureCategory) error {
	switch category {
	case FailureUnsupported:
		return ErrUnsupportedFormat
	case FailureMalformed:
		return ErrMalformedInput
	case FailureLimit:
		return ErrLimitExceeded
	case FailurePolicy:
		return ErrPolicyViolation
	case FailureIntegrity:
		return ErrIntegrityFailure
	case FailureCancellation:
		return ErrCancellation
	case FailureConverter:
		return ErrConverterFailure
	default:
		return errors.New("ingestion failed")
	}
}

func isSafeLabel(value string) bool {
	if value == "" || len(value) > 80 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// InvalidSourceError reports an unsupported source type.
type InvalidSourceError struct {
	Value any
}

func (e InvalidSourceError) Error() string {
	return fmt.Sprintf("%v: %T", ErrInvalidSource, e.Value)
}

func (e InvalidSourceError) Unwrap() error {
	return ErrInvalidSource
}

// UnsupportedFormatError reports that no registered converter could handle the input.
type UnsupportedFormatError struct {
	Info StreamInfo
}

func (e UnsupportedFormatError) Error() string {
	return ErrUnsupportedFormat.Error()
}

func (e UnsupportedFormatError) Unwrap() error {
	return ErrUnsupportedFormat
}

// ConversionError wraps a converter-specific failure.
type ConversionError struct {
	Converter string
	Err       error
}

func (e ConversionError) Error() string {
	if isSafeLabel(e.Converter) {
		if detail, ok := safeErrorDetail(e.Err); ok {
			return fmt.Sprintf("%s: %s", e.Converter, detail)
		}
		return fmt.Sprintf("%s: %v", e.Converter, ErrConverterFailure)
	}
	return ErrConverterFailure.Error()
}

func (e ConversionError) Unwrap() error {
	return e.Err
}

func (e ConversionError) Is(target error) bool {
	return target == ErrConverterFailure
}

func safeErrorDetail(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	for _, public := range []error{
		ErrUnsupportedFormat,
		ErrInvalidSource,
		ErrRemoteDisabled,
		ErrRemoteTooLarge,
		ErrMalformedInput,
		ErrLimitExceeded,
		ErrPolicyViolation,
		ErrIntegrityFailure,
		ErrCancellation,
	} {
		if errors.Is(err, public) {
			return public.Error(), true
		}
	}
	detail := err.Error()
	if !safeArchiveLimitDetail(detail) {
		return "", false
	}
	for _, r := range detail {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return detail, true
}

func safeArchiveLimitDetail(detail string) bool {
	for strings.HasPrefix(detail, "conversion failed: zip: ") {
		detail = strings.TrimPrefix(detail, "conversion failed: zip: ")
	}
	for _, prefix := range []string{
		"zip archive limit exceeded: entry limit of ",
		"zip archive limit exceeded: recursion depth limit of ",
	} {
		if value, found := strings.CutPrefix(detail, prefix); found {
			return decimalDigits(value)
		}
	}
	return false
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// FailedAttemptsError aggregates converter failures encountered during dispatch.
type FailedAttemptsError struct {
	Attempts []ConversionError
}

func (e FailedAttemptsError) Error() string {
	if len(e.Attempts) == 0 {
		return "conversion failed"
	}

	parts := make([]string, 0, len(e.Attempts))
	for _, attempt := range e.Attempts {
		parts = append(parts, attempt.Error())
	}

	return "conversion failed: " + strings.Join(parts, "; ")
}

func (e FailedAttemptsError) Unwrap() []error {
	if len(e.Attempts) == 0 {
		return nil
	}

	errs := make([]error, 0, len(e.Attempts))
	for _, attempt := range e.Attempts {
		errs = append(errs, attempt)
	}
	return errs
}
