package inkbite

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported format")
	ErrInvalidSource     = errors.New("invalid source")
	ErrRemoteDisabled    = errors.New("remote fetching is disabled")
)

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
	if e.Info.MIMEType != "" || e.Info.Extension != "" {
		return fmt.Sprintf("%v: mime=%q extension=%q", ErrUnsupportedFormat, e.Info.MIMEType, e.Info.Extension)
	}

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
	return fmt.Sprintf("%s: %v", e.Converter, e.Err)
}

func (e ConversionError) Unwrap() error {
	return e.Err
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
