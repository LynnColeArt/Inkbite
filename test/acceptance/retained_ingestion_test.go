package acceptance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

// Responsibility boundary: these tests are a host application. They use only
// Inkbite's exported registration, ingestion, model, and verification APIs.
func TestRetainedIngestionSurvivesFreshDiskReload(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		source     func() []byte
		artifacts  int
		wantMarker string
	}{
		{
			name:       "text",
			filename:   "retained.txt",
			source:     func() []byte { return []byte("retained host text\n") },
			wantMarker: "retained host text",
		},
		{
			name:       "PDF derivative",
			filename:   "retained.pdf",
			source:     retainedImagePDF,
			artifacts:  1,
			wantMarker: "inkbite-artifact:artifact-000001",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sourceDir := t.TempDir()
			storeDir := t.TempDir()
			sourcePath := filepath.Join(sourceDir, tc.filename)
			sourceBytes := tc.source()
			if err := os.WriteFile(sourcePath, sourceBytes, 0o600); err != nil {
				t.Fatal(err)
			}

			engine := inkbite.New()
			builtins.RegisterDefaultConverters(engine)
			envelope, err := engine.Ingest(context.Background(), sourcePath, nil, inkbite.IngestOptions{
				ConvertOptions: inkbite.ConvertOptions{PDFBackend: "purego"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if report := inkbite.VerifyEnvelope(envelope); !report.Valid {
				t.Fatalf("live envelope invalid: %#v", report.Findings)
			}
			if len(envelope.Artifacts) != tc.artifacts {
				t.Fatalf("artifacts = %d, want %d", len(envelope.Artifacts), tc.artifacts)
			}
			if !strings.Contains(string(envelope.Primary.Bytes), tc.wantMarker) {
				t.Fatalf("primary Markdown missing %q", tc.wantMarker)
			}

			if err := persistEnvelope(storeDir, envelope); err != nil {
				t.Fatal(err)
			}
			assertManifestContainsNoBytes(t, filepath.Join(storeDir, "envelope.json"))

			// Durability is tested only after every source/runtime value and the
			// disposable source directory are gone.
			sourceBytes = nil
			envelope = inkbite.IngestionEnvelope{}
			engine = nil
			if err := os.RemoveAll(sourceDir); err != nil {
				t.Fatal(err)
			}
			runtime.GC()

			reloaded, err := loadEnvelope(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			report := inkbite.VerifyEnvelope(reloaded)
			if !report.Valid {
				t.Fatalf("fresh retained envelope invalid: %#v", report.Findings)
			}
			if len(report.VerifiedArtifactIdentities) != 1+tc.artifacts {
				t.Fatalf("verified identities = %d, want %d", len(report.VerifiedArtifactIdentities), 1+tc.artifacts)
			}
			if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
				t.Fatalf("disposable source survived reload boundary: %v", err)
			}
		})
	}
}

func persistEnvelope(storeDir string, envelope inkbite.IngestionEnvelope) error {
	objectsDir := filepath.Join(storeDir, "objects")
	if err := os.MkdirAll(objectsDir, 0o700); err != nil {
		return err
	}
	if err := writeObject(objectsDir, "source", envelope.Source.Bytes); err != nil {
		return err
	}
	if err := writeObject(objectsDir, envelope.Primary.ArtifactID, envelope.Primary.Bytes); err != nil {
		return err
	}
	for _, artifact := range envelope.Artifacts {
		if err := writeObject(objectsDir, artifact.ArtifactID, artifact.Bytes); err != nil {
			return err
		}
	}

	manifest := cloneEnvelope(envelope)
	manifest.Source.Bytes = nil
	manifest.Primary.Bytes = nil
	for index := range manifest.Artifacts {
		manifest.Artifacts[index].Bytes = nil
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(storeDir, "envelope.json"), encoded, 0o600)
}

func loadEnvelope(storeDir string) (inkbite.IngestionEnvelope, error) {
	encoded, err := os.ReadFile(filepath.Join(storeDir, "envelope.json"))
	if err != nil {
		return inkbite.IngestionEnvelope{}, err
	}
	var envelope inkbite.IngestionEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return inkbite.IngestionEnvelope{}, err
	}
	objectsDir := filepath.Join(storeDir, "objects")
	envelope.Source.Bytes, err = readObject(objectsDir, "source")
	if err != nil {
		return inkbite.IngestionEnvelope{}, err
	}
	envelope.Primary.Bytes, err = readObject(objectsDir, envelope.Primary.ArtifactID)
	if err != nil {
		return inkbite.IngestionEnvelope{}, err
	}
	for index := range envelope.Artifacts {
		envelope.Artifacts[index].Bytes, err = readObject(objectsDir, envelope.Artifacts[index].ArtifactID)
		if err != nil {
			return inkbite.IngestionEnvelope{}, err
		}
	}
	return envelope, nil
}

func writeObject(dir, name string, data []byte) error {
	return os.WriteFile(filepath.Join(dir, name+".bin"), data, 0o600)
}

func readObject(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name+".bin"))
}

func cloneEnvelope(envelope inkbite.IngestionEnvelope) inkbite.IngestionEnvelope {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		panic(err)
	}
	var clone inkbite.IngestionEnvelope
	if err := json.Unmarshal(encoded, &clone); err != nil {
		panic(err)
	}
	return clone
}

func assertManifestContainsNoBytes(t *testing.T, path string) {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := bytes.Count(encoded, []byte(`"bytes":null`)), 2+bytes.Count(encoded, []byte(`"artifact_id":"artifact-000001"`)); got != want {
		t.Fatalf("null byte fields = %d, want %d in %s", got, want, encoded)
	}
}

func retainedImagePDF() []byte {
	textStream := "BT\n/F1 12 Tf\n10 10 Td\n(Retained PDF) Tj\nET\nq\n1 0 0 1 0 0 cm\n/Im1 Do\nQ"
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 20 20] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> /XObject << /Im1 6 0 R >> >> >>"),
		pdfStream([]byte(textStream), ""),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"),
		pdfStream([]byte{0}, "/Type /XObject /Subtype /Image /Width 1 /Height 1 /ColorSpace /DeviceGray /BitsPerComponent 8"),
	}
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

func pdfStream(data []byte, dictionary string) []byte {
	value := append([]byte(fmt.Sprintf("<< %s /Length %d >>\nstream\n", dictionary, len(data))), data...)
	return append(value, []byte("\nendstream")...)
}
