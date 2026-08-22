package xlsxconv

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/LynnColeArt/Inkbite"
	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
	"github.com/LynnColeArt/Inkbite/internal/ooxml"
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
	ctx context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	opts inkbite.ConvertOptions,
) (inkbite.Result, error) {
	return c.convert(ctx, r, info, opts, inkbite.DefaultIngestionPolicy())
}

// ConvertDetailed validates the original bounded archive with the caller's
// request ledger before handing those exact bytes to excelize.
func (c *Converter) ConvertDetailed(
	ctx context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	opts inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (inkbite.DetailedConversion, error) {
	result, err := c.convert(ctx, r, info, opts, policy)
	return inkbite.DetailedConversion{Result: result}, err
}

func (c *Converter) convert(
	ctx context.Context,
	r io.ReadSeeker,
	_ inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (inkbite.Result, error) {
	ctx, err := xlsxRequestContext(ctx, policy)
	if err != nil {
		return inkbite.Result{}, err
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return inkbite.Result{}, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.Result{}, err
	}

	data, err := internalingestion.ReadBounded(ctx, r, policy.MaxSourceBytes)
	if err != nil {
		return inkbite.Result{}, err
	}
	if _, err := ooxml.Open(ctx, data.Bytes); err != nil {
		return inkbite.Result{}, err
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return inkbite.Result{}, err
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(data.Bytes))
	if err != nil {
		return inkbite.Result{}, err
	}
	defer func() {
		_ = workbook.Close()
	}()

	var parts []string
	for _, sheet := range workbook.GetSheetList() {
		if err := internalingestion.Checkpoint(ctx); err != nil {
			return inkbite.Result{}, err
		}
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

func xlsxRequestContext(ctx context.Context, policy inkbite.IngestionPolicy) (context.Context, error) {
	if _, ok := internalingestion.RequestBudgetFromContext(ctx); ok {
		return ctx, nil
	}
	if policy == (inkbite.IngestionPolicy{}) {
		policy = inkbite.DefaultIngestionPolicy()
	}
	budget, err := internalingestion.NewRequestBudget(xlsxRequestLimits(policy))
	if err != nil {
		return nil, err
	}
	return internalingestion.WithRequestBudget(ctx, budget)
}

func xlsxRequestLimits(policy inkbite.IngestionPolicy) internalingestion.Limits {
	return internalingestion.Limits{
		MaxSourceBytes:         policy.MaxSourceBytes,
		MaxPrimaryBytes:        policy.MaxPrimaryBytes,
		MaxArtifacts:           policy.MaxArtifacts,
		MaxArtifactBytes:       policy.MaxArtifactBytes,
		MaxTotalArtifactBytes:  policy.MaxTotalArtifactBytes,
		MaxContainerEntries:    policy.MaxContainerEntries,
		MaxContainerEntryBytes: policy.MaxContainerEntryBytes,
		MaxExpandedBytes:       policy.MaxExpandedBytes,
		MaxContainerDepth:      policy.MaxContainerDepth,
		MaxExpansionRatio:      policy.MaxExpansionRatio,
	}
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
