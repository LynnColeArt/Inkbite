package epubconv

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"path"
	"strings"

	"github.com/LynnColeArt/Inkbite"
	htmlconv "github.com/LynnColeArt/Inkbite/converters/html"
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

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return inkbite.Result{}, err
	}

	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}

	containerFile, ok := files["META-INF/container.xml"]
	if !ok {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{}
	}

	containerXML, err := readZipFile(containerFile)
	if err != nil {
		return inkbite.Result{}, err
	}

	var containerDoc container
	if err := xml.Unmarshal(containerXML, &containerDoc); err != nil {
		return inkbite.Result{}, err
	}
	if len(containerDoc.Rootfiles) == 0 || strings.TrimSpace(containerDoc.Rootfiles[0].FullPath) == "" {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{}
	}

	opfPath := containerDoc.Rootfiles[0].FullPath
	opfFile, ok := files[opfPath]
	if !ok {
		return inkbite.Result{}, inkbite.UnsupportedFormatError{}
	}

	opfXML, err := readZipFile(opfFile)
	if err != nil {
		return inkbite.Result{}, err
	}

	var packageDoc packageDocument
	if err := xml.Unmarshal(opfXML, &packageDoc); err != nil {
		return inkbite.Result{}, err
	}

	manifest := make(map[string]string, len(packageDoc.Manifest))
	for _, item := range packageDoc.Manifest {
		manifest[item.ID] = item.Href
	}

	basePath := path.Dir(opfPath)
	if basePath == "." {
		basePath = ""
	}

	var parts []string
	if metadata := formatMetadata(packageDoc.Metadata); metadata != "" {
		parts = append(parts, metadata)
	}

	for _, item := range packageDoc.Spine {
		href := manifest[item.IDRef]
		if href == "" {
			continue
		}

		fullPath := href
		if basePath != "" {
			fullPath = path.Clean(path.Join(basePath, href))
		}

		entry, ok := files[fullPath]
		if !ok {
			continue
		}

		content, err := readZipFile(entry)
		if err != nil {
			return inkbite.Result{}, err
		}

		rendered, err := c.html.ConvertString(string(content))
		if err != nil {
			return inkbite.Result{}, err
		}
		if strings.TrimSpace(rendered.Markdown) != "" {
			parts = append(parts, strings.TrimSpace(rendered.Markdown))
		}
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
		Title:    strings.TrimSpace(packageDoc.Metadata.Title),
	}, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
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
