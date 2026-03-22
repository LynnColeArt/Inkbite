package builtins

import (
	"slices"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestRegisterDefaultConverters(t *testing.T) {
	engine := inkbite.New()
	RegisterDefaultConverters(engine)

	var names []string
	for _, converter := range engine.RegisteredConverters() {
		names = append(names, converter.Name())
	}

	for _, want := range []string{"ipynb", "xlsx", "docx", "pptx", "pdf", "csv", "epub", "rss", "zip", "html", "text"} {
		if !slices.Contains(names, want) {
			t.Fatalf("expected converter %q in defaults, got %v", want, names)
		}
	}
}
