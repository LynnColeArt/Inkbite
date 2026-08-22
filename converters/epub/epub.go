package epubconv

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	htmlconv "github.com/LynnColeArt/Inkbite/converters/html"
	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
)

const priority = 25

var (
	epubExtensions = map[string]struct{}{
		".epub": {},
	}
	epubMIMETypes = map[string]struct{}{
		"application/epub":       {},
		"application/epub+zip":   {},
		"application/x-epub+zip": {},
	}
)

type container struct {
	Rootfiles []rootfile `xml:"rootfiles>rootfile"`
}

type rootfile struct {
	FullPath string `xml:"full-path,attr"`
}

type packageDocument struct {
	Metadata packageMetadata `xml:"metadata"`
	Manifest []manifestItem  `xml:"manifest>item"`
	Spine    []spineItem     `xml:"spine>itemref"`
}

type packageMetadata struct {
	Title       string   `xml:"title"`
	Creators    []string `xml:"creator"`
	Language    string   `xml:"language"`
	Publisher   string   `xml:"publisher"`
	Date        string   `xml:"date"`
	Description string   `xml:"description"`
	Identifier  string   `xml:"identifier"`
}

type manifestItem struct {
	ID   string `xml:"id,attr"`
	Href string `xml:"href,attr"`
}

type spineItem struct {
	IDRef string `xml:"idref,attr"`
}

// Converter extracts metadata and spine content from EPUB files.
type Converter struct {
	html *htmlconv.Converter
}

// New returns an EPUB converter.
func New() *Converter {
	return &Converter{
		html: htmlconv.New(),
	}
}

