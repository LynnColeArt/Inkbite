package pdfconv

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	"github.com/dslipak/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
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
	Extract(context.Context, []byte) (string, error)
}

// Converter extracts text and best-effort tables from PDFs.
type Converter struct {
	extractors []extractor
}

var _ inkbite.DetailedConverter = (*Converter)(nil)

// New returns a PDF converter.
func New() *Converter {
	return &Converter{
		extractors: []extractor{
			pureGoExtractor{},
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

	markdown := layoutToMarkdown(text)
	if opts.KeepDataURIs {
		imageMarkdown, err := extractPDFImagesMarkdown(ctx, data)
		if err != nil {
			return inkbite.Result{}, fmt.Errorf("pdf: image extraction: %w", err)
		}
		if imageMarkdown != "" {
			markdown = strings.TrimSpace(markdown)
			if markdown != "" {
				markdown += "\n\n"
			}
			markdown += imageMarkdown
		}
	}

	return inkbite.Result{
		Markdown: markdown,
	}, nil
}

// ConvertDetailed returns the legacy text projection together with ordered,
// independently retainable embedded-image bytes. Detailed Markdown refers only
// to envelope-local artifact IDs and never embeds data URIs.
func (c *Converter) ConvertDetailed(
	ctx context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	opts inkbite.ConvertOptions,
	policy inkbite.IngestionPolicy,
) (inkbite.DetailedConversion, error) {
	if err := checkContext(ctx); err != nil {
		return inkbite.DetailedConversion{}, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.DetailedConversion{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return inkbite.DetailedConversion{}, err
	}
	if err := checkContext(ctx); err != nil {
		return inkbite.DetailedConversion{}, err
	}

	extractor, err := c.chooseExtractor(opts.PDFBackend)
	if err != nil {
		return inkbite.DetailedConversion{}, fmt.Errorf("pdf: %w", err)
	}
	text, err := extractor.Extract(ctx, data)
	if err != nil {
		return inkbite.DetailedConversion{}, err
	}
	markdown := layoutToMarkdown(text)

	images, imageErr := extractPDFImages(ctx, data)
	if imageErr != nil {
		if terminalDetailedImageError(imageErr) {
			return inkbite.DetailedConversion{}, imageErr
		}
		if int64(len(markdown)) > policy.MaxPrimaryBytes {
			return inkbite.DetailedConversion{}, inkbite.ErrLimitExceeded
		}
		return inkbite.DetailedConversion{
			Result:    inkbite.Result{Markdown: markdown},
			Artifacts: make([]inkbite.DetailedArtifact, 0),
			Warnings: []inkbite.WarningRecord{{
				Category:  "artifact_extraction_failed",
				Converter: "pdf",
				Detail:    "image extraction failed",
			}},
			Backend: extractor.Name(),
			Facts:   make([]inkbite.MetadataFact, 0),
		}, nil
	}

	artifacts, err := detailedImageArtifacts(ctx, images, policy)
	if err != nil {
		return inkbite.DetailedConversion{}, err
	}
	markdown = appendDetailedImageReferences(markdown, artifacts)
	if int64(len(markdown)) > policy.MaxPrimaryBytes {
		return inkbite.DetailedConversion{}, inkbite.ErrLimitExceeded
	}
	return inkbite.DetailedConversion{
		Result:    inkbite.Result{Markdown: markdown},
		Artifacts: artifacts,
		Warnings:  make([]inkbite.WarningRecord, 0),
		Backend:   extractor.Name(),
		Facts:     make([]inkbite.MetadataFact, 0),
	}, nil
}

func terminalDetailedImageError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, inkbite.ErrCancellation) ||
		errors.Is(err, inkbite.ErrLimitExceeded) ||
		errors.Is(err, inkbite.ErrPolicyViolation) ||
		errors.Is(err, inkbite.ErrIntegrityFailure)
}

func detailedImageArtifacts(
	ctx context.Context,
	images []extractedImage,
	policy inkbite.IngestionPolicy,
) ([]inkbite.DetailedArtifact, error) {
	if len(images) > policy.MaxArtifacts {
		return nil, inkbite.ErrLimitExceeded
	}
	artifacts := make([]inkbite.DetailedArtifact, 0, len(images))
	var total int64
	for _, image := range images {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		byteLength := int64(len(image.Data))
		if byteLength > policy.MaxArtifactBytes || total > policy.MaxTotalArtifactBytes-byteLength {
			return nil, inkbite.ErrLimitExceeded
		}
		total += byteLength
		extension := extensionForMediaType(image.MediaType)
		if extension == "" || image.Page <= 0 || image.Object <= 0 {
			return nil, inkbite.ErrIntegrityFailure
		}
		if image.Width < 0 || image.Height < 0 || image.Bpc < 0 {
			return nil, inkbite.ErrIntegrityFailure
		}
		artifacts = append(artifacts, inkbite.DetailedArtifact{
			Role:       inkbite.ArtifactRoleEmbeddedImage,
			Bytes:      cloneImageBytes(image.Data),
			MediaType:  image.MediaType,
			SafeName:   fmt.Sprintf("page-%06d-object-%06d.%s", image.Page, image.Object, extension),
			Occurrence: fmt.Sprintf("page-%06d/object-%06d", image.Page, image.Object),
			Attributes: imageArtifactFacts(image),
		})
	}
	return artifacts, nil
}

func cloneImageBytes(data []byte) []byte {
	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned[:len(cloned):len(cloned)]
}

func extensionForMediaType(mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/tiff":
		return "tiff"
	case "image/webp":
		return "webp"
	default:
		return ""
	}
}

