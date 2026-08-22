package pdfconv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func pdfEngine() *inkbite.Engine {
	engine := inkbite.New()
	engine.RegisterConverter(New())
	return engine
}

func imagePDFInfo() *inkbite.StreamInfo {
	return &inkbite.StreamInfo{Extension: ".pdf", Filename: "fixture.pdf"}
}

func ingestImagePDF(t *testing.T, engine *inkbite.Engine, source []byte, options inkbite.IngestOptions) inkbite.IngestionEnvelope {
	t.Helper()
	envelope, err := engine.Ingest(context.Background(), source, imagePDFInfo(), options)
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	return envelope
}

func decodeInlineImage(t *testing.T, markdown string) []byte {
	t.Helper()
	start := strings.Index(markdown, "base64,")
	if start < 0 {
		t.Fatalf("legacy Markdown has no inline image: %q", markdown)
	}
	start += len("base64,")
	end := strings.Index(markdown[start:], ")")
	if end < 0 {
		t.Fatalf("legacy Markdown has unterminated inline image: %q", markdown)
	}
	decoded, err := base64.StdEncoding.DecodeString(markdown[start : start+end])
	if err != nil {
		t.Fatalf("decode inline image: %v", err)
	}
	return decoded
}

func independentSHA256(data []byte) inkbite.ContentIdentity {
	sum := sha256.Sum256(data)
	return inkbite.ContentIdentity("sha256:" + hex.EncodeToString(sum[:]))
}

func TestPDFDetailedIngestionReturnsOwnedImageArtifactAndReference(t *testing.T) {
	t.Setenv("PATH", "")
	source := makeImageXObjectPDF("Detailed PDF image")
	legacyInline, err := New().Convert(
		context.Background(),
		bytes.NewReader(source),
		*imagePDFInfo(),
		inkbite.ConvertOptions{PDFBackend: "purego", KeepDataURIs: true},
	)
	if err != nil {
		t.Fatalf("legacy Convert() error = %v", err)
	}
	wantBytes := decodeInlineImage(t, legacyInline.Markdown)

	envelope := ingestImagePDF(t, pdfEngine(), source, inkbite.IngestOptions{
		ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
	})
	if len(envelope.Artifacts) != 1 {
		t.Fatalf("artifacts = %#v, want one embedded image", envelope.Artifacts)
	}
	artifact := envelope.Artifacts[0]
	if artifact.ArtifactID != "artifact-000001" || artifact.Role != inkbite.ArtifactRoleEmbeddedImage ||
		artifact.MediaType != "image/png" || artifact.SafeName != "page-000001-object-000006.png" {
		t.Fatalf("artifact contract = %#v", artifact)
	}
	if !bytes.Equal(artifact.Bytes, wantBytes) || artifact.ByteLength != int64(len(wantBytes)) || artifact.Identity != independentSHA256(wantBytes) {
		t.Fatalf("artifact bytes/identity = %d/%q, want %d/%q", artifact.ByteLength, artifact.Identity, len(wantBytes), independentSHA256(wantBytes))
	}
	wantOccurrence := "page-000001/object-000006"
	if len(artifact.Relations) != 1 || artifact.Relations[0].Occurrence != wantOccurrence ||
		artifact.Relations[0].ToID != artifact.ArtifactID || artifact.Relations[0].FromID != string(envelope.Source.Identity) {
		t.Fatalf("artifact relationships = %#v", artifact.Relations)
	}
	if !hasPDFAttribute(artifact.Attributes, "page", "1") || !hasPDFAttribute(artifact.Attributes, "object", "6") ||
		!hasPDFAttribute(artifact.Attributes, "width", "1") || !hasPDFAttribute(artifact.Attributes, "height", "1") {
		t.Fatalf("artifact attributes = %#v", artifact.Attributes)
	}
	markdown := string(envelope.Primary.Bytes)
	if strings.Contains(markdown, "data:image/") || strings.Count(markdown, "inkbite-artifact:artifact-000001") != 1 {
		t.Fatalf("detailed Markdown reference = %q", markdown)
	}
	if report := inkbite.VerifyEnvelope(envelope); !report.Valid {
		t.Fatalf("VerifyEnvelope() = %#v", report.Findings)
	}

	source[0] ^= 1
	wantBytes[0] ^= 1
	if envelope.Source.Bytes[0] == source[0] || envelope.Artifacts[0].Bytes[0] == wantBytes[0] {
		t.Fatal("detailed envelope aliases caller or legacy extraction buffers")
	}
	mutated := envelope
	mutated.Artifacts = append([]inkbite.ContentArtifact(nil), envelope.Artifacts...)
	mutated.Artifacts[0].Bytes = append([]byte(nil), envelope.Artifacts[0].Bytes...)
	mutated.Artifacts[0].Bytes[0] ^= 1
	if report := inkbite.VerifyEnvelope(mutated); report.Valid {
		t.Fatal("one-byte artifact mutation passed verification")
	}
}

