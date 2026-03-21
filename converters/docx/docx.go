package docxconv

import (
	"context"
	"io"
	"path"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/internal/ooxml"
)

const priority = 12

var (
	docxExtensions = map[string]struct{}{
		".docx": {},
	}
	docxMIMETypes = map[string]struct{}{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	}
)

// Converter extracts reduced-scope surface context from DOCX files.
type Converter struct{}

// New returns a DOCX converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "docx"
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
	if _, ok := docxExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := docxMIMETypes[info.MIMEType]; ok {
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

	documentXML, ok := pkg.ReadFile("word/document.xml")
	if !ok {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{Info: info}
	}

	document, err := ooxml.ParseNode(documentXML)
	if err != nil {
		return inkbite.Result{}, err
	}

	relationships := map[string]string{}
	if relXML, ok := pkg.ReadFile("word/_rels/document.xml.rels"); ok {
		mapped, err := ooxml.RelationshipMap(relXML, path.Dir("word/document.xml"))
		if err != nil {
			return inkbite.Result{}, err
		}
		relationships = mapped
	}

	body := findFirst(document, "body")
	if body == nil {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{Info: info}
	}

	var parts []string
	var title string
	for _, child := range body.Nodes {
		switch strings.ToLower(child.XMLName.Local) {
		case "p":
			markdown, candidateTitle := renderParagraph(&child, relationships)
			if strings.TrimSpace(markdown) == "" {
				continue
			}
			if title == "" && candidateTitle != "" {
				title = candidateTitle
			}
			parts = append(parts, markdown)
		case "tbl":
			table := renderTable(&child, relationships)
			if strings.TrimSpace(table) != "" {
				parts = append(parts, table)
			}
		}
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
		Title:    title,
	}, nil
}

func renderParagraph(node *ooxml.Node, relationships map[string]string) (markdown string, title string) {
	text := strings.TrimSpace(renderInline(node, relationships))
	if text == "" {
		return "", ""
	}

	style := paragraphStyle(node)
	switch {
	case isTitleStyle(style):
		return "# " + text, text
	case headingLevel(style) > 0:
		level := headingLevel(style)
		return strings.Repeat("#", level) + " " + text, text
	default:
		return text, ""
	}
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
			current = append(current, strings.TrimSpace(renderCell(cell, relationships)))
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

func renderCell(node *ooxml.Node, relationships map[string]string) string {
	var parts []string
	for _, child := range node.Nodes {
		switch strings.ToLower(child.XMLName.Local) {
		case "p":
			text := strings.TrimSpace(renderInline(&child, relationships))
			if text != "" {
				parts = append(parts, text)
			}
		case "tbl":
			text := strings.TrimSpace(renderTable(&child, relationships))
			if text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "<br>")
}

func renderInline(node *ooxml.Node, relationships map[string]string) string {
	if node == nil {
		return ""
	}

	var builder strings.Builder
	var previous *ooxml.Node
	for idx := range node.Nodes {
		child := &node.Nodes[idx]
		var fragment string
		switch strings.ToLower(child.XMLName.Local) {
		case "r", "smarttag", "sdtcontent":
			fragment = renderInline(child, relationships)
		case "hyperlink":
			label := strings.TrimSpace(renderInline(child, relationships))
			if label == "" {
				previous = child
				continue
			}
			if target := relationships[child.Attr("id")]; target != "" {
				fragment = "[" + label + "](" + target + ")"
			} else {
				fragment = label
			}
		case "t":
			fragment = child.Text()
		case "tab":
			fragment = " "
		case "br", "cr":
			fragment = "\n"
		case "noBreakHyphen":
			fragment = "-"
		default:
			fragment = renderInline(child, relationships)
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

func paragraphStyle(node *ooxml.Node) string {
	pPr := node.Child("pPr")
	if pPr == nil {
		return ""
	}
	style := pPr.Child("pStyle")
	if style == nil {
		return ""
	}
	return strings.TrimSpace(style.Attr("val"))
}

func headingLevel(style string) int {
	style = strings.ToLower(strings.TrimSpace(style))
	switch style {
	case "heading1":
		return 1
	case "heading2":
		return 2
	case "heading3":
		return 3
	case "heading4":
		return 4
	case "heading5":
		return 5
	case "heading6":
		return 6
	default:
		return 0
	}
}

func isTitleStyle(style string) bool {
	style = strings.ToLower(strings.TrimSpace(style))
	return style == "title" || style == "subtitle"
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
