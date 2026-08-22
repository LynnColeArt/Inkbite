package inkbite

import (
	"context"
	"io"
	"net/http"
	"slices"
	"sort"
	"time"
)

// Engine coordinates source handling, stream typing, and converter dispatch.
type Engine struct {
	converters []Converter
	httpClient *http.Client
}

// New creates a new engine with default configuration.
func New(opts ...Option) *Engine {
	engine := &Engine{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(engine)
	}

	return engine
}

// RegisterConverter adds a converter to the engine registry.
func (e *Engine) RegisterConverter(converter Converter) {
	if converter == nil {
		return
	}

	e.converters = append(e.converters, converter)
}

// RegisteredConverters returns a snapshot of the registry sorted by priority.
func (e *Engine) RegisteredConverters() []Converter {
	registered := slices.Clone(e.converters)
	sort.SliceStable(registered, func(i, j int) bool {
		return registered[i].Priority() < registered[j].Priority()
	})

	return registered
}

// Convert dispatches a supported source to the first compatible converter.
func (e *Engine) Convert(
	ctx context.Context,
	src any,
	info *StreamInfo,
	opts ConvertOptions,
) (Result, error) {
	policy := DefaultIngestionPolicy()
	policy.Remote.Enabled = opts.EnableHTTP
	result, err := e.runIngestionPipeline(ctx, src, info, opts, policy, ingestionDispatchLegacy)
	if err != nil {
		return Result{}, err
	}
	return result.legacy, nil
}

// ConvertPath converts a local file path.
func (e *Engine) ConvertPath(
	ctx context.Context,
	path string,
	info *StreamInfo,
	opts ConvertOptions,
) (Result, error) {
	return e.Convert(ctx, path, info, opts)
}

// ConvertReader converts the full contents of an io.Reader.
func (e *Engine) ConvertReader(
	ctx context.Context,
	r io.Reader,
	info *StreamInfo,
	opts ConvertOptions,
) (Result, error) {
	return e.Convert(ctx, r, info, opts)
}

// ConvertURI converts a supported URI.
func (e *Engine) ConvertURI(
	ctx context.Context,
	uri string,
	info *StreamInfo,
	opts ConvertOptions,
) (Result, error) {
	return e.Convert(ctx, uri, info, opts)
}
