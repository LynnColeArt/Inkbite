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

const (
	maxArchiveEntries               = 256
	maxArchiveEntryBytes     uint64 = 8 << 20
	maxArchiveTotalBytes     uint64 = 32 << 20
	maxArchiveRecursionDepth        = 4
)

var (
	zipExtensions = map[string]struct{}{
		".zip": {},
	}
	zipMIMETypes = map[string]struct{}{
		"application/zip": {},
	}
	errArchiveLimit = errors.New("zip archive limit exceeded")
)

// Converter recursively processes supported files inside ZIP archives.
type Converter struct {
	engine *inkbite.Engine
}

type archiveDepthKey struct{}

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
	if depth := archiveDepthFromContext(ctx); depth >= maxArchiveRecursionDepth {
		return inkbite.Result{}, fmt.Errorf("%w: recursion depth limit of %d", errArchiveLimit, maxArchiveRecursionDepth)
	}

	label := archiveLabel(info)
	parts := []string{fmt.Sprintf("Content from zip file `%s`", label)}
	var (
		fileCount  int
		totalBytes uint64
	)
	nestedCtx := context.WithValue(ctx, archiveDepthKey{}, archiveDepthFromContext(ctx)+1)
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		fileCount++
		if fileCount > maxArchiveEntries {
			return inkbite.Result{}, fmt.Errorf("%w: entry limit of %d", errArchiveLimit, maxArchiveEntries)
		}
		if file.UncompressedSize64 > maxArchiveEntryBytes {
			return inkbite.Result{}, fmt.Errorf("%w: entry %q exceeds size limit of %d bytes", errArchiveLimit, file.Name, maxArchiveEntryBytes)
		}
		totalBytes += file.UncompressedSize64
		if totalBytes > maxArchiveTotalBytes {
			return inkbite.Result{}, fmt.Errorf("%w: total uncompressed size limit of %d bytes", errArchiveLimit, maxArchiveTotalBytes)
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
		if uint64(len(entryData)) > maxArchiveEntryBytes {
			return inkbite.Result{}, fmt.Errorf("%w: entry %q exceeds size limit of %d bytes", errArchiveLimit, file.Name, maxArchiveEntryBytes)
		}

		entryInfo := &inkbite.StreamInfo{
			Extension: strings.ToLower(filepath.Ext(file.Name)),
			Filename:  path.Base(file.Name),
		}

		result, err := c.engine.Convert(nestedCtx, entryData, entryInfo, opts)
		if err != nil {
			if errors.Is(err, inkbite.ErrUnsupportedFormat) {
				continue
			}
			if errors.Is(err, errArchiveLimit) {
				return inkbite.Result{}, err
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

func archiveDepthFromContext(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	depth, _ := ctx.Value(archiveDepthKey{}).(int)
	return depth
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
