package xlsxconv

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
	"sync"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/LynnColeArt/Inkbite"
	docxconv "github.com/LynnColeArt/Inkbite/converters/docx"
	pptxconv "github.com/LynnColeArt/Inkbite/converters/pptx"
	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
	"github.com/LynnColeArt/Inkbite/internal/testutil"
)

func TestXLSXContainerSecurityMatrix(t *testing.T) {
	valid := xlsxArchive(t)
	firstName := xlsxFirstArchiveName(t, valid)
	tests := []struct {
		name    string
		archive []byte
	}{
		{name: "traversal", archive: xlsxAppendArchiveEntry(t, valid, "../escape.xml", []byte("x"), 0)},
		{name: "portable collision", archive: xlsxAppendArchiveEntry(t, valid, strings.ToUpper(firstName), []byte("x"), 0)},
		{name: "symlink", archive: xlsxAppendArchiveEntry(t, valid, "xl/link.xml", []byte("target"), os.ModeSymlink|0o777)},
		{name: "forged declared size", archive: xlsxForgeCentralField(t, valid, 24, 1)},
		{name: "bad CRC", archive: xlsxForgeCentralField(t, valid, 16, 0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New().Convert(context.Background(), bytes.NewReader(tc.archive), inkbite.StreamInfo{Extension: ".xlsx"}, inkbite.ConvertOptions{})
			if !errors.Is(err, internalingestion.ErrIntegrityFailure) {
				t.Fatalf("Convert() error = %v, want integrity failure", err)
			}
		})
	}
}

func TestXLSXPreflightUsesCallerPolicyAndExistingLedger(t *testing.T) {
	archive := xlsxArchive(t)
	entries, expanded := xlsxArchiveAccounting(t, archive)
	policy := inkbite.DefaultIngestionPolicy()
	policy.MaxContainerEntries = entries
	policy.MaxExpandedBytes = expanded
	policy.MaxExpansionRatio = 100000
	if _, err := New().ConvertDetailed(context.Background(), bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".xlsx"}, inkbite.ConvertOptions{}, policy); err != nil {
		t.Fatalf("at-limit ConvertDetailed() error = %v", err)
	}

	tooSmall := policy
	tooSmall.MaxContainerEntryBytes = 1
	if _, err := New().ConvertDetailed(context.Background(), bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".xlsx"}, inkbite.ConvertOptions{}, tooSmall); !errors.Is(err, internalingestion.ErrLimitExceeded) {
		t.Fatalf("entry +1 ConvertDetailed() error = %v, want limit exceeded before excelize", err)
	}

	shared := policy
	shared.MaxContainerEntries++
	shared.MaxExpandedBytes++
	budget, err := internalingestion.NewRequestBudget(xlsxLimits(shared))
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
	if _, err := New().ConvertDetailed(ctx, bytes.NewReader(archive), inkbite.StreamInfo{Extension: ".xlsx"}, inkbite.ConvertOptions{}, shared); err != nil {
		t.Fatalf("shared-ledger ConvertDetailed() error = %v", err)
	}
	got := budget.Snapshot()
	if got.ContainerEntries != entries+1 || got.ExpandedBytes != expanded+1 || got.ContainerDepth != 0 {
		t.Fatalf("shared ledger snapshot = %+v, want entries=%d expanded=%d depth=0", got, entries+1, expanded+1)
	}
}

func TestXLSXEngineRejectsUnsafeArchiveBeforeThirdPartyExpansion(t *testing.T) {
	archive := xlsxAppendArchiveEntry(t, xlsxArchive(t), "../escape.xml", []byte("ignored by workbook parser"), 0)
	engine := inkbite.New()
	engine.RegisterConverter(New())
	_, err := engine.Ingest(context.Background(), archive, &inkbite.StreamInfo{Extension: ".xlsx"}, inkbite.IngestOptions{})
	if !errors.Is(err, inkbite.ErrIntegrityFailure) {
		t.Fatalf("Ingest() error = %v, want public integrity failure", err)
	}
}

func TestXLSXPreflightObservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Convert(ctx, bytes.NewReader(xlsxArchive(t)), inkbite.StreamInfo{Extension: ".xlsx"}, inkbite.ConvertOptions{})
	if !errors.Is(err, internalingestion.ErrCancellation) {
		t.Fatalf("Convert() error = %v, want cancellation", err)
	}
}

