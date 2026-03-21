package pdfconv

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 14

var (
	pdfExtensions = map[string]struct{}{
		".pdf": {},
	}
	pdfMIMETypes = map[string]struct{}{
		"application/pdf":   {},
		"application/x-pdf": {},
	}
	columnSplitRE = regexp.MustCompile(`\s{2,}`)
)

type extractor interface {
	Name() string
	Available() bool
	Extract(context.Context, []byte) (string, error)
}

// Converter extracts text and best-effort tables from PDFs.
type Converter struct {
	extractors []extractor
}

// New returns a PDF converter.
func New() *Converter {
	return &Converter{
		extractors: []extractor{
			pdfToTextExtractor{},
		},
	}
}

func (c *Converter) Name() string {
	return "pdf"
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
	if _, ok := pdfExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := pdfMIMETypes[info.MIMEType]; ok {
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
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.Result{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return inkbite.Result{}, err
	}

	extractor, err := c.chooseExtractor(opts.PDFBackend)
	if err != nil {
		return inkbite.Result{}, fmt.Errorf("pdf: %w", err)
	}

	text, err := extractor.Extract(ctx, data)
	if err != nil {
		return inkbite.Result{}, err
	}

	return inkbite.Result{
		Markdown: layoutToMarkdown(text),
	}, nil
}

func (c *Converter) chooseExtractor(requested string) (extractor, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" || requested == "auto" {
		for _, candidate := range c.extractors {
			if candidate.Available() {
				return candidate, nil
			}
		}
		return nil, fmt.Errorf("no PDF extractor backend available")
	}

	for _, candidate := range c.extractors {
		if candidate.Name() == requested {
			if !candidate.Available() {
				return nil, fmt.Errorf("requested PDF extractor %q is unavailable", requested)
			}
			return candidate, nil
		}
	}

	return nil, fmt.Errorf("unknown PDF extractor %q", requested)
}

type pdfToTextExtractor struct{}

func (pdfToTextExtractor) Name() string {
	return "pdftotext"
}

func (pdfToTextExtractor) Available() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

func (pdfToTextExtractor) Extract(ctx context.Context, data []byte) (string, error) {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-nopgbrk", "-enc", "UTF-8", "-", "-")
	cmd.Stdin = bytes.NewReader(data)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("pdftotext: %w", err)
	}

	return stdout.String(), nil
}

func layoutToMarkdown(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	input = strings.ReplaceAll(input, "\f", "\n")

	lines := strings.Split(input, "\n")
	var parts []string
	for i := 0; i < len(lines); {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			i++
			continue
		}

		cols := splitColumns(trimmed)
		if len(cols) >= 2 {
			j := i
			var block [][]string
			for j < len(lines) {
				next := strings.TrimSpace(strings.TrimRight(lines[j], " \t"))
				if next == "" {
					break
				}
				row := splitColumns(next)
				if len(row) != len(cols) {
					break
				}
				block = append(block, row)
				j++
			}

			if looksTabular(block) {
				parts = append(parts, renderTable(block))
				i = j
				continue
			}
		}

		var paragraph []string
		for i < len(lines) {
			current := strings.TrimSpace(strings.TrimRight(lines[i], " \t"))
			if current == "" {
				break
			}
			if len(splitColumns(current)) >= 2 {
				break
			}
			paragraph = append(paragraph, current)
			i++
		}
		if len(paragraph) > 0 {
			parts = append(parts, strings.Join(paragraph, "\n"))
			continue
		}

		i++
	}

	return strings.Join(parts, "\n\n")
}

func splitColumns(line string) []string {
	if line == "" {
		return nil
	}
	parts := columnSplitRE.Split(line, -1)
	if len(parts) < 2 {
		return nil
	}

	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		columns = append(columns, part)
	}

	if len(columns) < 2 {
		return nil
	}
	return columns
}

func looksTabular(block [][]string) bool {
	if len(block) < 2 {
		return false
	}
	width := len(block[0])
	if width < 2 || width > 8 {
		return false
	}

	longCells := 0
	totalCells := 0
	for _, row := range block {
		if len(row) != width {
			return false
		}
		for _, cell := range row {
			totalCells++
			if len(cell) > 48 {
				longCells++
			}
		}
	}

	return totalCells > 0 && longCells*3 < totalCells
}

func renderTable(rows [][]string) string {
	width := len(rows[0])
	var lines []string
	lines = append(lines, formatRow(rows[0], width))

	separator := make([]string, width)
	for idx := range separator {
		separator[idx] = "---"
	}
	lines = append(lines, formatRow(separator, width))

	for _, row := range rows[1:] {
		lines = append(lines, formatRow(row, width))
	}

	return strings.Join(lines, "\n")
}

func formatRow(row []string, width int) string {
	cells := make([]string, width)
	for idx := 0; idx < width; idx++ {
		if idx < len(row) {
			cells[idx] = escapeCell(row[idx])
		}
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

func escapeCell(value string) string {
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.TrimSpace(value)
}
