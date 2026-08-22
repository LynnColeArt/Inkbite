package zipconv

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
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
	detailed, err := c.convert(ctx, r, info, opts, inkbite.DefaultIngestionPolicy())
	return detailed.Result, err
}

// ConvertDetailed uses the request's shared container ledger and exposes
// permitted member degradation as stable warnings.
func (c *Converter) ConvertDetailed(
	ctx context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	opts inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (inkbite.DetailedConversion, error) {
	return c.convert(ctx, r, info, opts, policy)
}

func (c *Converter) convert(
	ctx context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	opts inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (conversion inkbite.DetailedConversion, err error) {
	if c.engine == nil {
		return conversion, fmt.Errorf("zip converter requires engine")
	}
	if policy == (inkbite.IngestionPolicy{}) {
		policy = inkbite.DefaultIngestionPolicy()
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return conversion, zipIntegrityFailure()
	}
	budget, ownsBudget, err := zipRequestBudget(ctx, policy)
	if err != nil {
		return conversion, err
	}
	owned, err := internalingestion.ReadBounded(ctx, r, policy.MaxSourceBytes)
	if err != nil {
		return conversion, err
	}
	if ownsBudget {
		if err := budget.AdmitSource(owned.ByteLength); err != nil {
			return conversion, err
		}
	}
	reader, err := zip.NewReader(bytes.NewReader(owned.Bytes), owned.ByteLength)
	if err != nil {
		return conversion, zipMalformedInput()
	}
	if err := budget.EnterContainer(); err != nil {
		return conversion, err
	}
	defer func() {
		leaveErr := budget.LeaveContainer()
		if err == nil && leaveErr != nil {
			err = leaveErr
			conversion = inkbite.DetailedConversion{}
		}
	}()

	label := archiveLabel(info)
	parts := []string{fmt.Sprintf("Content from zip file `%s`", label)}
	warnings := make([]inkbite.WarningRecord, 0)
	canonicalNames := make(map[string]struct{}, len(reader.File))
	for _, file := range reader.File {
		if err := internalingestion.Checkpoint(ctx); err != nil {
			return conversion, err
		}
		isDirectory := file.FileInfo().IsDir()
		candidate := file.Name
		if isDirectory {
			candidate = strings.TrimSuffix(candidate, "/")
		}
		name, err := internalingestion.CanonicalArchivePath(candidate)
		if err != nil || !validZIPEntryType(file, isDirectory) {
			return conversion, zipMalformedInput()
		}
		collisionKey := strings.ToLower(name)
		if _, exists := canonicalNames[collisionKey]; exists {
			return conversion, zipMalformedInput()
		}
		canonicalNames[collisionKey] = struct{}{}

		if err := preflightZIPEntry(file, budget); err != nil {
			return conversion, err
		}
		if isDirectory {
			if err := budget.AdmitContainerEntry(int64(file.UncompressedSize64), int64(file.CompressedSize64), 0); err != nil {
				return conversion, err
			}
			continue
		}
		entryData, err := readZIPEntry(ctx, file, budget)
		if err != nil {
			return conversion, err
		}
		entryInfo := &inkbite.StreamInfo{
			Extension: strings.ToLower(path.Ext(name)),
			Filename:  path.Base(name),
		}

		envelope, err := c.engine.Ingest(ctx, entryData, entryInfo, inkbite.IngestOptions{Policy: policy, ConvertOptions: opts})
		if err != nil {
			if errors.Is(err, inkbite.ErrUnsupportedFormat) {
				warnings = append(warnings, inkbite.WarningRecord{
					Category:  "unsupported_member",
					Converter: c.Name(),
					Location:  name,
				})
				continue
			}
			if terminalZIPMemberError(err) {
				return conversion, err
			}
			warnings = append(warnings, inkbite.WarningRecord{
				Category:  "member_conversion_failed",
				Converter: c.Name(),
				Location:  name,
			})
			continue
		}
		for _, warning := range envelope.Warnings {
			if warning.Location == "" {
				warning.Location = name
			}
			warnings = append(warnings, warning)
		}
		markdown := strings.TrimSpace(string(envelope.Primary.Bytes))
		if markdown == "" {
			continue
		}

		parts = append(parts, fmt.Sprintf("## File: %s\n\n%s", name, markdown))
	}

	return inkbite.DetailedConversion{
		Result:   inkbite.Result{Markdown: strings.Join(parts, "\n\n")},
		Warnings: warnings,
	}, nil
}

func readZIPEntry(ctx context.Context, file *zip.File, budget *internalingestion.RequestBudget) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, zipIntegrityFailure()
	}
	limit := budget.Limits().MaxContainerEntryBytes
	if remaining := budget.RemainingExpandedBytes(); remaining < limit {
		limit = remaining
	}
	owned, readErr := internalingestion.ReadBounded(ctx, reader, limit)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, zipIntegrityFailure()
	}
	if err := budget.AdmitContainerEntry(int64(file.UncompressedSize64), int64(file.CompressedSize64), owned.ByteLength); err != nil {
		return nil, err
	}
	return owned.Bytes, nil
}

func preflightZIPEntry(file *zip.File, budget *internalingestion.RequestBudget) error {
	if file.UncompressedSize64 > math.MaxInt64 || file.CompressedSize64 > math.MaxInt64 {
		return zipIntegrityFailure()
	}
	limits := budget.Limits()
	snapshot := budget.Snapshot()
	if snapshot.ContainerEntries >= limits.MaxContainerEntries ||
		int64(file.UncompressedSize64) > limits.MaxContainerEntryBytes ||
		int64(file.UncompressedSize64) > budget.RemainingExpandedBytes() {
		return fmt.Errorf("%w", internalingestion.ErrLimitExceeded)
	}
	return nil
}

func validZIPEntryType(file *zip.File, directory bool) bool {
	mode := file.Mode()
	if directory {
		return mode.IsDir()
	}
	return mode.IsRegular()
}

func zipRequestBudget(ctx context.Context, policy inkbite.IngestionPolicy) (*internalingestion.RequestBudget, bool, error) {
	if budget, ok := internalingestion.RequestBudgetFromContext(ctx); ok {
		return budget, false, nil
	}
	budget, err := internalingestion.NewRequestBudget(zipLimits(policy))
	return budget, true, err
}

func zipLimits(policy inkbite.IngestionPolicy) internalingestion.Limits {
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

func terminalZIPMemberError(err error) bool {
	return errors.Is(err, inkbite.ErrMalformedInput) ||
		errors.Is(err, inkbite.ErrLimitExceeded) ||
		errors.Is(err, inkbite.ErrPolicyViolation) ||
		errors.Is(err, inkbite.ErrIntegrityFailure) ||
		errors.Is(err, inkbite.ErrCancellation) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func zipMalformedInput() error {
	return fmt.Errorf("%w", inkbite.ErrMalformedInput)
}

func zipIntegrityFailure() error {
	return fmt.Errorf("%w", inkbite.ErrIntegrityFailure)
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
