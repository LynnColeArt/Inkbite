package ooxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"math"
	"math/big"
	"net/url"
	"path"
	"strings"

	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
)

// Package is an in-memory OOXML zip package.
type Package struct {
	files map[string][]byte
}

// Open validates and reads a zipped OOXML package through the request ledger
// attached by the ingestion pipeline. A missing ledger fails closed so no raw
// archive-opening path can bypass request accounting.
func Open(ctx context.Context, data []byte) (*Package, error) {
	budget, ok := internalingestion.RequestBudgetFromContext(ctx)
	if !ok {
		return nil, internalingestion.ErrPolicyViolation
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, internalingestion.ErrIntegrityFailure
	}
	if err := budget.EnterContainer(); err != nil {
		return nil, err
	}
	defer func() { _ = budget.LeaveContainer() }()

	files := make(map[string][]byte, len(reader.File))
	seen := make(map[string]struct{}, len(reader.File))
	for _, file := range reader.File {
		if err := internalingestion.Checkpoint(ctx); err != nil {
			return nil, err
		}
		name, directory, err := validateMember(file)
		if err != nil {
			return nil, err
		}
		collisionKey, err := portableCollisionKey(name)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[collisionKey]; duplicate {
			return nil, internalingestion.ErrIntegrityFailure
		}
		seen[collisionKey] = struct{}{}

		content, err := readMember(ctx, file, budget)
		if err != nil {
			return nil, err
		}
		if !directory {
			files[name] = content
		}
	}
	if err := validateRelationshipParts(files); err != nil {
		return nil, err
	}

	return &Package{files: files}, nil
}

func portableCollisionKey(name string) (string, error) {
	current := strings.TrimSuffix(name, "/")
	for range 17 {
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return "", internalingestion.ErrIntegrityFailure
		}
		if decoded == current {
			return strings.ToLower(current), nil
		}
		current = decoded
	}
	return "", internalingestion.ErrIntegrityFailure
}

func validateMember(file *zip.File) (name string, directory bool, err error) {
	if file == nil || file.Name == "" {
		return "", false, internalingestion.ErrIntegrityFailure
	}
	directory = file.FileInfo().IsDir()
	if !directory && !file.FileInfo().Mode().IsRegular() {
		return "", false, internalingestion.ErrIntegrityFailure
	}
	name = file.Name
	if directory {
		if !strings.HasSuffix(name, "/") || strings.HasSuffix(name, "//") {
			return "", false, internalingestion.ErrIntegrityFailure
		}
		name = strings.TrimSuffix(name, "/")
	}
	canonical, canonicalErr := internalingestion.CanonicalArchivePath(name)
	if canonicalErr != nil || canonical != name {
		return "", false, internalingestion.ErrIntegrityFailure
	}
	if directory {
		name += "/"
	}
	return name, directory, nil
}

func readMember(ctx context.Context, file *zip.File, budget *internalingestion.RequestBudget) ([]byte, error) {
	limits := budget.Limits()
	if budget.Snapshot().ContainerEntries >= limits.MaxContainerEntries ||
		file.UncompressedSize64 > uint64(limits.MaxContainerEntryBytes) ||
		file.UncompressedSize64 > uint64(budget.RemainingExpandedBytes()) {
		return nil, internalingestion.ErrLimitExceeded
	}
	if file.CompressedSize64 > math.MaxInt64 || file.UncompressedSize64 > math.MaxInt64 {
		return nil, internalingestion.ErrLimitExceeded
	}
	compressed := int64(file.CompressedSize64)
	readLimit := min(limits.MaxContainerEntryBytes, budget.RemainingExpandedBytes())
	readLimit = min(readLimit, expansionReadLimit(compressed, limits.MaxExpansionRatio))

	rc, err := file.Open()
	if err != nil {
		return nil, internalingestion.ErrIntegrityFailure
	}
	owned, readErr := internalingestion.ReadBounded(ctx, rc, readLimit)
	closeErr := rc.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, internalingestion.ErrIntegrityFailure
	}
	declared := int64(file.UncompressedSize64)
	if owned.ByteLength != declared {
		return nil, internalingestion.ErrIntegrityFailure
	}
	if err := budget.AdmitContainerEntry(declared, compressed, owned.ByteLength); err != nil {
		return nil, err
	}
	return owned.Bytes, nil
}

