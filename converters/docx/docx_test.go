package docxconv

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestDOCXConversion(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeZipFile(t, zw, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`)

	writeZipFile(t, zw, "word/document.xml", `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:pPr><w:pStyle w:val="Heading1"/></w:pPr>
      <w:r><w:t>Sample Doc</w:t></w:r>
    </w:p>
    <w:p>
      <w:r><w:t xml:space="preserve">Hello </w:t></w:r>
      <w:hyperlink r:id="rId1">
        <w:r><w:t>world</w:t></w:r>
      </w:hyperlink>
    </w:p>
    <w:tbl>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Name</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>Age</w:t></w:r></w:p></w:tc>
      </w:tr>
      <w:tr>
        <w:tc><w:p><w:r><w:t>Ada</w:t></w:r></w:p></w:tc>
        <w:tc><w:p><w:r><w:t>30</w:t></w:r></w:p></w:tc>
      </w:tr>
    </w:tbl>
  </w:body>
</w:document>`)

	writeZipFile(t, zw, "word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(buf.Bytes()), inkbite.StreamInfo{
		Extension: ".docx",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if result.Title != "Sample Doc" {
		t.Fatalf("expected title Sample Doc, got %q", result.Title)
	}

	for _, fragment := range []string{
		"# Sample Doc",
		"Hello [world](https://example.com)",
		"| Name | Age |",
		"| Ada | 30 |",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}

func writeZipFile(t *testing.T, zw *zip.Writer, name string, content string) {
	t.Helper()

	writer, err := zw.Create(name)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", name, err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("Write(%q) error = %v", name, err)
	}
}