func (c *Converter) Name() string {
	return "epub"
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
	if _, ok := epubExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := epubMIMETypes[info.MIMEType]; ok {
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

// ConvertDetailed applies the request's shared container ledger and reports
// permitted EPUB degradation without changing the legacy Markdown projection.
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
	_ inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (conversion inkbite.DetailedConversion, err error) {
	if policy == (inkbite.IngestionPolicy{}) {
		policy = inkbite.DefaultIngestionPolicy()
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return conversion, integrityFailure()
	}
	budget, ownsBudget, err := epubRequestBudget(ctx, policy)
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
		return conversion, malformedInput()
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

	files, err := readEPUBEntries(ctx, reader.File, budget)
	if err != nil {
		return conversion, err
	}

	containerFile, ok := files["META-INF/container.xml"]
	if !ok {
		return conversion, inkbite.UnsupportedFormatError{}
	}

	var containerDoc container
	if err := xml.Unmarshal(containerFile, &containerDoc); err != nil {
		return conversion, malformedInput()
	}
	if len(containerDoc.Rootfiles) == 0 || strings.TrimSpace(containerDoc.Rootfiles[0].FullPath) == "" {
		return conversion, inkbite.UnsupportedFormatError{}
	}

	opfPath, err := internalingestion.CanonicalArchivePath(containerDoc.Rootfiles[0].FullPath)
	if err != nil {
		return conversion, malformedInput()
	}
	opfFile, ok := files[opfPath]
	if !ok {
		return conversion, inkbite.UnsupportedFormatError{}
	}

	var packageDoc packageDocument
	if err := xml.Unmarshal(opfFile, &packageDoc); err != nil {
		return conversion, malformedInput()
	}

	manifest := make(map[string]string, len(packageDoc.Manifest))
	for _, item := range packageDoc.Manifest {
		if item.ID == "" || item.Href == "" {
			return conversion, malformedInput()
		}
		if _, exists := manifest[item.ID]; exists {
			return conversion, malformedInput()
		}
		href, err := internalingestion.CanonicalArchivePath(item.Href)
		if err != nil {
			return conversion, malformedInput()
		}
		manifest[item.ID] = href
	}

	basePath := path.Dir(opfPath)
	if basePath == "." {
		basePath = ""
	}

	var parts []string
	warnings := make([]inkbite.WarningRecord, 0)
	if metadata := formatMetadata(packageDoc.Metadata); metadata != "" {
		parts = append(parts, metadata)
	}

	for _, item := range packageDoc.Spine {
		if err := internalingestion.Checkpoint(ctx); err != nil {
			return conversion, err
		}
		href := manifest[item.IDRef]
		if href == "" {
			warnings = append(warnings, inkbite.WarningRecord{
				Category:  "missing_manifest_item",
				Converter: c.Name(),
				Location:  safeEPUBWarningLocation(item.IDRef),
			})
			continue
		}

		fullPath := href
		if basePath != "" {
			fullPath = path.Join(basePath, href)
		}
		fullPath, err = internalingestion.CanonicalArchivePath(fullPath)
		if err != nil {
			return conversion, malformedInput()
		}

		entry, ok := files[fullPath]
		if !ok {
			warnings = append(warnings, inkbite.WarningRecord{
				Category:  "missing_spine_content",
				Converter: c.Name(),
				Location:  fullPath,
			})
			continue
		}
		rendered, err := c.html.ConvertString(string(entry))
		if err != nil {
			return conversion, fmt.Errorf("%w", inkbite.ErrConverterFailure)
		}
		if strings.TrimSpace(rendered.Markdown) != "" {
			parts = append(parts, strings.TrimSpace(rendered.Markdown))
		}
	}

	return inkbite.DetailedConversion{
		Result: inkbite.Result{
			Markdown: strings.Join(parts, "\n\n"),
			Title:    strings.TrimSpace(packageDoc.Metadata.Title),
		},
		Warnings: warnings,
	}, nil
}

func readEPUBEntries(ctx context.Context, archiveFiles []*zip.File, budget *internalingestion.RequestBudget) (map[string][]byte, error) {
	files := make(map[string][]byte, len(archiveFiles))
	canonicalNames := make(map[string]struct{}, len(archiveFiles))
	for _, file := range archiveFiles {
		if err := internalingestion.Checkpoint(ctx); err != nil {
			return nil, err
		}
		isDirectory := file.FileInfo().IsDir()
		candidate := file.Name
		if isDirectory {
			candidate = strings.TrimSuffix(candidate, "/")
		}
		name, err := internalingestion.CanonicalArchivePath(candidate)
		if err != nil || !validEPUBEntryType(file, isDirectory) {
			return nil, malformedInput()
		}
		collisionKey := strings.ToLower(name)
		if _, exists := canonicalNames[collisionKey]; exists {
			return nil, malformedInput()
		}
		canonicalNames[collisionKey] = struct{}{}

		if err := preflightEPUBEntry(file, budget); err != nil {
			return nil, err
		}
		if isDirectory {
			if err := budget.AdmitContainerEntry(int64(file.UncompressedSize64), int64(file.CompressedSize64), 0); err != nil {
				return nil, err
			}
			continue
		}
		content, err := readEPUBEntry(ctx, file, budget)
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return files, nil
}

func readEPUBEntry(ctx context.Context, file *zip.File, budget *internalingestion.RequestBudget) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, integrityFailure()
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
		return nil, integrityFailure()
	}
	if err := budget.AdmitContainerEntry(int64(file.UncompressedSize64), int64(file.CompressedSize64), owned.ByteLength); err != nil {
		return nil, err
	}
	return owned.Bytes, nil
}

func preflightEPUBEntry(file *zip.File, budget *internalingestion.RequestBudget) error {
	if file.UncompressedSize64 > math.MaxInt64 || file.CompressedSize64 > math.MaxInt64 {
		return integrityFailure()
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

func validEPUBEntryType(file *zip.File, directory bool) bool {
	mode := file.Mode()
	if directory {
		return mode.IsDir()
	}
	return mode.IsRegular()
}

func epubRequestBudget(ctx context.Context, policy inkbite.IngestionPolicy) (*internalingestion.RequestBudget, bool, error) {
	if budget, ok := internalingestion.RequestBudgetFromContext(ctx); ok {
		return budget, false, nil
	}
	budget, err := internalingestion.NewRequestBudget(epubLimits(policy))
	return budget, true, err
}

func epubLimits(policy inkbite.IngestionPolicy) internalingestion.Limits {
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

func malformedInput() error {
	return fmt.Errorf("%w", inkbite.ErrMalformedInput)
}

func integrityFailure() error {
	return fmt.Errorf("%w", inkbite.ErrIntegrityFailure)
}

func safeEPUBWarningLocation(value string) string {
	value, err := internalingestion.CanonicalLogicalName(value)
	if err != nil {
		return ""
	}
	return value
}

func formatMetadata(meta packageMetadata) string {
	var lines []string
	if title := strings.TrimSpace(meta.Title); title != "" {
		lines = append(lines, "**Title:** "+title)
	}
	if len(meta.Creators) > 0 {
		var creators []string
		for _, creator := range meta.Creators {
			creator = strings.TrimSpace(creator)
			if creator != "" {
				creators = append(creators, creator)
			}
		}
		if len(creators) > 0 {
			lines = append(lines, "**Authors:** "+strings.Join(creators, ", "))
		}
	}
	if language := strings.TrimSpace(meta.Language); language != "" {
		lines = append(lines, "**Language:** "+language)
	}
	if publisher := strings.TrimSpace(meta.Publisher); publisher != "" {
		lines = append(lines, "**Publisher:** "+publisher)
	}
	if date := strings.TrimSpace(meta.Date); date != "" {
		lines = append(lines, "**Date:** "+date)
	}
	if description := strings.TrimSpace(meta.Description); description != "" {
		lines = append(lines, "**Description:** "+description)
	}
	if identifier := strings.TrimSpace(meta.Identifier); identifier != "" {
		lines = append(lines, "**Identifier:** "+identifier)
	}

	return strings.Join(lines, "\n")
}