func imageArtifactFacts(image extractedImage) []inkbite.MetadataFact {
	values := []struct {
		kind  string
		value string
	}{
		{kind: "bits_per_component", value: strconv.Itoa(image.Bpc)},
		{kind: "height", value: strconv.Itoa(image.Height)},
		{kind: "image_mask", value: strconv.FormatBool(image.ImageMask)},
		{kind: "object", value: strconv.Itoa(image.Object)},
		{kind: "page", value: strconv.Itoa(image.Page)},
		{kind: "width", value: strconv.Itoa(image.Width)},
	}
	facts := make([]inkbite.MetadataFact, 0, len(values))
	for _, value := range values {
		facts = append(facts, inkbite.MetadataFact{
			Kind:   value.kind,
			Value:  value.value,
			Origin: inkbite.MetadataOriginConverter,
		})
	}
	return facts
}

func appendDetailedImageReferences(markdown string, artifacts []inkbite.DetailedArtifact) string {
	if len(artifacts) == 0 {
		return markdown
	}
	var out strings.Builder
	out.WriteString(strings.TrimSpace(markdown))
	if out.Len() > 0 {
		out.WriteString("\n\n")
	}
	out.WriteString("## PDF Images\n\n")
	for index, artifact := range artifacts {
		if index > 0 {
			out.WriteString("\n\n")
		}
		page := artifactFact(artifact.Attributes, "page")
		object := artifactFact(artifact.Attributes, "object")
		width := artifactFact(artifact.Attributes, "width")
		height := artifactFact(artifact.Attributes, "height")
		bpc := artifactFact(artifact.Attributes, "bits_per_component")
		fmt.Fprintf(
			&out,
			"![PDF image page %s object %s %sx%s %s bpc](inkbite-artifact:artifact-%06d)\n\n",
			page,
			object,
			width,
			height,
			bpc,
			index+1,
		)
		out.WriteString("| Page | Object | Type | Dimensions | Bits/Component | Bytes |\n")
		out.WriteString("| --- | --- | --- | --- | --- | --- |\n")
		fmt.Fprintf(&out, "| %s | %s | %s | %sx%s | %s | %d |", page, object, artifact.MediaType, width, height, bpc, len(artifact.Bytes))
	}
	return out.String()
}

func artifactFact(facts []inkbite.MetadataFact, kind string) string {
	for _, fact := range facts {
		if fact.Kind == kind {
			return fact.Value
		}
	}
	return "unknown"
}

