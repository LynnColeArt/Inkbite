package zipconv

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 35

var (
	zipExtensions = map[string]struct{}{
		".zip": {},
	}
	zipMIMETypes = map[string]struct{}{
		"application/zip": {},
	}
)

// Converter recursively processes supported files inside ZIP archives.
type Converter struct {
	engine *inkbite.Engine
}

// New returns a ZIP converter.
func New(engine *inkbite.Engine) *Converter {
	return &Converter{engine: engine}
}

func (c *Converter) Name() string {
	return "zip"
}

func (c *Converter) Priority() float64 {
	return priority
}

func (c *Converter) Accepts(
	_ context.Context,
	_ io.ReadSeeker,
	info inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) bool {
	if _, ok := zipExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := zipMIMETypes[info.MIMEType]; ok {
		return true
	}
	return false
}

func (c *Converter) Convert(
	ctx context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	opts inkbite.ConvertOptions,
) (inkbite.Result, error) {
	if c.engine == nil {
		return inkbite.Result{}, fmt.Errorf("zip converter requires engine")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.Result{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return inkbite.Result{}, err
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return inkbite.Result{}, err
	}

	label := archiveLabel(info)
	parts := []string{fmt.Sprintf("Content from zip file `%s`", label)}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return inkbite.Result{}, err
		}
		entryData, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return inkbite.Result{}, readErr
		}
		if closeErr != nil {
			return inkbite.Result{}, closeErr
		}

		entryInfo := &inkbite.StreamInfo{
			Extension: strings.ToLower(filepath.Ext(file.Name)),
			Filename:  path.Base(file.Name),
		}

		result, err := c.engine.Convert(ctx, entryData, entryInfo, opts)
		if err != nil {
			if errors.Is(err, inkbite.ErrUnsupportedFormat) {
				continue
			}
			continue
		}
		if strings.TrimSpace(result.Markdown) == "" {
			continue
		}

		parts = append(parts, fmt.Sprintf("## File: %s\n\n%s", file.Name, strings.TrimSpace(result.Markdown)))
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
	}, nil
}

func archiveLabel(info inkbite.StreamInfo) string {
	switch {
	case info.URL != "":
		return info.URL
	case info.LocalPath != "":
		return info.LocalPath
	case info.Filename != "":
		return info.Filename
	default:
		return "archive.zip"
	}
}
