package inkbite

import (
	"context"
	"io"
)

// Converter transforms an input stream into Markdown when it recognizes the
// content described by the provided StreamInfo.
type Converter interface {
	Name() string
	Priority() float64
	Accepts(ctx context.Context, r io.ReadSeeker, info StreamInfo, opts ConvertOptions) bool
	Convert(ctx context.Context, r io.ReadSeeker, info StreamInfo, opts ConvertOptions) (Result, error)
}

// DetailedConverter is an optional additive capability. Implementing the
// legacy Converter interface remains sufficient for registration and use.
type DetailedConverter interface {
	Converter
	ConvertDetailed(
		ctx context.Context,
		r io.ReadSeeker,
		info StreamInfo,
		opts ConvertOptions,
		policy IngestionPolicy,
	) (DetailedConversion, error)
}