func (c *Converter) chooseExtractor(requested string) (extractor, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" || requested == "auto" {
		for _, candidate := range c.extractors {
			if candidate.Name() == "purego" {
				return candidate, nil
			}
		}
		return nil, fmt.Errorf("no PDF extractor backend available")
	}

	for _, candidate := range c.extractors {
		if candidate.Name() == requested {
			return candidate, nil
		}
	}

	return nil, fmt.Errorf("unknown PDF extractor %q", requested)
}

type pureGoExtractor struct{}

func (pureGoExtractor) Name() string {
	return "purego"
}

func (pureGoExtractor) Extract(ctx context.Context, data []byte) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("purego: %w", err)
	}

	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("purego: %w", err)
	}

	var out bytes.Buffer
	if _, err := out.ReadFrom(textReader); err != nil {
		return "", fmt.Errorf("purego: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	return out.String(), nil
}

type extractedImage struct {
	Page       int
	Object     int
	Name       string
	MediaType  string
	FileType   string
	Width      int
	Height     int
	Bpc        int
	ColorSpace string
	Filter     string
	ImageMask  bool
	Data       []byte
}

func extractPDFImagesMarkdown(ctx context.Context, data []byte) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	images, err := extractPDFImages(ctx, data)
	if err != nil {
		return "", err
	}
	if len(images) == 0 {
		return "", nil
	}

	sort.SliceStable(images, func(i, j int) bool {
		if images[i].Page != images[j].Page {
			return images[i].Page < images[j].Page
		}
		return images[i].Name < images[j].Name
	})

	var out strings.Builder
	out.WriteString("## PDF Images\n\n")
	for idx, image := range images {
		if idx > 0 {
			out.WriteString("\n\n")
		}
		alt := fmt.Sprintf(
			"PDF image page %d %s %s %s %s",
			image.Page,
			image.Name,
			imageDimensions(image),
			image.ColorSpace,
			imageBitsPerComponent(image),
		)
		fmt.Fprintf(&out, "![%s](data:%s;base64,%s)\n\n", alt, image.MediaType, base64.StdEncoding.EncodeToString(image.Data))
		out.WriteString("| Page | Name | Type | Dimensions | Color Space | Bits/Component | Filters | Bytes |\n")
		out.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\n")
		fmt.Fprintf(
			&out,
			"| %d | %s | %s | %dx%d | %s | %d | %s | %d |",
			image.Page,
			escapeCell(image.Name),
			escapeCell(image.FileType),
			image.Width,
			image.Height,
			escapeCell(image.ColorSpace),
			image.Bpc,
			escapeCell(image.Filter),
			len(image.Data),
		)
	}
	return out.String(), nil
}

func extractPDFImages(ctx context.Context, data []byte) ([]extractedImage, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	conf.Cmd = model.EXTRACTIMAGES

	pdfCtx, err := api.ReadValidateAndOptimize(bytes.NewReader(data), conf)
	if err != nil {
		return nil, err
	}

	patchImageMaskColorSpaces(pdfCtx)

	pages, err := api.PagesForPageSelection(pdfCtx.PageCount, nil, true, true)
	if err != nil {
		return nil, err
	}
	var pageNrs []int
	for page, selected := range pages {
		if selected {
			pageNrs = append(pageNrs, page)
		}
	}
	sort.Ints(pageNrs)

	var images []extractedImage
	for _, page := range pageNrs {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		pageImages, err := pdfcpu.ExtractPageImages(pdfCtx, page, false)
		if err != nil {
			return nil, err
		}
		var objNrs []int
		for objNr := range pageImages {
			objNrs = append(objNrs, objNr)
		}
		sort.Ints(objNrs)
		for _, objNr := range objNrs {
			image := pageImages[objNr]
			extracted, err := extractImagePayload(ctx, pdfCtx, image)
			if err != nil {
				return nil, err
			}
			if extracted != nil {
				extracted.Object = objNr
				images = append(images, *extracted)
			}
		}
	}
	if len(images) == 0 && bytes.Contains(data, []byte("/Subtype /Image")) {
		return nil, fmt.Errorf("embedded image extraction produced no retained payload")
	}
	return images, nil
}

