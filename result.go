package inkbite

// Result is the normalized output of a successful conversion.
type Result struct {
	Markdown string
	Title    string
}

// TextContent is a soft alias kept for compatibility with the Python project.
func (r Result) TextContent() string {
	return r.Markdown
}
