package ooxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"testing"

	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
)

type packageEntry struct {
	name   string
	body   []byte
	mode   os.FileMode
	method uint16
}

func TestOpenUsesTheSharedRequestBudget(t *testing.T) {
	limits := packageLimits()
	limits.MaxContainerEntries = 2
	limits.MaxContainerEntryBytes = 4
	limits.MaxExpandedBytes = 8
	budget, err := internalingestion.NewRequestBudget(limits)
	if err != nil {
		t.Fatalf("NewRequestBudget() error = %v", err)
	}
	if err := budget.AdmitContainerEntry(1, 1, 1); err != nil {
		t.Fatalf("precharge budget: %v", err)
	}

	archive := buildPackage(t, packageEntry{name: "word/document.xml", body: []byte("doc")})
	pkg, err := Open(contextWithBudget(t, budget), archive)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got, ok := pkg.ReadFile("word/document.xml"); !ok || string(got) != "doc" {
		t.Fatalf("ReadFile() = %q, %v", got, ok)
	}

	want := internalingestion.BudgetSnapshot{ContainerEntries: 2, ExpandedBytes: 4}
	got := budget.Snapshot()
	if got.ContainerEntries != want.ContainerEntries || got.ExpandedBytes != want.ExpandedBytes || got.ContainerDepth != 0 {
		t.Fatalf("budget snapshot = %+v, want entries=%d expanded=%d depth=0", got, want.ContainerEntries, want.ExpandedBytes)
	}

	_, err = Open(contextWithBudget(t, budget), buildPackage(t, packageEntry{name: "word/second.xml", body: []byte("x")}))
	if !errors.Is(err, internalingestion.ErrLimitExceeded) {
		t.Fatalf("second Open() error = %v, want limit exceeded from shared ledger", err)
	}
}

func TestOpenContainerBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		limits  func() internalingestion.Limits
		entries []packageEntry
		wantErr error
	}{
		{
			name: "at entry count and expanded byte limits",
			limits: func() internalingestion.Limits {
				v := packageLimits()
				v.MaxContainerEntries = 2
				v.MaxContainerEntryBytes = 4
				v.MaxExpandedBytes = 8
				return v
			},
			entries: []packageEntry{{name: "a.xml", body: []byte("1234")}, {name: "b.xml", body: []byte("5678")}},
		},
		{
			name: "entry count plus one",
			limits: func() internalingestion.Limits {
				v := packageLimits()
				v.MaxContainerEntries = 1
				return v
			},
			entries: []packageEntry{{name: "a.xml", body: []byte("a")}, {name: "b.xml", body: []byte("b")}},
			wantErr: internalingestion.ErrLimitExceeded,
		},
		{
			name: "entry bytes plus one",
			limits: func() internalingestion.Limits {
				v := packageLimits()
				v.MaxContainerEntryBytes = 4
				return v
			},
			entries: []packageEntry{{name: "a.xml", body: []byte("12345")}},
			wantErr: internalingestion.ErrLimitExceeded,
		},
		{
			name: "expanded bytes plus one",
			limits: func() internalingestion.Limits {
				v := packageLimits()
				v.MaxExpandedBytes = 4
				return v
			},
			entries: []packageEntry{{name: "a.xml", body: []byte("12")}, {name: "b.xml", body: []byte("345")}},
			wantErr: internalingestion.ErrLimitExceeded,
		},
		{
			name: "expansion ratio at limit",
			limits: func() internalingestion.Limits {
				v := packageLimits()
				v.MaxExpansionRatio = 1
				return v
			},
			entries: []packageEntry{{name: "a.xml", body: []byte("1234")}},
		},
		{
			name: "expansion ratio plus one",
			limits: func() internalingestion.Limits {
				v := packageLimits()
				v.MaxExpansionRatio = 1
				return v
			},
			entries: []packageEntry{{name: "a.xml", body: bytes.Repeat([]byte("a"), 128), method: zip.Deflate}},
			wantErr: internalingestion.ErrLimitExceeded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			budget, err := internalingestion.NewRequestBudget(tc.limits())
			if err != nil {
				t.Fatalf("NewRequestBudget() error = %v", err)
			}
			_, err = Open(contextWithBudget(t, budget), buildPackage(t, tc.entries...))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Open() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestOpenRejectsUnsafeOrAmbiguousMembers(t *testing.T) {
	tests := []struct {
		name    string
		entries []packageEntry
	}{
		{name: "traversal", entries: []packageEntry{{name: "../document.xml", body: []byte("x")}}},
		{name: "backslash traversal", entries: []packageEntry{{name: `word\..\document.xml`, body: []byte("x")}}},
		{name: "duplicate", entries: []packageEntry{{name: "word/document.xml", body: []byte("a")}, {name: "word/document.xml", body: []byte("b")}}},
		{name: "portable collision", entries: []packageEntry{{name: "word/document.xml", body: []byte("a")}, {name: "WORD/DOCUMENT.XML", body: []byte("b")}}},
		{name: "encoded portable collision", entries: []packageEntry{{name: "word/document.xml", body: []byte("a")}, {name: "word/%64ocument.xml", body: []byte("b")}}},
		{name: "symlink", entries: []packageEntry{{name: "word/document.xml", body: []byte("target"), mode: os.ModeSymlink | 0o777}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			budget, err := internalingestion.NewRequestBudget(packageLimits())
			if err != nil {
				t.Fatalf("NewRequestBudget() error = %v", err)
			}
			_, err = Open(contextWithBudget(t, budget), buildPackage(t, tc.entries...))
			if !errors.Is(err, internalingestion.ErrIntegrityFailure) {
				t.Fatalf("Open() error = %v, want integrity failure", err)
			}
		})
	}
}

func TestOpenReadsMembersThroughEOFAndRejectsForgedMetadata(t *testing.T) {
	archive := buildPackage(t, packageEntry{name: "word/document.xml", body: []byte("checksum-body")})
	payload := bytes.Index(archive, []byte("checksum-body"))
	if payload < 0 {
		t.Fatal("fixture payload not found")
	}
	archive[payload] ^= 0xff
	budget, _ := internalingestion.NewRequestBudget(packageLimits())
	if _, err := Open(contextWithBudget(t, budget), archive); !errors.Is(err, internalingestion.ErrIntegrityFailure) {
		t.Fatalf("CRC-corrupt Open() error = %v, want integrity failure", err)
	}

	forged := buildPackage(t, packageEntry{name: "word/document.xml", body: []byte("123456789")})
	central := bytes.Index(forged, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("central-directory header not found")
	}
	binary.LittleEndian.PutUint32(forged[central+24:central+28], 1)
	budget, _ = internalingestion.NewRequestBudget(packageLimits())
	if _, err := Open(contextWithBudget(t, budget), forged); !errors.Is(err, internalingestion.ErrIntegrityFailure) {
		t.Fatalf("forged-size Open() error = %v, want integrity failure", err)
	}
}

func TestOpenRejectsCancellationAndUnsafeRelationshipTargets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	budget, _ := internalingestion.NewRequestBudget(packageLimits())
	ctx, err := internalingestion.WithRequestBudget(ctx, budget)
	if err != nil {
		t.Fatalf("WithRequestBudget() error = %v", err)
	}
	if _, err := Open(ctx, buildPackage(t, packageEntry{name: "word/document.xml", body: []byte("x")})); !errors.Is(err, internalingestion.ErrCancellation) {
		t.Fatalf("cancelled Open() error = %v, want cancellation", err)
	}

	rels := `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Target="../../escape.xml"/></Relationships>`
	archive := buildPackage(t,
		packageEntry{name: "word/document.xml", body: []byte("x")},
		packageEntry{name: "word/_rels/document.xml.rels", body: []byte(rels)},
	)
	budget, _ = internalingestion.NewRequestBudget(packageLimits())
	if _, err := Open(contextWithBudget(t, budget), archive); !errors.Is(err, internalingestion.ErrIntegrityFailure) {
		t.Fatalf("unsafe relationship Open() error = %v, want integrity failure", err)
	}
}

func TestRelationshipMapRejectsDuplicateIDsAndUnsafeTargets(t *testing.T) {
	tests := []string{
		`<NotRelationships/>`,
		`<Relationships><Relationship Id="rId1" Target="a.xml"/><Relationship Id="rId1" Target="b.xml"/></Relationships>`,
		`<Relationships><Relationship Id="rId1" Target="../../escape.xml"/></Relationships>`,
		`<Relationships><Relationship Id="rId1" Target="//authority/absolute.xml"/></Relationships>`,
		`<Relationships><Relationship Id="rId1" Target="word\\document.xml"/></Relationships>`,
		`<Relationships><Relationship Id="rId1" Target="document.xml" TargetMode="unexpected"/></Relationships>`,
	}
	for _, data := range tests {
		if _, err := RelationshipMap([]byte(data), "word"); !errors.Is(err, internalingestion.ErrIntegrityFailure) {
			t.Fatalf("RelationshipMap(%q) error = %v, want integrity failure", data, err)
		}
	}
}

func packageLimits() internalingestion.Limits {
	limits := internalingestion.DefaultLimits()
	limits.MaxExpansionRatio = 100000
	return limits
}

func contextWithBudget(t *testing.T, budget *internalingestion.RequestBudget) context.Context {
	t.Helper()
	ctx, err := internalingestion.WithRequestBudget(context.Background(), budget)
	if err != nil {
		t.Fatalf("WithRequestBudget() error = %v", err)
	}
	return ctx
}

func buildPackage(t *testing.T, entries ...packageEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		method := entry.method
		if method == 0 {
			method = zip.Store
		}
		header := &zip.FileHeader{Name: entry.name, Method: method}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		member, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", entry.name, err)
		}
		if _, err := member.Write(entry.body); err != nil {
			t.Fatalf("Write(%q) error = %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return bytes.Clone(buffer.Bytes())
}
