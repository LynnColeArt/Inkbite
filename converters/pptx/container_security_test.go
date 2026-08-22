package pptxconv

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
)

func TestPPTXContainerSecurityMatrix(t *testing.T) {
	valid := testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))
	firstName := pptxFirstArchiveName(t, valid)
	tests := []struct {
		name    string
		archive []byte
	}{
		{name: "traversal", archive: pptxAppendArchiveEntry(t, valid, "../escape.xml", []byte("x"), 0)},
		{name: "portable collision", archive: pptxAppendArchiveEntry(t, valid, strings.ToUpper(firstName), []byte("x"), 0)},
		{name: "symlink", archive: pptxAppendArchiveEntry(t, valid, "ppt/link.xml", []byte("target"), os.ModeSymlink|0o777)},
		{name: "forged declared size", archive: pptxForgeCentralField(t, valid, 24, 1)},
		{name: "bad CRC", archive: pptxForgeCentralField(t, valid, 16, 0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New().Convert(context.Background(), bytes.NewReader(tc.archive), inkbite.StreamInfo{Extension: ".pptx"}, inkbite.ConvertOptions{})
			if !errors.Is(err, internalingestion.ErrIntegrityFailure) {
				t.Fatalf("Convert() error = %v, want integrity failure", err)
			}
		})
	}
}

func TestPPTXUsesCallerPolicyAndExistingLedger(t *testing.T) {
	archive := testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))
	entries, expanded := pptxArchiveAccounting(t, archive)
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntries = entries
	policy.MaxExpandedBytes = expanded
	policy.MaxExpansionRatio = 100000
	if _, err := New().ConvertDetailed(context.Background(), bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".pptx"}, inkbite.ConvertOptions{}, policy); err != nil {
		t.Fatalf("at-limit ConvertDetailed() error = %v", err)
	}

	tooSmall := policy
	tooSmall.MaxExpandedBytes--
	if _, err := New().ConvertDetailed(context.Background(), bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".pptx"}, inkbite.ConvertOptions{}, tooSmall); !errors.Is(err, internalingestion.ErrLimitExceeded) {
		t.Fatalf("expanded +1 ConvertDetailed() error = %v, want limit exceeded", err)
	}

	shared := policy
	shared.MaxContainerEntries++
	shared.MaxExpandedBytes++
	budget, err := internalingestion.NewRequestBudget(pptxLimits(shared))
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
	if _, err := New().ConvertDetailed(ctx, bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".pptx"}, inkbite.ConvertOptions{}, shared); err != nil {
		t.Fatalf("shared-ledger ConvertDetailed() error = %v", err)
	}
	got := budget.Snapshot()
	if got.ContainerEntries != entries+1 || got.ExpandedBytes != expanded+1 || got.ContainerDepth != 0 {
		t.Fatalf("shared ledger snapshot = %+v, want entries=%d expanded=%d depth=0", got, entries+1, expanded+1)
	}
}

func TestPPTXEngineIngestWarnsWhenReferencedNotesAreMalformed(t *testing.T) {
	archive := testutil.BuildZipFixture(t, filepath.Join("testdata", "simple"))
	archive = pptxReplaceArchiveEntry(
		t,
		archive,
		"ppt/notesSlides/notesSlide1.xml",
		[]byte(`<p:notes xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">`),
	)
	info := inkbite.StreamInfo{Extension: ".pptx"}

	legacy, err := New().Convert(context.Background(), bytes.NewReader(archive), info, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	legacyHash := fmt.Sprintf("%x", sha256.Sum256([]byte(legacy.Markdown)))
	if want := "78fa850dc1b053510191c03b03a8ec184de8c51998d25ddf7eb177424b5ced4f"; legacyHash != want {
		t.Fatalf("legacy Markdown hash = %s, want %s", legacyHash, want)
	}

	engine := inkbite.New()
	engine.RegisterConverter(New())
	envelope, err := engine.Ingest(context.Background(), archive, &info, inkbite.IngestOptions{})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if got, want := string(envelope.Primary.Bytes), legacy.Markdown; got != want {
		t.Fatalf("Ingest() primary Markdown = %q, want legacy Markdown %q", got, want)
	}
	wantWarning := inkbite.WarningRecord{
		Category:  "optional_extraction_failed",
		Converter: "pptx",
		Location:  "ppt/notesSlides/notesSlide1.xml",
		Detail:    "notes omitted",
	}
	if len(envelope.Warnings) != 1 || envelope.Warnings[0] != wantWarning {
		t.Fatalf("Ingest() warnings = %+v, want [%+v]", envelope.Warnings, wantWarning)
	}
}

func pptxLimits(policy inkbite.IngestionPolicy) internalingestion.Limits {
	return internalingestion.Limits{
		MaxSourceBytes: policy.MaxSourceBytes, MaxPrimaryBytes: policy.MaxPrimaryBytes,
		MaxArtifacts: policy.MaxArtifacts, MaxArtifactBytes: policy.MaxArtifactBytes,
		MaxTotalArtifactBytes: policy.MaxTotalArtifactBytes, MaxContainerEntries: policy.MaxContainerEntries,
		MaxContainerEntryBytes: policy.MaxContainerEntryBytes, MaxExpandedBytes: policy.MaxExpandedBytes,
		MaxContainerDepth: policy.MaxContainerDepth, MaxExpansionRatio: policy.MaxExpansionRatio,
	}
}

func pptxArchiveAccounting(t *testing.T, archive []byte) (int, int64) {
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

func pptxFirstArchiveName(t *testing.T, archive []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(reader.File) == 0 {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	return reader.File[0].Name
}

func pptxAppendArchiveEntry(t *testing.T, archive []byte, name string, body []byte, mode os.FileMode) []byte {
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

func pptxReplaceArchiveEntry(t *testing.T, archive []byte, name string, replacement []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	found := false
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
		if file.Name == name {
			content = replacement
			found = true
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
	if !found {
		t.Fatalf("archive member %q not found", name)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return bytes.Clone(output.Bytes())
}

func pptxForgeCentralField(t *testing.T, archive []byte, offset int, value uint32) []byte {
	t.Helper()
	forged := bytes.Clone(archive)
	central := bytes.Index(forged, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("central-directory header not found")
	}
	binary.LittleEndian.PutUint32(forged[central+offset:central+offset+4], value)
	return forged
}
