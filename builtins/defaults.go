package builtins

import (
	"github.com/LynnColeArt/Inkbite"
	csvconv "github.com/LynnColeArt/Inkbite/converters/csv"
	htmlconv "github.com/LynnColeArt/Inkbite/converters/html"
	ipynbconv "github.com/LynnColeArt/Inkbite/converters/ipynb"
	rssconv "github.com/LynnColeArt/Inkbite/converters/rss"
	textconv "github.com/LynnColeArt/Inkbite/converters/text"
)

// RegisterDefaultConverters installs the current built-in converter set.
func RegisterDefaultConverters(engine *inkbite.Engine) {
	if engine == nil {
		return
	}

	engine.RegisterConverter(ipynbconv.New())
	engine.RegisterConverter(csvconv.New())
	engine.RegisterConverter(rssconv.New())
	engine.RegisterConverter(htmlconv.New())
	engine.RegisterConverter(textconv.New())
}
