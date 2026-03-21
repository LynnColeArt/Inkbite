package htmlconv

import (
	"bytes"
	"context"
	"io"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 40

var (
	htmlExtensions = map[string]struct{}{
		".htm":  {},
		".html": {},
	}
	htmlMIMETypes = map[string]struct{}{
		"application/xhtml+xml": {},
		"text/html":             {},
	}
)

// Converter transforms HTML documents into Markdown.
type Converter struct{}

// New returns an HTML converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "html"
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
	if _, ok := htmlExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := htmlMIMETypes[info.MIMEType]; ok {
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

	return c.ConvertString(inkbite.DecodeText(data, info.Charset))
}

// ConvertString converts an HTML string into Markdown.
func (c *Converter) ConvertString(input string) (inkbite.Result, error) {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return inkbite.Result{}, err
	}

	removeNodes(doc, "script", "style")

	title := strings.TrimSpace(findFirstText(doc, "title"))

	target := findFirstNode(doc, "body")
	if target == nil {
		target = doc
	}

	var rendered bytes.Buffer
	if err := html.Render(&rendered, target); err != nil {
		return inkbite.Result{}, err
	}

	markdown, err := htmltomarkdown.ConvertString(rendered.String())
	if err != nil {
		return inkbite.Result{}, err
	}

	return inkbite.Result{
		Markdown: strings.TrimSpace(markdown),
		Title:    title,
	}, nil
}

func removeNodes(node *html.Node, names ...string) {
	if node == nil {
		return
	}

	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}

	var walk func(*html.Node)
	walk = func(current *html.Node) {
		for child := current.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == html.ElementNode {
				if _, ok := nameSet[strings.ToLower(child.Data)]; ok {
					current.RemoveChild(child)
					child = next
					continue
				}
			}
			walk(child)
			child = next
		}
	}

	walk(node)
}

func findFirstNode(node *html.Node, name string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirstNode(child, name); found != nil {
			return found
		}
	}
	return nil
}

func findFirstText(node *html.Node, name string) string {
	target := findFirstNode(node, name)
	if target == nil {
		return ""
	}
	return strings.TrimSpace(extractText(target))
}

func extractText(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}

	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(extractText(child))
	}

	return builder.String()
}
