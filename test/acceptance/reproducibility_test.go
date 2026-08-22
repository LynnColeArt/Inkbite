package acceptance_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

func TestCanonicalFormatsAreIdenticalAcrossOneHundredConversions(t *testing.T) {
	formats := canonicalFormats(t)
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			baseline := ingestCanonical(t, engine, format)
			want, err := json.Marshal(baseline)
			if err != nil {
				t.Fatal(err)
			}
			for run := 1; run < 100; run++ {
				got, err := json.Marshal(ingestCanonical(t, engine, format))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("conversion %d differs from canonical envelope", run)
				}
			}
		})
	}
}

func TestOneHundredConcurrentCanonicalRequestsRemainIsolated(t *testing.T) {
	formats := canonicalFormats(t)
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	want := make([][]byte, len(formats))
	for index, format := range formats {
		encoded, err := json.Marshal(ingestCanonical(t, engine, format))
		if err != nil {
			t.Fatal(err)
		}
		want[index] = encoded
	}

	const requests = 100
	failures := make(chan error, requests)
	var wait sync.WaitGroup
	for request := 0; request < requests; request++ {
		request := request
		wait.Add(1)
		go func() {
			defer wait.Done()
			index := request % len(formats)
			envelope, err := engine.Ingest(context.Background(), formats[index].source, &formats[index].hints, inkbite.IngestOptions{
				ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
			})
			if err != nil {
				failures <- fmt.Errorf("request %d: %w", request, err)
				return
			}
			encoded, err := json.Marshal(envelope)
			if err != nil {
				failures <- fmt.Errorf("request %d: %w", request, err)
				return
			}
			if !bytes.Equal(encoded, want[index]) {
				failures <- fmt.Errorf("request %d crossed envelope state", request)
				return
			}
			envelope.Source.Bytes[0] ^= 0xff
		}()
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}

	for index, format := range formats {
		encoded, err := json.Marshal(ingestCanonical(t, engine, format))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, want[index]) {
			t.Fatalf("post-mutation %s envelope was aliased", format.name)
		}
	}
}

func TestVerificationRejectsEveryRetainedObjectMutation(t *testing.T) {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	formats := canonicalFormats(t)
	pdfEnvelope := ingestCanonical(t, engine, formats[1])

	objects := []struct {
		name   string
		mutate func(*inkbite.IngestionEnvelope)
		omit   func(*inkbite.IngestionEnvelope)
	}{
		{
			name: "source",
			mutate: func(envelope *inkbite.IngestionEnvelope) {
				envelope.Source.Bytes[0] ^= 0x01
			},
			omit: func(envelope *inkbite.IngestionEnvelope) { envelope.Source.Bytes = nil },
		},
		{
			name: "primary",
			mutate: func(envelope *inkbite.IngestionEnvelope) {
				envelope.Primary.Bytes[0] ^= 0x01
			},
			omit: func(envelope *inkbite.IngestionEnvelope) { envelope.Primary.Bytes = nil },
		},
		{
			name: "derivative",
			mutate: func(envelope *inkbite.IngestionEnvelope) {
				envelope.Artifacts[0].Bytes[0] ^= 0x01
			},
			omit: func(envelope *inkbite.IngestionEnvelope) { envelope.Artifacts[0].Bytes = nil },
		},
	}
	for _, object := range objects {
		t.Run(object.name+" one byte", func(t *testing.T) {
			mutated := cloneEnvelope(pdfEnvelope)
			object.mutate(&mutated)
			if report := inkbite.VerifyEnvelope(mutated); report.Valid {
				t.Fatal("one-byte mutation verified")
			}
		})
		t.Run(object.name+" missing", func(t *testing.T) {
			mutated := cloneEnvelope(pdfEnvelope)
			object.omit(&mutated)
			if report := inkbite.VerifyEnvelope(mutated); report.Valid {
				t.Fatal("missing retained object verified")
			}
		})
	}

	t.Run("duplicate derivative", func(t *testing.T) {
		mutated := cloneEnvelope(pdfEnvelope)
		duplicate := mutated.Artifacts[0]
		duplicate.ArtifactID = "artifact-000002"
		duplicate.Relations[0].ToID = duplicate.ArtifactID
		mutated.Artifacts = append(mutated.Artifacts, duplicate)
		mutated.Provenance.OutputIdentities = append(mutated.Provenance.OutputIdentities, duplicate.Identity)
		if report := inkbite.VerifyEnvelope(mutated); report.Valid {
			t.Fatal("duplicate derivative verified")
		}
	})

	t.Run("cross envelope source", func(t *testing.T) {
		mutated := cloneEnvelope(pdfEnvelope)
		textEnvelope := ingestCanonical(t, engine, formats[0])
		mutated.Source = cloneEnvelope(textEnvelope).Source
		if report := inkbite.VerifyEnvelope(mutated); report.Valid {
			t.Fatal("cross-envelope source verified")
		}
	})
}

type canonicalFormat struct {
	name   string
	source []byte
	hints  inkbite.StreamInfo
}

func canonicalFormats(t *testing.T) []canonicalFormat {
	t.Helper()
	inner := makeZIP(t, []zipEntry{{name: "leaf.txt", body: []byte("nested leaf"), method: zip.Store}})
	return []canonicalFormat{
		{name: "text", source: []byte("canonical text\n"), hints: inkbite.StreamInfo{Filename: "canonical.txt", Extension: ".txt"}},
		{name: "PDF", source: retainedImagePDF(), hints: inkbite.StreamInfo{Filename: "canonical.pdf", Extension: ".pdf"}},
		{name: "office", source: minimalDOCX(t), hints: inkbite.StreamInfo{Filename: "canonical.docx", Extension: ".docx"}},
		{name: "nested ZIP", source: makeZIP(t, []zipEntry{{name: "inner.zip", body: inner, method: zip.Store}}), hints: inkbite.StreamInfo{Filename: "canonical.zip", Extension: ".zip"}},
	}
}

func ingestCanonical(t *testing.T, engine *inkbite.Engine, format canonicalFormat) inkbite.IngestionEnvelope {
	t.Helper()
	envelope, err := engine.Ingest(context.Background(), format.source, &format.hints, inkbite.IngestOptions{
		ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
	})
	if err != nil {
		t.Fatalf("%s Ingest(): %v", format.name, err)
	}
	if report := inkbite.VerifyEnvelope(envelope); !report.Valid {
		t.Fatalf("%s envelope invalid: %#v", format.name, report.Findings)
	}
	return envelope
}

type zipEntry struct {
	name   string
	body   []byte
	method uint16
}

func makeZIP(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: entry.method}
		header.SetMode(0o600)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func minimalDOCX(t *testing.T) []byte {
	t.Helper()
	return makeZIP(t, []zipEntry{
		{
			name:   "[Content_Types].xml",
			method: zip.Store,
			body: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`),
		},
		{
			name:   "word/document.xml",
			method: zip.Store,
			body: []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Canonical office text</w:t></w:r></w:p></w:body></w:document>`),
		},
	})
}
