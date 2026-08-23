package acceptance_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"runtime"
	"strconv"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
	docxconv "github.com/LynnColeArt/Inkbite/converters/docx"
	pdfconv "github.com/LynnColeArt/Inkbite/converters/pdf"
	textconv "github.com/LynnColeArt/Inkbite/converters/text"
	zipconv "github.com/LynnColeArt/Inkbite/converters/zip"
)

func TestExplicitLargePolicyRepresentativeFamiliesAtExactBoundary(t *testing.T) {
	const boundary = int(inkbite.V1MaxSourceBytes)
	if os.Getenv("INKBITE_LARGE_PROFILE_QUALIFICATION") != "1" {
		t.Skip("run the sequential large-profile gate with INKBITE_LARGE_PROFILE_QUALIFICATION=1")
	}
	if testing.Short() {
		t.Fatal("large-profile qualification cannot be skipped in short mode")
	}

	plain := bytes.Repeat([]byte("a"), boundary)
	zipSource := prependToExactSize(t, makeZIP(t, []zipEntry{{name: "leaf.txt", body: []byte("large zip leaf\n"), method: 0}}), boundary)
	docxSource := prependToExactSize(t, minimalDOCX(t), boundary)
	pdfSource := padPDFToExactSize(t, retainedImagePDF(), boundary)

	tests := []struct {
		name   string
		source []byte
		hints  inkbite.StreamInfo
		engine func() *inkbite.Engine
	}{
		{
			name:   "plain",
			source: plain,
			hints:  inkbite.StreamInfo{Filename: "large.txt", Extension: ".txt", MIMEType: "text/plain"},
			engine: func() *inkbite.Engine {
				engine := inkbite.New()
				engine.RegisterConverter(textconv.New())
				return engine
			},
		},
		{
			name:   "zip",
			source: zipSource,
			hints:  inkbite.StreamInfo{Filename: "large.zip", Extension: ".zip", MIMEType: "application/zip"},
			engine: func() *inkbite.Engine {
				engine := inkbite.New()
				engine.RegisterConverter(textconv.New())
				engine.RegisterConverter(zipconv.New(engine))
				return engine
			},
		},
		{
			name:   "docx",
			source: docxSource,
			hints:  inkbite.StreamInfo{Filename: "large.docx", Extension: ".docx", MIMEType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
			engine: func() *inkbite.Engine {
				engine := inkbite.New()
				engine.RegisterConverter(docxconv.New())
				return engine
			},
		},
		{
			name:   "pdf",
			source: pdfSource,
			hints:  inkbite.StreamInfo{Filename: "large.pdf", Extension: ".pdf", MIMEType: "application/pdf"},
			engine: func() *inkbite.Engine {
				engine := inkbite.New()
				engine.RegisterConverter(pdfconv.New())
				return engine
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + string(os.PathSeparator) + tc.hints.Filename
			if err := os.WriteFile(path, tc.source, 0o600); err != nil {
				t.Fatal(err)
			}
			actual, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(actual) != boundary {
				t.Fatalf("actual bytes = %d, want %d", len(actual), boundary)
			}
			digest := sha256.Sum256(actual)
			t.Logf("family=%s actual_bytes=%d sha256=%x", tc.name, len(actual), digest)

			policy := inkbite.DefaultIngestionPolicy()
			policy.MaxSourceBytes = inkbite.V1MaxSourceBytes
			policy.MaxPrimaryBytes = inkbite.V1MaxPrimaryBytes
			envelope, err := tc.engine().Ingest(context.Background(), actual, &tc.hints, inkbite.IngestOptions{
				Policy:         policy,
				ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
			})
			if err != nil {
				t.Fatalf("Ingest() error = %v", err)
			}
			if envelope.Source.ByteLength != int64(boundary) || sha256.Sum256(envelope.Source.Bytes) != digest {
				t.Fatalf("retained source differs: bytes=%d identity=%s", envelope.Source.ByteLength, envelope.Source.Identity)
			}
			if report := inkbite.VerifyEnvelope(envelope); !report.Valid {
				t.Fatalf("VerifyEnvelope() = %#v", report)
			}
		})
		runtime.GC()
	}
}

func prependToExactSize(t *testing.T, trailer []byte, target int) []byte {
	t.Helper()
	if len(trailer) > target {
		t.Fatalf("fixture bytes = %d, exceed target %d", len(trailer), target)
	}
	result := make([]byte, target)
	copy(result[target-len(trailer):], trailer)
	return result
}

func padPDFToExactSize(t *testing.T, source []byte, target int) []byte {
	t.Helper()
	xref := bytes.Index(source, []byte("xref\n"))
	if xref < 0 {
		t.Fatal("PDF fixture has no xref")
	}
	startxref := bytes.Index(source[xref:], []byte("startxref\n"))
	if startxref < 0 {
		t.Fatal("PDF fixture has no startxref")
	}
	startxref += xref + len("startxref\n")
	numberEnd := startxref + bytes.IndexByte(source[startxref:], '\n')
	if numberEnd < startxref {
		t.Fatal("PDF fixture has malformed startxref")
	}

	filler := target - len(source) - 10
	for attempt := 0; attempt < 8; attempt++ {
		inserted := filler + 2
		newXref := xref + inserted
		var result bytes.Buffer
		result.Grow(target)
		result.Write(source[:xref])
		result.WriteByte('%')
		result.Write(bytes.Repeat([]byte("p"), filler))
		result.WriteByte('\n')
		result.Write(source[xref:startxref])
		result.WriteString(strconv.Itoa(newXref))
		result.Write(source[numberEnd:])
		if result.Len() == target {
			return result.Bytes()
		}
		filler += target - result.Len()
		if filler < 0 {
			t.Fatalf("cannot pad PDF to %d bytes", target)
		}
	}
	t.Fatalf("could not stabilize exact PDF size %d", target)
	return nil
}