var (
	detailedPDFReferencePattern = regexp.MustCompile(`!\[PDF image page ([0-9]+) object ([0-9]+) ([0-9]+)x([0-9]+) ([0-9]+) bpc\]\(inkbite-artifact:(artifact-[0-9]{6})\)`)
	detailedPDFTablePattern     = regexp.MustCompile(`(?m)^\| ([0-9]+) \| ([0-9]+) \| ([^|]+) \| ([0-9]+)x([0-9]+) \| ([0-9]+) \| ([0-9]+) \|$`)
)

func TestPDFDetailedReferencesResolveAfterCanonicalEnvelopeOrdering(t *testing.T) {
	t.Setenv("PATH", "")
	envelope := ingestImagePDF(t, pdfEngine(), makeTwoPageImagePDF(t, false), inkbite.IngestOptions{
		ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
	})
	if len(envelope.Artifacts) != 2 {
		t.Fatalf("artifacts = %#v, want JPEG and PNG derivatives", envelope.Artifacts)
	}
	if envelope.Artifacts[0].MediaType != "image/png" || envelope.Artifacts[1].MediaType != "image/jpeg" {
		t.Fatalf("fixture did not exercise reversed final ordering: %#v", envelope.Artifacts)
	}
	if err := validatePDFReferenceFidelity(envelope); err != nil {
		t.Fatal(err)
	}

	mutated := envelope
	mutated.Primary.Bytes = append([]byte(nil), envelope.Primary.Bytes...)
	matches := detailedPDFReferencePattern.FindAllSubmatch(mutated.Primary.Bytes, -1)
	if len(matches) != 2 {
		t.Fatalf("reference matches = %d, want 2", len(matches))
	}
	firstID, secondID := string(matches[0][6]), string(matches[1][6])
	const placeholder = "artifact-999999"
	mutated.Primary.Bytes = []byte(strings.NewReplacer(
		firstID, placeholder,
		secondID, firstID,
		placeholder, secondID,
	).Replace(string(mutated.Primary.Bytes)))
	mutated.Primary.ByteLength = int64(len(mutated.Primary.Bytes))
	mutated.Primary.Identity = independentSHA256(mutated.Primary.Bytes)
	mutated.Provenance.OutputIdentities = append([]inkbite.ContentIdentity(nil), envelope.Provenance.OutputIdentities...)
	mutated.Provenance.OutputIdentities[0] = mutated.Primary.Identity
	if report := inkbite.VerifyEnvelope(mutated); !report.Valid {
		t.Fatalf("reference-order mutation should retain structural validity: %#v", report.Findings)
	}
	if err := validatePDFReferenceFidelity(mutated); err == nil {
		t.Fatal("reference-order mutation silently retargeted valid artifact IDs")
	}
}

func TestPDFDetailedIdenticalBytesRetainDistinctReferenceOccurrences(t *testing.T) {
	t.Setenv("PATH", "")
	envelope := ingestImagePDF(t, pdfEngine(), makeTwoPageImagePDF(t, true), inkbite.IngestOptions{
		ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
	})
	if len(envelope.Artifacts) != 2 {
		t.Fatalf("artifacts = %#v, want two occurrence-specific derivatives", envelope.Artifacts)
	}
	if !bytes.Equal(envelope.Artifacts[0].Bytes, envelope.Artifacts[1].Bytes) {
		t.Fatal("fixture derivatives are not byte-identical")
	}
	if envelope.Artifacts[0].ArtifactID == envelope.Artifacts[1].ArtifactID ||
		envelope.Artifacts[0].Relations[0].Occurrence == envelope.Artifacts[1].Relations[0].Occurrence {
		t.Fatalf("identical derivatives lost occurrence identity: %#v", envelope.Artifacts)
	}
	if err := validatePDFReferenceFidelity(envelope); err != nil {
		t.Fatal(err)
	}
}