func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func patchImageMaskColorSpaces(ctx *model.Context) {
	for _, imageObject := range ctx.Optimize.ImageObjects {
		if imageObject == nil || imageObject.ImageDict == nil {
			continue
		}
		imageMask := imageObject.ImageDict.BooleanEntry("ImageMask")
		if imageMask == nil || !*imageMask {
			continue
		}
		if _, ok := imageObject.ImageDict.Find("ColorSpace"); !ok {
			imageObject.ImageDict.InsertName("ColorSpace", model.DeviceGrayCS)
		}
		if _, ok := imageObject.ImageDict.Find("BitsPerComponent"); !ok {
			imageObject.ImageDict.InsertInt("BitsPerComponent", 1)
		}
	}
}

func extractImagePayload(ctx context.Context, pdfCtx *model.Context, image model.Image) (*extractedImage, error) {
	if image.Reader == nil {
		return nil, nil
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	mediaType := mediaTypeForImage(image.FileType)
	if mediaType == "" {
		return nil, fmt.Errorf("unsupported image file type %q for page %d image %q", image.FileType, image.PageNr, image.Name)
	}
	payload, err := io.ReadAll(image.Reader)
	if err != nil {
		return nil, fmt.Errorf("read page %d image %q: %w", image.PageNr, image.Name, err)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("empty image payload for page %d image %q", image.PageNr, image.Name)
	}

	width, height, bpc, colorSpace, filterName, imageMask := imageMetadata(pdfCtx, image)
	if width == 0 || height == 0 {
		config, _, err := imageConfig(payload)
		if err == nil {
			width = config.Width
			height = config.Height
		}
	}

	return &extractedImage{
		Page:       image.PageNr,
		Name:       image.Name,
		MediaType:  mediaType,
		FileType:   image.FileType,
		Width:      width,
		Height:     height,
		Bpc:        bpc,
		ColorSpace: colorSpace,
		Filter:     filterName,
		ImageMask:  imageMask,
		Data:       payload,
	}, nil
}

func imageMetadata(ctx *model.Context, image model.Image) (width, height, bpc int, colorSpace, filterName string, imageMask bool) {
	imageObject := ctx.Optimize.ImageObjects[image.ObjNr]
	if imageObject == nil || imageObject.ImageDict == nil {
		return image.Width, image.Height, image.Bpc, image.Cs, image.Filter, image.IsImgMask
	}

	sd := imageObject.ImageDict
	if value := sd.IntEntry("Width"); value != nil {
		width = *value
	}
	if value := sd.IntEntry("Height"); value != nil {
		height = *value
	}
	if value := sd.IntEntry("BitsPerComponent"); value != nil {
		bpc = *value
	}
	if value := sd.NameEntry("ColorSpace"); value != nil {
		colorSpace = string(*value)
	}
	if value := sd.BooleanEntry("ImageMask"); value != nil && *value {
		imageMask = true
		if bpc == 0 {
			bpc = 1
		}
		if colorSpace == "" {
			colorSpace = model.DeviceGrayCS
		}
		colorSpace += " image mask"
	}
	if sd.FilterPipeline != nil {
		var filters []string
		for _, filter := range sd.FilterPipeline {
			filters = append(filters, filter.Name)
		}
		filterName = strings.Join(filters, ",")
	}

	return width, height, bpc, colorSpace, filterName, imageMask
}

func imageConfig(data []byte) (image.Config, string, error) {
	return image.DecodeConfig(bytes.NewReader(data))
}

func imageDimensions(image extractedImage) string {
	if image.Width <= 0 || image.Height <= 0 {
		return "unknown dimensions"
	}
	return fmt.Sprintf("%dx%d", image.Width, image.Height)
}

func imageBitsPerComponent(image extractedImage) string {
	if image.Bpc <= 0 {
		return "unknown bpc"
	}
	return fmt.Sprintf("%d bpc", image.Bpc)
}

func mediaTypeForImage(fileType string) string {
	switch strings.ToLower(strings.TrimPrefix(fileType, ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "tif", "tiff":
		return "image/tiff"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
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
