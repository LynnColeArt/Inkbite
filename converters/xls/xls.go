package xlsconv

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/extrame/xls"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 16

var (
	xlsExtensions = map[string]struct{}{
		".xls": {},
	}
	xlsMIMETypes = map[string]struct{}{
		"application/vnd.ms-excel": {},
		"application/msexcel":      {},
		"application/xls":          {},
		"application/x-excel":      {},
	}
)

// Converter renders legacy XLS workbook sheets as Markdown tables.
type Converter struct{}

// New returns an XLS converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "xls"
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
	if _, ok := xlsExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := xlsMIMETypes[info.MIMEType]; ok {
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

	workbook, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil {
		return inkbite.Result{}, err
	}

	var parts []string
	for idx := 0; idx < workbook.NumSheets(); idx++ {
		sheet := workbook.GetSheet(idx)
		if sheet == nil {
			continue
		}

		rows := sheetRows(sheet)
		section := []string{"## " + strings.TrimSpace(sheet.Name)}
		if table := renderTable(rows); table != "" {
			section = append(section, table)
		}
		parts = append(parts, strings.Join(section, "\n\n"))
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
	}, nil
}

func sheetRows(sheet *xls.WorkSheet) [][]string {
	if sheet == nil {
		return nil
	}

	var rows [][]string
	for idx := 0; idx <= int(sheet.MaxRow); idx++ {
		row := sheet.Row(idx)
		if row == nil {
			continue
		}
		width := row.LastCol()
		if width <= 0 {
			continue
		}

		current := make([]string, width)
		nonEmpty := false
		for col := 0; col < width; col++ {
			value := strings.TrimSpace(row.Col(col))
			current[col] = value
			if value != "" {
				nonEmpty = true
			}
		}
		if nonEmpty {
			rows = append(rows, current)
		}
	}
	return rows
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
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.TrimSpace(value)
}
