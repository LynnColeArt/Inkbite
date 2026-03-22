package xlsconv

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/shakinm/xlsReader/xls"
	"github.com/shakinm/xlsReader/xls/structure"

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

	workbook, err := xls.OpenReader(bytes.NewReader(data))
	if err != nil {
		return inkbite.Result{}, err
	}

	var parts []string
	for idx := 0; idx < workbook.GetNumberSheets(); idx++ {
		sheet, err := workbook.GetSheet(idx)
		if err != nil || sheet == nil {
			continue
		}

		rows := sheetRows(&workbook, sheet)
		name := strings.TrimSpace(sheet.GetName())
		if name == "" {
			name = "Sheet " + strconv.Itoa(idx+1)
		}
		section := []string{"## " + name}
		if table := renderTable(rows); table != "" {
			section = append(section, table)
		}
		parts = append(parts, strings.Join(section, "\n\n"))
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
	}, nil
}

func sheetRows(workbook *xls.Workbook, sheet *xls.Sheet) [][]string {
	if sheet == nil {
		return nil
	}

	var rows [][]string
	for idx := 0; idx < sheet.GetNumberRows(); idx++ {
		row, err := sheet.GetRow(idx)
		if err != nil || row == nil {
			continue
		}
		cols := row.GetCols()
		if len(cols) == 0 {
			continue
		}

		current := make([]string, len(cols))
		nonEmpty := false
		for col, cell := range cols {
			value := strings.TrimSpace(formattedCell(workbook, cell))
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

func formattedCell(workbook *xls.Workbook, cell structure.CellData) string {
	if workbook == nil || cell == nil {
		return ""
	}

	raw := strings.TrimSpace(cell.GetString())
	xf := workbook.GetXFbyIndex(cell.GetXFIndex())
	format := workbook.GetFormatByIndex(xf.GetFormatIndex())
	formatted := strings.TrimSpace(format.GetFormatString(cell))
	if formatted != "" {
		return formatted
	}
	return raw
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