func TestOOXMLConvertersCancellationAndConcurrentPolicyIsolation(t *testing.T) {
	type formatCase struct {
		name    string
		data    []byte
		info    inkbite.StreamInfo
		marker  string
		hash    string
		convert func(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.IngestionPolicy) (inkbite.DetailedConversion, error)
	}
	formats := []formatCase{
		{
			name: "docx", data: testutil.BuildZipFixture(t, filepath.Join("..", "docx", "testdata", "simple")),
			info: inkbite.StreamInfo{Extension: ".docx"}, marker: "Sample Doc", hash: "2e1443dabe953decefb63bfeb2566c95444bca56659458363ff7ac3089409156",
			convert: func(ctx context.Context, reader io.ReadSeeker, info inkbite.StreamInfo, policy inkbite.IngestionPolicy) (inkbite.DetailedConversion, error) {
				return docxconv.New().ConvertDetailed(ctx, reader, info, inkbite.ConvertOptions{}, policy)
			},
		},
		{
			name: "pptx", data: testutil.BuildZipFixture(t, filepath.Join("..", "pptx", "testdata", "simple")),
			info: inkbite.StreamInfo{Extension: ".pptx"}, marker: "Deck Title", hash: "14435c6edf4499d47ffd8e959ca74656ed4984b57ecce6e765ebd70c7d67e68a",
			convert: func(ctx context.Context, reader io.ReadSeeker, info inkbite.StreamInfo, policy inkbite.IngestionPolicy) (inkbite.DetailedConversion, error) {
				return pptxconv.New().ConvertDetailed(ctx, reader, info, inkbite.ConvertOptions{}, policy)
			},
		},
		{
			name: "xlsx", data: xlsxArchive(t), info: inkbite.StreamInfo{Extension: ".xlsx"}, marker: "People", hash: "ab59ca068d4fc9b0af5d62a091551c75eb54cea2457e6c8392333b9ac95e498f",
			convert: func(ctx context.Context, reader io.ReadSeeker, info inkbite.StreamInfo, policy inkbite.IngestionPolicy) (inkbite.DetailedConversion, error) {
				return New().ConvertDetailed(ctx, reader, info, inkbite.ConvertOptions{}, policy)
			},
		},
	}
	for _, format := range formats {
		result, err := format.convert(context.Background(), bytes.NewReader(format.data), format.info, inkbite.DefaultIngestionPolicy())
		if err != nil {
			t.Fatalf("%s fidelity conversion: %v", format.name, err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(result.Result.Markdown))); got != format.hash {
			t.Fatalf("%s Markdown hash = %s, want %s", format.name, got, format.hash)
		}
	}

	for _, format := range formats {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := format.convert(ctx, bytes.NewReader(format.data), format.info, inkbite.DefaultIngestionPolicy()); !errors.Is(err, internalingestion.ErrCancellation) {
			t.Fatalf("%s cancelled conversion error = %v, want cancellation", format.name, err)
		}
	}

	const requests = 100
	errorsCh := make(chan error, requests)
	var group sync.WaitGroup
	for index := 0; index < requests; index++ {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			format := formats[index%len(formats)]
			policy := inkbite.DefaultIngestionPolicy()
			wantLimit := index%2 == 1
			if wantLimit {
				policy.MaxContainerEntries = 1
			}
			result, err := format.convert(context.Background(), bytes.NewReader(format.data), format.info, policy)
			if wantLimit {
				if !errors.Is(err, internalingestion.ErrLimitExceeded) {
					errorsCh <- fmt.Errorf("%s request %d error = %v, want limit", format.name, index, err)
				}
				return
			}
			if err != nil {
				errorsCh <- fmt.Errorf("%s request %d: %w", format.name, index, err)
				return
			}
			if !strings.Contains(result.Result.Markdown, format.marker) {
				errorsCh <- fmt.Errorf("%s request %d missing marker %q", format.name, index, format.marker)
			}
		}()
	}
	group.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
}

func xlsxArchive(t *testing.T) []byte {
	t.Helper()
	workbook := excelize.NewFile()
	defer func() { _ = workbook.Close() }()
	workbook.SetSheetName("Sheet1", "People")
	if err := workbook.SetSheetRow("People", "A1", &[]string{"name", "age"}); err != nil {
		t.Fatalf("SetSheetRow(A1) error = %v", err)
	}
	if err := workbook.SetSheetRow("People", "A2", &[]string{"Ada", "30"}); err != nil {
		t.Fatalf("SetSheetRow(A2) error = %v", err)
	}
	var buffer bytes.Buffer
	if err := workbook.Write(&buffer); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	return bytes.Clone(buffer.Bytes())
}

func xlsxLimits(policy inkbite.IngestionPolicy) internalingestion.Limits {
	return internalingestion.Limits{
		MaxSourceBytes: policy.MaxSourceBytes, MaxPrimaryBytes: policy.MaxPrimaryBytes,
		MaxArtifacts: policy.MaxArtifacts, MaxArtifactBytes: policy.MaxArtifactBytes,
		MaxTotalArtifactBytes: policy.MaxTotalArtifactBytes, MaxContainerEntries: policy.MaxContainerEntries,
		MaxContainerEntryBytes: policy.MaxContainerEntryBytes, MaxExpandedBytes: policy.MaxExpandedBytes,
		MaxContainerDepth: policy.MaxContainerDepth, MaxExpansionRatio: policy.MaxExpansionRatio,
	}
}

func xlsxArchiveAccounting(t *testing.T, archive []byte) (int, int64) {
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

func xlsxFirstArchiveName(t *testing.T, archive []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil || len(reader.File) == 0 {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	return reader.File[0].Name
}

func xlsxAppendArchiveEntry(t *testing.T, archive []byte, name string, body []byte, mode os.FileMode) []byte {
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

func xlsxForgeCentralField(t *testing.T, archive []byte, offset int, value uint32) []byte {
	t.Helper()
	forged := bytes.Clone(archive)
	central := bytes.Index(forged, []byte{'P', 'K', 1, 2})
	if central < 0 {
		t.Fatal("central-directory header not found")
	}
	binary.LittleEndian.PutUint32(forged[central+offset:central+offset+4], value)
	return forged
}
