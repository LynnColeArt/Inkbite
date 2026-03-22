package builtins

import (
	"github.com/LynnColeArt/Inkbite"
	csvconv "github.com/LynnColeArt/Inkbite/converters/csv"
	docxconv "github.com/LynnColeArt/Inkbite/converters/docx"
	epubconv "github.com/LynnColeArt/Inkbite/converters/epub"
	htmlconv "github.com/LynnColeArt/Inkbite/converters/html"
	ipynbconv "github.com/LynnColeArt/Inkbite/converters/ipynb"
	pdfconv "github.com/LynnColeArt/Inkbite/converters/pdf"
	pptxconv "github.com/LynnColeArt/Inkbite/converters/pptx"
	rssconv "github.com/LynnColeArt/Inkbite/converters/rss"
	textconv "github.com/LynnColeArt/Inkbite/converters/text"
	xlsconv "github.com/LynnColeArt/Inkbite/converters/xls"
	xlsxconv "github.com/LynnColeArt/Inkbite/converters/xlsx"
	zipconv "github.com/LynnColeArt/Inkbite/converters/zip"
)

// RegisterDefaultConverters installs the current built-in converter set.
func RegisterDefaultConverters(engine *inkbite.Engine) {
	if engine == nil {
		return
	}

	engine.RegisterConverter(ipynbconv.New())
	engine.RegisterConverter(xlsxconv.New())
	engine.RegisterConverter(xlsconv.New())
	engine.RegisterConverter(docxconv.New())
	engine.RegisterConverter(pptxconv.New())
	engine.RegisterConverter(pdfconv.New())
	engine.RegisterConverter(csvconv.New())
	engine.RegisterConverter(epubconv.New())
	engine.RegisterConverter(rssconv.New())
	engine.RegisterConverter(zipconv.New(engine))
	engine.RegisterConverter(htmlconv.New())
	engine.RegisterConverter(textconv.New())
}
