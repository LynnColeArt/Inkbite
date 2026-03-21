package normalize

import "testing"

func TestMarkdownNormalization(t *testing.T) {
	input := "Line one  \r\n\r\n\r\n###   \r\nLine two\r\n"

	got := Markdown(input, false)
	want := "Line one\n\nLine two"
	if got != want {
		t.Fatalf("Markdown() = %q, want %q", got, want)
	}
}

func TestMarkdownTruncatesDataURIs(t *testing.T) {
	input := "![img](data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA)"

	got := Markdown(input, false)
	want := "![img](data:image/png;base64,...)"
	if got != want {
		t.Fatalf("Markdown() = %q, want %q", got, want)
	}
}
