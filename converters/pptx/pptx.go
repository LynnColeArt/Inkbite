package pptxconv

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/internal/ooxml"
)

const (
	priority = 13
	relNS    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

var (
	pptxExtensions = map[string]struct{}{
		".pptx": {},
	}
	pptxMIMETypes = map[string]struct{}{
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": {},
	}
)

// Converter extracts reduced-scope surface context from PPTX files.
type Converter struct{}

// New returns a PPTX converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "pptx"
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
	if _, ok := pptxExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := pptxMIMETypes[info.MIMEType]; ok {
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

	pkg, err := ooxml.Open(data)
	if err != nil {
		return inkbite.Result{}, err
	}

	presentationXML, ok := pkg.ReadFile("ppt/presentation.xml")
	if !ok {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{Info: info}
	}

	presentation, err := ooxml.ParseNode(presentationXML)
	if err != nil {
		return inkbite.Result{}, err
	}

	relationships := map[string]string{}
	if relXML, ok := pkg.ReadFile("ppt/_rels/presentation.xml.rels"); ok {
		mapped, err := ooxml.RelationshipMap(relXML, path.Dir("ppt/presentation.xml"))
		if err != nil {
			return inkbite.Result{}, err
		}
		relationships = mapped
	}

	slidePaths := orderedSlidePaths(presentation, relationships)
	if len(slidePaths) == 0 {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{Info: info}
	}

	var (
		parts []string
		title string
	)
	for idx, slidePath := range slidePaths {
		markdown, slideTitle, err := renderSlide(pkg, slidePath, idx+1)
		if err != nil {
			return inkbite.Result{}, err
		}
		if strings.TrimSpace(markdown) == "" {
			continue
		}
		if title == "" && slideTitle != "" {
			title = slideTitle
		}
		parts = append(parts, markdown)
	}

	if len(parts) == 0 {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{Info: info}
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
		Title:    title,
	}, nil
}

func orderedSlidePaths(presentation *ooxml.Node, relationships map[string]string) []string {
	sldIDList := findFirst(presentation, "sldIdLst")
	if sldIDList == nil {
		return nil
	}

	var slides []string
	for idx := range sldIDList.Nodes {
		node := &sldIDList.Nodes[idx]
		if !strings.EqualFold(node.XMLName.Local, "sldId") {
			continue
		}
		relID := node.AttrNS(relNS, "id")
		if relID == "" {
			continue
		}
		if target := relationships[relID]; target != "" {
			slides = append(slides, target)
		}
	}
	return slides
}

func renderSlide(pkg *ooxml.Package, slidePath string, number int) (markdown string, title string, err error) {
	slideXML, ok := pkg.ReadFile(slidePath)
	if !ok {
		return "", "", fmt.Errorf("pptx: missing slide %q", slidePath)
	}

	slide, err := ooxml.ParseNode(slideXML)
	if err != nil {
		return "", "", err
	}

	relationships, err := relationshipsForPart(pkg, slidePath)
	if err != nil {
		return "", "", err
	}

	spTree := findFirst(slide, "spTree")
	bodyTitle, blocks := collectBlocks(spTree, relationships, false)

	heading := fmt.Sprintf("## Slide %d", number)
	if bodyTitle != "" {
		title = bodyTitle
		heading += ": " + bodyTitle
	}

	parts := []string{heading}
	for _, block := range blocks {
		if strings.TrimSpace(block) != "" {
			parts = append(parts, block)
		}
	}

	if notes := renderNotes(pkg, relationships); strings.TrimSpace(notes) != "" {
		parts = append(parts, "### Notes", notes)
	}

	return strings.Join(parts, "\n\n"), title, nil
}

func relationshipsForPart(pkg *ooxml.Package, partPath string) (map[string]string, error) {
	relPath := path.Join(path.Dir(partPath), "_rels", path.Base(partPath)+".rels")
	relXML, ok := pkg.ReadFile(relPath)
	if !ok {
		return map[string]string{}, nil
	}
	return ooxml.RelationshipMap(relXML, path.Dir(partPath))
}

func renderNotes(pkg *ooxml.Package, relationships map[string]string) string {
	notesPath := findNotesPath(relationships)
	if notesPath == "" {
		return ""
	}

	notesXML, ok := pkg.ReadFile(notesPath)
	if !ok {
		return ""
	}

	notes, err := ooxml.ParseNode(notesXML)
	if err != nil {
		return ""
	}

	spTree := findFirst(notes, "spTree")
	_, blocks := collectBlocks(spTree, map[string]string{}, true)
	return strings.Join(blocks, "\n\n")
}

func findNotesPath(relationships map[string]string) string {
	for _, target := range relationships {
		if strings.Contains(target, "/notesSlides/") && strings.HasSuffix(strings.ToLower(target), ".xml") {
			return target
		}
	}
	return ""
}

func collectBlocks(node *ooxml.Node, relationships map[string]string, notes bool) (title string, blocks []string) {
	if node == nil {
		return "", nil
	}

	for idx := range node.Nodes {
		child := &node.Nodes[idx]
		switch strings.ToLower(child.XMLName.Local) {
		case "sp":
			placeholderType := shapePlaceholderType(child)
			if shouldSkipPlaceholder(placeholderType, notes) {
				continue
			}

			text := strings.TrimSpace(renderShapeText(child, relationships))
			if text == "" {
				continue
			}

			if !notes && title == "" && isTitlePlaceholder(placeholderType) {
				title = firstLine(text)
				continue
			}

			blocks = append(blocks, text)
		case "graphicframe":
			table := strings.TrimSpace(renderGraphicFrame(child, relationships))
			if table != "" {
				blocks = append(blocks, table)
			}
		case "grpsp", "sptree":
			nestedTitle, nestedBlocks := collectBlocks(child, relationships, notes)
			if title == "" && nestedTitle != "" {
				title = nestedTitle
			}
			blocks = append(blocks, nestedBlocks...)
		}
	}

	return title, blocks
}

func renderShapeText(node *ooxml.Node, relationships map[string]string) string {
	if node == nil {
		return ""
	}

	txBody := findFirst(node, "txBody")
	if txBody == nil {
		return ""
	}

	var paragraphs []string
	for idx := range txBody.Nodes {
		child := &txBody.Nodes[idx]
		if !strings.EqualFold(child.XMLName.Local, "p") {
			continue
		}
		text := strings.TrimSpace(renderParagraph(child, relationships))
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}

	return strings.Join(paragraphs, "\n")
}

func renderParagraph(node *ooxml.Node, relationships map[string]string) string {
	if node == nil {
		return ""
	}

	var builder strings.Builder
	var previous *ooxml.Node
	for idx := range node.Nodes {
		child := &node.Nodes[idx]
		var fragment string
		switch strings.ToLower(child.XMLName.Local) {
		case "r", "fld":
			fragment = renderRun(child, relationships)
		case "br":
			fragment = "\n"
		case "tab":
			fragment = " "
		default:
			fragment = renderParagraph(findFirst(child, "p"), relationships)
		}

		if fragment != "" {
			if shouldInsertSpace(previous, &builder, fragment) {
				builder.WriteByte(' ')
			}
			builder.WriteString(fragment)
		}
		previous = child
	}

	return normalizeInline(builder.String())
}

func renderRun(node *ooxml.Node, relationships map[string]string) string {
	if node == nil {
		return ""
	}

	label := strings.TrimSpace(renderRunText(node))
	if label == "" {
		return ""
	}

	if target := runHyperlinkTarget(node, relationships); target != "" {
		return "[" + label + "](" + target + ")"
	}

	return label
}

func renderRunText(node *ooxml.Node) string {
	if node == nil {
		return ""
	}

	var builder strings.Builder
	for idx := range node.Nodes {
		child := &node.Nodes[idx]
		switch strings.ToLower(child.XMLName.Local) {
		case "t":
			builder.WriteString(child.Text())
		case "tab":
			builder.WriteByte(' ')
		case "br":
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

func runHyperlinkTarget(node *ooxml.Node, relationships map[string]string) string {
	if node == nil {
		return ""
	}

	hlink := findFirst(node, "hlinkClick")
	if hlink == nil {
		return ""
	}

	relID := hlink.AttrNS(relNS, "id")
	if relID == "" {
		return ""
	}
	return relationships[relID]
}

func renderGraphicFrame(node *ooxml.Node, relationships map[string]string) string {
	table := findFirst(node, "tbl")
	if table == nil {
		return ""
	}
	return renderTable(table, relationships)
}

func renderTable(node *ooxml.Node, relationships map[string]string) string {
	rows := node.Children("tr")
	if len(rows) == 0 {
		return ""
	}

	width := 0
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells := row.Children("tc")
		current := make([]string, 0, len(cells))
		for _, cell := range cells {
			current = append(current, strings.TrimSpace(renderTableCell(cell, relationships)))
		}
		if len(current) > width {
			width = len(current)
		}
		tableRows = append(tableRows, current)
	}
	if width == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, formatRow(tableRows[0], width))

	separator := make([]string, width)
	for idx := range separator {
		separator[idx] = "---"
	}
	lines = append(lines, formatRow(separator, width))

	for _, row := range tableRows[1:] {
		lines = append(lines, formatRow(row, width))
	}

	return strings.Join(lines, "\n")
}

func renderTableCell(node *ooxml.Node, relationships map[string]string) string {
	if node == nil {
		return ""
	}

	txBody := findFirst(node, "txBody")
	if txBody == nil {
		return ""
	}

	var parts []string
	for idx := range txBody.Nodes {
		child := &txBody.Nodes[idx]
		if !strings.EqualFold(child.XMLName.Local, "p") {
			continue
		}
		text := strings.TrimSpace(renderParagraph(child, relationships))
		if text != "" {
			parts = append(parts, text)
		}
	}

	return strings.Join(parts, "<br>")
}

func shapePlaceholderType(node *ooxml.Node) string {
	placeholder := findFirst(node, "ph")
	if placeholder == nil {
		return ""
	}
	return strings.TrimSpace(placeholder.Attr("type"))
}

func shouldSkipPlaceholder(placeholderType string, notes bool) bool {
	placeholderType = strings.ToLower(strings.TrimSpace(placeholderType))
	switch placeholderType {
	case "dt", "ftr", "hdr", "sldnum", "sldimg":
		return true
	}
	if notes && isTitlePlaceholder(placeholderType) {
		return true
	}
	return false
}

func isTitlePlaceholder(placeholderType string) bool {
	placeholderType = strings.ToLower(strings.TrimSpace(placeholderType))
	return placeholderType == "title" || placeholderType == "ctrtitle"
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}

func normalizeInline(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	lines := strings.Split(value, "\n")
	for idx := range lines {
		lines[idx] = strings.Join(strings.Fields(lines[idx]), " ")
	}
	value = strings.Join(lines, "\n")
	value = strings.ReplaceAll(value, "\n ", "\n")
	value = strings.ReplaceAll(value, " \n", "\n")
	return strings.TrimSpace(value)
}

func shouldInsertSpace(previous *ooxml.Node, builder *strings.Builder, fragment string) bool {
	if previous == nil || builder == nil || builder.Len() == 0 || fragment == "" {
		return false
	}
	if strings.HasPrefix(fragment, " ") || strings.HasPrefix(fragment, "\n") {
		return false
	}

	current := builder.String()
	last := current[len(current)-1]
	if last == ' ' || last == '\n' || last == '\t' {
		return false
	}

	return hasPreservedTrailingSpace(previous)
}

func hasPreservedTrailingSpace(node *ooxml.Node) bool {
	if node == nil {
		return false
	}
	if strings.EqualFold(node.XMLName.Local, "t") && strings.EqualFold(node.Attr("space"), "preserve") {
		return true
	}
	for idx := len(node.Nodes) - 1; idx >= 0; idx-- {
		if hasPreservedTrailingSpace(&node.Nodes[idx]) {
			return true
		}
	}
	return false
}

func findFirst(node *ooxml.Node, local string) *ooxml.Node {
	if node == nil {
		return nil
	}
	if strings.EqualFold(node.XMLName.Local, local) {
		return node
	}
	for idx := range node.Nodes {
		if found := findFirst(&node.Nodes[idx], local); found != nil {
			return found
		}
	}
	return nil
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
	value = strings.ReplaceAll(value, "\n", "<br>")
	value = strings.ReplaceAll(value, "|", `\|`)
	return strings.TrimSpace(value)
}
