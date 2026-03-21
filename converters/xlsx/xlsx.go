package xlsxconv

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 15

var (
	xlsxExtensions = map[string]struct{}{
		".xlsx": {},
	}
	xlsxMIMETypes = map[string]struct{}{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": {},
	}
)

// Converter renders workbook sheets as Markdown tables.
type Converter struct{}

// New returns an XLSX converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "xlsx"
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
	if _, ok := xlsxExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := xlsxMIMETypes[info.MIMEType]; ok {
		return true
	}
	return false
}

func (c *Converter) Convert(
	_ context.Context,
	r io.ReadSeeker,
	_ inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) (inkbite.Result, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.Result{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return inkbite.Result{}, err
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return inkbite.Result{}, err
	}
	defer func() {
		_ = workbook.Close()
	}()

	var parts []string
	for _, sheet := range workbook.GetSheetList() {
		rows, err := workbook.GetRows(sheet)
		if err != nil {
			return inkbite.Result{}, err
		}

		section := []string{"## " + sheet}
		if table := renderTable(rows); table != "" {
			section = append(section, table)
		}
		parts = append(parts, strings.Join(section, "\n\n"))
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
	}, nil
}

func renderTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if width == 0 {
		return ""
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
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.TrimSpace(value)
}
