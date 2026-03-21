package csvconv

import (
	"context"
	"encoding/csv"
	"io"
	"strings"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 20

var (
	csvExtensions = map[string]struct{}{
		".csv": {},
	}
	csvMIMETypes = map[string]struct{}{
		"application/csv": {},
		"text/csv":        {},
	}
)

// Converter renders CSV content as a Markdown table.
type Converter struct{}

// New returns a CSV converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "csv"
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
	if _, ok := csvExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := csvMIMETypes[info.MIMEType]; ok {
		return true
	}
	return false
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

	reader := csv.NewReader(strings.NewReader(inkbite.DecodeText(data, info.Charset)))
	rows, err := reader.ReadAll()
	if err != nil {
		return inkbite.Result{}, err
	}
	if len(rows) == 0 {
		return inkbite.Result{}, nil
	}

	width := len(rows[0])
	if width == 0 {
		return inkbite.Result{}, nil
	}

	var lines []string
	lines = append(lines, formatRow(rows[0], width))

	separator := make([]string, width)
	for i := range separator {
		separator[i] = "---"
	}
	lines = append(lines, formatRow(separator, width))

	for _, row := range rows[1:] {
		lines = append(lines, formatRow(row, width))
	}

	return inkbite.Result{
		Markdown: strings.Join(lines, "\n"),
	}, nil
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
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.TrimSpace(value)
}
