package pptxconv

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestPPTXConversion(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeZipFile(t, zw, "[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`)

	writeZipFile(t, zw, "ppt/presentation.xml", `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId1"/>
    <p:sldId id="257" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`)

	writeZipFile(t, zw, "ppt/_rels/presentation.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
</Relationships>`)

	writeZipFile(t, zw, "ppt/slides/slide1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="2" name="Title 1"/>
          <p:nvPr><p:ph type="title"/></p:nvPr>
        </p:nvSpPr>
        <p:txBody>
          <a:bodyPr/>
          <a:lstStyle/>
          <a:p><a:r><a:t>Deck Title</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="3" name="Content Placeholder 2"/>
          <p:nvPr><p:ph type="body"/></p:nvPr>
        </p:nvSpPr>
        <p:txBody>
          <a:bodyPr/>
          <a:lstStyle/>
          <a:p>
            <a:r><a:t xml:space="preserve">Hello </a:t></a:r>
            <a:r>
              <a:rPr><a:hlinkClick r:id="rIdHyper"/></a:rPr>
              <a:t>world</a:t>
            </a:r>
          </a:p>
          <a:p>
            <a:r><a:t>Second paragraph</a:t></a:r>
          </a:p>
        </p:txBody>
      </p:sp>
      <p:graphicFrame>
        <p:nvGraphicFramePr>
          <p:cNvPr id="4" name="Table 4"/>
        </p:nvGraphicFramePr>
        <a:graphic>
          <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">
            <a:tbl>
              <a:tr>
                <a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>Name</a:t></a:r></a:p></a:txBody></a:tc>
                <a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>Role</a:t></a:r></a:p></a:txBody></a:tc>
              </a:tr>
              <a:tr>
                <a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>Ada</a:t></a:r></a:p></a:txBody></a:tc>
                <a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>Researcher</a:t></a:r></a:p></a:txBody></a:tc>
              </a:tr>
            </a:tbl>
          </a:graphicData>
        </a:graphic>
      </p:graphicFrame>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	writeZipFile(t, zw, "ppt/slides/_rels/slide1.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdHyper" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
  <Relationship Id="rIdNotes" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/>
</Relationships>`)

	writeZipFile(t, zw, "ppt/notesSlides/notesSlide1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="5" name="Header Placeholder"/>
          <p:nvPr><p:ph type="hdr"/></p:nvPr>
        </p:nvSpPr>
        <p:txBody>
          <a:bodyPr/>
          <a:p><a:r><a:t>Ignore me</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="6" name="Notes Placeholder"/>
          <p:nvPr><p:ph type="body"/></p:nvPr>
        </p:nvSpPr>
        <p:txBody>
          <a:bodyPr/>
          <a:p><a:r><a:t>Presenter note</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:notes>`)

	writeZipFile(t, zw, "ppt/slides/slide2.xml", `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="7" name="Content Placeholder"/>
          <p:nvPr><p:ph type="body"/></p:nvPr>
        </p:nvSpPr>
        <p:txBody>
          <a:bodyPr/>
          <a:p><a:r><a:t>Closing remarks</a:t></a:r></a:p>
        </p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader(buf.Bytes()), inkbite.StreamInfo{
		Extension: ".pptx",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	if result.Title != "Deck Title" {
		t.Fatalf("expected title Deck Title, got %q", result.Title)
	}

	for _, fragment := range []string{
		"## Slide 1: Deck Title",
		"Hello [world](https://example.com)",
		"Second paragraph",
		"| Name | Role |",
		"| Ada | Researcher |",
		"### Notes",
		"Presenter note",
		"## Slide 2",
		"Closing remarks",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}

	if strings.Contains(result.Markdown, "Ignore me") {
		t.Fatalf("expected header placeholder to be skipped, got %q", result.Markdown)
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
