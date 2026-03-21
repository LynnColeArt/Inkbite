package text

import (
	"bytes"
	"context"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 100

var (
	textExtensions = map[string]struct{}{
		".csv":  {},
		".json": {},
		".log":  {},
		".md":   {},
		".rst":  {},
		".txt":  {},
		".toml": {},
		".xml":  {},
		".yaml": {},
		".yml":  {},
	}
	textMIMETypes = map[string]struct{}{
		"application/json":       {},
		"application/xml":        {},
		"application/x-yaml":     {},
		"application/yaml":       {},
		"text/csv":               {},
		"text/markdown":          {},
		"text/plain":             {},
		"text/xml":               {},
		"text/yaml":              {},
		"application/javascript": {},
	}
)

// Converter extracts text-like sources with minimal transformation.
type Converter struct{}

// New returns a plain text converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "text"
}

func (c *Converter) Priority() float64 {
	return priority
}

func (c *Converter) Accepts(
	_ context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) bool {
	if strings.HasPrefix(info.MIMEType, "text/") {
		return true
	}
	if _, ok := textMIMETypes[info.MIMEType]; ok {
		return true
	}
	if _, ok := textExtensions[info.Extension]; ok {
		return true
	}

	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false
	}
	defer func() {
		_, _ = r.Seek(cur, io.SeekStart)
	}()

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return false
	}

	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	if n == 0 {
		return true
	}

	sample := buf[:n]
	if bytes.IndexByte(sample, 0) >= 0 {
		return false
	}

	return utf8.Valid(sample)
}

func (c *Converter) Convert(
	_ context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) (inkbite.Result, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.Result{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return inkbite.Result{}, err
	}

	return inkbite.Result{
		Markdown: inkbite.DecodeText(data, info.Charset),
	}, nil
}
