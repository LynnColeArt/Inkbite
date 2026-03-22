package ooxml

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strings"
)

// Package is an in-memory OOXML zip package.
type Package struct {
	files map[string][]byte
}

// Open reads a zipped OOXML package into memory.
func Open(data []byte) (*Package, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		files[file.Name] = content
	}

	return &Package{files: files}, nil
}

// ReadFile returns the contents of a package member.
func (p *Package) ReadFile(name string) ([]byte, bool) {
	if p == nil {
		return nil, false
	}
	value, ok := p.files[name]
	return value, ok
}

// Node is a minimal XML tree node that preserves child ordering.
type Node struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
	Nodes   []Node     `xml:",any"`
}

// ParseNode parses XML bytes into a generic node tree.
func ParseNode(data []byte) (*Node, error) {
	var node Node
	if err := xml.Unmarshal(data, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

// Child returns the first child with the given local name.
func (n *Node) Child(local string) *Node {
	if n == nil {
		return nil
	}
	for idx := range n.Nodes {
		if strings.EqualFold(n.Nodes[idx].XMLName.Local, local) {
			return &n.Nodes[idx]
		}
	}
	return nil
}

// Children returns all children with the given local name.
func (n *Node) Children(local string) []*Node {
	if n == nil {
		return nil
	}
	var children []*Node
	for idx := range n.Nodes {
		if strings.EqualFold(n.Nodes[idx].XMLName.Local, local) {
			children = append(children, &n.Nodes[idx])
		}
	}
	return children
}

// Attr returns the first attribute value whose local name matches.
func (n *Node) Attr(local string) string {
	if n == nil {
		return ""
	}
	for _, attr := range n.Attrs {
		if strings.EqualFold(attr.Name.Local, local) {
			return attr.Value
		}
	}
	return ""
}

// AttrNS returns the first attribute value whose namespace URI and local name match.
func (n *Node) AttrNS(space string, local string) string {
	if n == nil {
		return ""
	}
	for _, attr := range n.Attrs {
		if attr.Name.Space == space && strings.EqualFold(attr.Name.Local, local) {
			return attr.Value
		}
	}
	return ""
}

// Text returns the concatenated character data of the subtree.
func (n *Node) Text() string {
	if n == nil {
		return ""
	}

	var builder strings.Builder
	if strings.TrimSpace(n.Content) != "" {
		builder.WriteString(n.Content)
	}
	for idx := range n.Nodes {
		builder.WriteString(n.Nodes[idx].Text())
	}

	return builder.String()
}

// RelationshipMap parses a relationships part into an ID -> target map.
func RelationshipMap(data []byte, base string) (map[string]string, error) {
	type relationship struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	}
	type relationships struct {
		Items []relationship `xml:"Relationship"`
	}

	var rels relationships
	if err := xml.Unmarshal(data, &rels); err != nil {
		return nil, err
	}

	mapped := make(map[string]string, len(rels.Items))
	for _, rel := range rels.Items {
		target := rel.Target
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:") {
			mapped[rel.ID] = target
			continue
		}
		if base != "" {
			mapped[rel.ID] = path.Clean(path.Join(base, target))
		} else {
			mapped[rel.ID] = path.Clean(target)
		}
	}

	return mapped, nil
}
