package docxconv

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
)

func TestDOCXContainerSecurityMatrix(t *testing.T) {
	valid := testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))
	firstName := firstArchiveName(t, valid)
	tests := []struct {
		name    string
		archive []byte
	}{
		{name: "traversal", archive: appendArchiveEntry(t, valid, "../escape.xml", []byte("x"), 0)},
		{name: "portable collision", archive: appendArchiveEntry(t, valid, strings.ToUpper(firstName), []byte("x"), 0)},
		{name: "symlink", archive: appendArchiveEntry(t, valid, "word/link.xml", []byte("target"), os.ModeSymlink|0o777)},
		{name: "forged declared size", archive: forgeCentralField(t, valid, 24, 1)},
		{name: "bad CRC", archive: forgeCentralField(t, valid, 16, 0)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New().Convert(context.Background(), bytes.NewReader(tc.archive), inkbite.StreamInfo{Extension: ".docx"}, inkbite.ConvertOptions{})
			if !errors.Is(err, internalingestion.ErrIntegrityFailure) {
				t.Fatalf("Convert() error = %v, want integrity failure", err)
			}
		})
	}
}

func TestDOCXUsesCallerPolicyAndExistingLedger(t *testing.T) {
	archive := testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))
	entries, expanded := archiveAccounting(t, archive)
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntries = entries
	policy.MaxExpandedBytes = expanded
	policy.MaxExpansionRatio = 100000

	if _, err := New().ConvertDetailed(context.Background(), bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".docx"}, inkbite.ConvertOptions{}, policy); err != nil {
		t.Fatalf("at-limit ConvertDetailed() error = %v", err)
	}

	tooSmall := policy
	tooSmall.MaxContainerEntries--
	if _, err := New().ConvertDetailed(context.Background(), bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".docx"}, inkbite.ConvertOptions{}, tooSmall); !errors.Is(err, internalingestion.ErrLimitExceeded) {
		t.Fatalf("count +1 ConvertDetailed() error = %v, want limit exceeded", err)
	}

	sharedPolicy := policy
	sharedPolicy.MaxContainerEntries++
	sharedPolicy.MaxExpandedBytes++
	budget, err := internalingestion.NewRequestBudget(docxLimits(sharedPolicy))
	if err != nil {
		t.Fatalf("NewRequestBudget() error = %v", err)
	}
	if err := budget.AdmitContainerEntry(1, 1, 1); err != nil {
		t.Fatalf("precharge budget: %v", err)
	}
	ctx, err := internalingestion.WithRequestBudget(context.Background(), budget)
	if err != nil {
		t.Fatalf("WithRequestBudget() error = %v", err)
	}
	if _, err := New().ConvertDetailed(ctx, bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".docx"}, inkbite.ConvertOptions{}, sharedPolicy); err != nil {
		t.Fatalf("shared-ledger ConvertDetailed() error = %v", err)
	}
	got := budget.Snapshot()
	if got.ContainerEntries != entries+1 || got.ExpandedBytes != expanded+1 || got.ContainerDepth != 0 {
		t.Fatalf("shared ledger snapshot = %+v, want entries=%d expanded=%d depth=0", got, entries+1, expanded+1)
	}
}

func docxLimits(policy inkbite.IngestionPolicy) internalingestion.Limits {
	return internalingestion.Limits{
		MaxSourceBytes: policy.MaxSourceBytes, MaxPrimaryBytes: policy.MaxPrimaryBytes,
		MaxArtifacts: policy.MaxArtifacts, MaxArtifactBytes: policy.MaxArtifactBytes,
		MaxTotalArtifactBytes: policy.MaxTotalArtifactBytes, MaxContainerEntries: policy.MaxContainerEntries,
		MaxContainerEntryBytes: policy.MaxContainerEntryBytes, MaxExpandedBytes: policy.MaxExpandedBytes,
		MaxContainerDepth: policy.MaxContainerDepth, MaxExpansionRatio: policy.MaxExpansionRatio,
	}
}

func archiveAccounting(t *testing.T, archive []byte) (int, int64) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	var expanded int64
	for _, file := range reader.File {
		expanded += int64(file.UncompressedSize64)
	}
	return len(reader.File), expanded
}

func firstArchiveName(t *testing.T, archive []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(reader.File) == 0 {
		t.Fatalf("zip.NewReader() = %v, entries=%d", err, len(reader.File))
	}
	return reader.File[0].Name
}

func appendArchiveEntry(t *testing.T, archive []byte, name string, body []byte, mode os.FileMode) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("Open(%q) error = %v", file.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("ReadAll(%q) error = %v", file.Name, err)
		}
		header := file.FileHeader
		member, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatalf("CreateHeader(%q) error = %v", file.Name, err)
		}
		if _, err := member.Write(content); err != nil {
			t.Fatalf("Write(%q) error = %v", file.Name, err)
		}
	}
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	if mode != 0 {
		header.SetMode(mode)
	}
	member, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatalf("CreateHeader(%q) error = %v", name, err)
	}
	if _, err := member.Write(body); err != nil {
		t.Fatalf("Write(%q) error = %v", name, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return bytes.Clone(output.Bytes())
}

func forgeCentralField(t *testing.T, archive []byte, offset int, value uint32) []byte {
	t.Helper()
	forged := bytes.Clone(archive)
	central := bytes.Index(forged, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("central-directory header not found")
	}
	binary.LittleEndian.PutUint32(forged[central+offset:central+offset+4], value)
	return forged
}