func expansionReadLimit(compressed int64, ratio float64) int64 {
	if compressed == 0 {
		return 0
	}
	ratioValue := new(big.Rat)
	if ratioValue.SetFloat64(ratio) == nil {
		return 0
	}
	limit := new(big.Rat).Mul(new(big.Rat).SetInt64(compressed), ratioValue)
	whole := new(big.Int).Quo(limit.Num(), limit.Denom())
	if !whole.IsInt64() {
		return math.MaxInt64
	}
	return whole.Int64()
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
		ID         string `xml:"Id,attr"`
		Target     string `xml:"Target,attr"`
		TargetMode string `xml:"TargetMode,attr"`
	}
	type relationships struct {
		XMLName xml.Name       `xml:"Relationships"`
		Items   []relationship `xml:"Relationship"`
	}

	var rels relationships
	if err := xml.Unmarshal(data, &rels); err != nil {
		return nil, internalingestion.ErrIntegrityFailure
	}

	mapped := make(map[string]string, len(rels.Items))
	for _, rel := range rels.Items {
		if rel.ID == "" {
			return nil, internalingestion.ErrIntegrityFailure
		}
		if _, duplicate := mapped[rel.ID]; duplicate {
			return nil, internalingestion.ErrIntegrityFailure
		}
		target := strings.TrimSpace(rel.Target)
		if target == "" || target != rel.Target {
			return nil, internalingestion.ErrIntegrityFailure
		}
		lowerTarget := strings.ToLower(target)
		targetMode := strings.TrimSpace(rel.TargetMode)
		if targetMode != "" && !strings.EqualFold(targetMode, "external") {
			return nil, internalingestion.ErrIntegrityFailure
		}
		external := strings.EqualFold(targetMode, "external") ||
			strings.HasPrefix(lowerTarget, "http://") || strings.HasPrefix(lowerTarget, "https://") || strings.HasPrefix(lowerTarget, "mailto:")
		if external {
			if !strings.HasPrefix(lowerTarget, "http://") && !strings.HasPrefix(lowerTarget, "https://") && !strings.HasPrefix(lowerTarget, "mailto:") {
				return nil, internalingestion.ErrIntegrityFailure
			}
			mapped[rel.ID] = target
			continue
		}
		if strings.Contains(target, `\`) || strings.HasPrefix(target, "//") {
			return nil, internalingestion.ErrIntegrityFailure
		}
		packageAbsolute := strings.HasPrefix(target, "/")
		target = strings.TrimPrefix(target, "/")
		resolved := path.Clean(target)
		if !packageAbsolute {
			resolved = path.Clean(path.Join(base, target))
		}
		canonical, err := internalingestion.CanonicalArchivePath(resolved)
		if err != nil || canonical != resolved {
			return nil, internalingestion.ErrIntegrityFailure
		}
		mapped[rel.ID] = canonical
	}

	return mapped, nil
}

func validateRelationshipParts(files map[string][]byte) error {
	for relPath, data := range files {
		if !strings.HasSuffix(strings.ToLower(relPath), ".rels") {
			continue
		}
		base, ok := relationshipBase(relPath)
		if !ok {
			return internalingestion.ErrIntegrityFailure
		}
		relationships, err := RelationshipMap(data, base)
		if err != nil {
			return err
		}
		for _, target := range relationships {
			lower := strings.ToLower(target)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") {
				continue
			}
			if _, exists := files[target]; !exists {
				return internalingestion.ErrIntegrityFailure
			}
		}
	}
	return nil
}

func relationshipBase(relPath string) (string, bool) {
	if relPath == "_rels/.rels" {
		return "", true
	}
	directory := path.Dir(relPath)
	if path.Base(directory) != "_rels" {
		return "", false
	}
	ownerDirectory := path.Dir(directory)
	ownerName := strings.TrimSuffix(path.Base(relPath), ".rels")
	if ownerName == "" || ownerName == path.Base(relPath) {
		return "", false
	}
	owner := path.Join(ownerDirectory, ownerName)
	return path.Dir(owner), true
}