func validatePDFReferenceFidelity(envelope inkbite.IngestionEnvelope) error {
	references := detailedPDFReferencePattern.FindAllStringSubmatch(string(envelope.Primary.Bytes), -1)
	rows := detailedPDFTablePattern.FindAllStringSubmatch(string(envelope.Primary.Bytes), -1)
	if len(references) != len(envelope.Artifacts) || len(rows) != len(references) {
		return fmt.Errorf("PDF records/references/artifacts = %d/%d/%d", len(rows), len(references), len(envelope.Artifacts))
	}
	byID := make(map[string]inkbite.ContentArtifact, len(envelope.Artifacts))
	for _, artifact := range envelope.Artifacts {
		byID[artifact.ArtifactID] = artifact
	}
	for index, reference := range references {
		page, object, width, height, bpc, artifactID := reference[1], reference[2], reference[3], reference[4], reference[5], reference[6]
		row := rows[index]
		artifact, ok := byID[artifactID]
		if !ok {
			return fmt.Errorf("PDF reference %q does not resolve", artifactID)
		}
		if row[1] != page || row[2] != object || strings.TrimSpace(row[3]) != artifact.MediaType ||
			row[4] != width || row[5] != height || row[6] != bpc || row[7] != fmt.Sprint(len(artifact.Bytes)) {
			return fmt.Errorf("PDF visible record %d disagrees with artifact %q", index, artifactID)
		}
		if !hasPDFAttribute(artifact.Attributes, "page", page) ||
			!hasPDFAttribute(artifact.Attributes, "object", object) ||
			!hasPDFAttribute(artifact.Attributes, "width", width) ||
			!hasPDFAttribute(artifact.Attributes, "height", height) ||
			!hasPDFAttribute(artifact.Attributes, "bits_per_component", bpc) {
			return fmt.Errorf("PDF reference %q resolves to mismatched attributes %#v", artifactID, artifact.Attributes)
		}
		occurrence := fmt.Sprintf("page-%06s/object-%06s", page, object)
		if len(artifact.Relations) != 1 || artifact.Relations[0].Occurrence != occurrence || artifact.Relations[0].ToID != artifactID {
			return fmt.Errorf("PDF reference %q resolves to mismatched occurrence %#v", artifactID, artifact.Relations)
		}
	}
	return nil
}

func makeTwoPageImagePDF(t *testing.T, identicalRawImages bool) []byte {
	t.Helper()
	var firstData []byte
	firstDictionary := "/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode"
	if identicalRawImages {
		firstData = []byte{0}
		firstDictionary = "/ColorSpace /DeviceGray /BitsPerComponent 8"
	} else {
		var encoded bytes.Buffer
		pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
		pixel.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
		if err := jpeg.Encode(&encoded, pixel, &jpeg.Options{Quality: 100}); err != nil {
			t.Fatal(err)
		}
		firstData = encoded.Bytes()
	}
	secondData := []byte{0}
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 4 0 R /Resources << /XObject << /ImFirst 5 0 R >> >> >>"),
		pdfStreamObject([]byte("q\n1 0 0 1 0 0 cm\n/ImFirst Do\nQ"), ""),
		pdfStreamObject(firstData, "/Type /XObject /Subtype /Image /Width 1 /Height 1 "+firstDictionary),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 10 10] /Contents 7 0 R /Resources << /XObject << /ImSecond 8 0 R >> >> >>"),
		pdfStreamObject([]byte("q\n1 0 0 1 0 0 cm\n/ImSecond Do\nQ"), ""),
		pdfStreamObject(secondData, "/Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8"),
	}
	return buildBinaryPDF(objects)
}

func pdfStreamObject(data []byte, dictionary string) []byte {
	header := fmt.Sprintf("<< %s /Length %d >>\nstream\n", dictionary, len(data))
	object := append([]byte(header), data...)
	return append(object, []byte("\nendstream")...)
}

