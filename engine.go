package inkbite

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/LynnColeArt/Inkbite/internal/normalize"
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
	resolved, err := e.resolveSource(ctx, src, info, opts)
	if err != nil {
		return Result{}, err
	}

	enriched, err := enrichStreamInfo(resolved.reader, resolved.info)
	if err != nil {
		return Result{}, err
	}

	return e.convertResolved(ctx, resolved.reader, enriched, opts)
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

func (e *Engine) convertResolved(
	ctx context.Context,
	reader io.ReadSeeker,
	info StreamInfo,
	opts ConvertOptions,
) (Result, error) {
	var attempts []ConversionError

	for _, converter := range e.RegisteredConverters() {
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			return Result{}, err
		}

		if !converter.Accepts(ctx, reader, info, opts) {
			continue
		}

		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			return Result{}, err
		}

		result, err := converter.Convert(ctx, reader, info, opts)
		if err != nil {
			if errors.Is(err, ErrUnsupportedFormat) {
				continue
			}
			attempts = append(attempts, ConversionError{
				Converter: converter.Name(),
				Err:       err,
			})
			continue
		}

		result.Markdown = normalize.Markdown(result.Markdown, opts.KeepDataURIs)
		return result, nil
	}

	if len(attempts) > 0 {
		return Result{}, FailedAttemptsError{Attempts: attempts}
	}

	return Result{}, UnsupportedFormatError{Info: info}
}