func buildBinaryPDF(objects [][]byte) []byte {
	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n", index+1)
		document.Write(object)
		document.WriteString("\nendobj\n")
	}
	xrefOffset := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n", len(objects)+1)
	document.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&document, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&document, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return document.Bytes()
}

func hasPDFAttribute(facts []inkbite.MetadataFact, kind, value string) bool {
	for _, fact := range facts {
		if fact.Kind == kind && fact.Value == value && fact.Origin == inkbite.MetadataOriginConverter {
			return true
		}
	}
	return false
}

func TestPDFLegacyEngineSnapshotsRemainExact(t *testing.T) {
	source := makeImageXObjectPDF("Legacy PDF snapshot")
	for _, options := range []inkbite.ConvertOptions{
		{PDFBackend: "purego"},
		{PDFBackend: "purego", KeepDataURIs: true},
	} {
		direct, err := New().Convert(context.Background(), bytes.NewReader(source), *imagePDFInfo(), options)
		if err != nil {
			t.Fatal(err)
		}
		viaEngine, err := pdfEngine().Convert(context.Background(), source, imagePDFInfo(), options)
		if err != nil {
			t.Fatal(err)
		}
		if viaEngine != direct {
			t.Fatalf("legacy engine result changed\n got: %#v\nwant: %#v", viaEngine, direct)
		}
		if strings.Contains(viaEngine.Markdown, "inkbite-artifact:") {
			t.Fatalf("legacy Markdown contains detailed reference: %q", viaEngine.Markdown)
		}
		if strings.Contains(viaEngine.Markdown, "data:image/") != options.KeepDataURIs {
			t.Fatalf("legacy KeepDataURIs=%v Markdown = %q", options.KeepDataURIs, viaEngine.Markdown)
		}
	}
}

func TestPDFDetailedOrderingIsStableAcrossOneHundredRuns(t *testing.T) {
	engine := pdfEngine()
	source := makeImageXObjectPDF("Deterministic PDF image")
	baseline := ingestImagePDF(t, engine, source, inkbite.IngestOptions{})
	want, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run < 100; run++ {
		got := ingestImagePDF(t, engine, source, inkbite.IngestOptions{})
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encoded, want) {
			t.Fatalf("run %d differs from canonical envelope", run)
		}
	}
}

func TestPDFDetailedTextOnlyDocumentHasNoArtifactReferences(t *testing.T) {
	envelope := ingestImagePDF(t, pdfEngine(), makeSimplePDF("Text only PDF"), inkbite.IngestOptions{})
	if len(envelope.Artifacts) != 0 || strings.Contains(string(envelope.Primary.Bytes), "inkbite-artifact:") {
		t.Fatalf("text-only detailed output = artifacts %#v, Markdown %q", envelope.Artifacts, envelope.Primary.Bytes)
	}
}

func TestPDFConcurrentDetailedAndLegacyConversion(t *testing.T) {
	engine := pdfEngine()
	source := makeImageXObjectPDF("Concurrent PDF image")
	wantDetailed, err := json.Marshal(ingestImagePDF(t, engine, source, inkbite.IngestOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	wantLegacy, err := engine.Convert(context.Background(), source, imagePDFInfo(), inkbite.ConvertOptions{KeepDataURIs: true})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 24
	var wg sync.WaitGroup
	errorsFound := make(chan string, workers)
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			if worker%2 == 0 {
				got, err := engine.Ingest(context.Background(), source, imagePDFInfo(), inkbite.IngestOptions{})
				if err != nil {
					errorsFound <- err.Error()
					return
				}
				encoded, _ := json.Marshal(got)
				if !bytes.Equal(encoded, wantDetailed) {
					errorsFound <- "detailed result differs"
				}
				return
			}
			got, err := engine.Convert(context.Background(), source, imagePDFInfo(), inkbite.ConvertOptions{KeepDataURIs: true})
			if err != nil {
				errorsFound <- err.Error()
				return
			}
			if got != wantLegacy {
				errorsFound <- "legacy result differs"
			}
		}()
	}
	wg.Wait()
	close(errorsFound)
	for failure := range errorsFound {
		t.Error(failure)
	}
}

func TestPDFFixtureBytesHavePortableIdentity(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "simple.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := independentSHA256(fixture), inkbite.ContentIdentity("sha256:0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b"); got != want {
		t.Fatalf("fixture identity = %q, want %q", got, want)
	}
}
