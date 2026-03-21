package normalize

import (
	"regexp"
	"strings"
)

var (
	blankLineRunRE = regexp.MustCompile(`\n{3,}`)
	dataURIRE      = regexp.MustCompile(`data:[^)\s]+`)
)

// Markdown applies shared output normalization to converter results.
func Markdown(input string, keepDataURIs bool) string {
	text := strings.ReplaceAll(input, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if isEmptyHeading(line) {
			continue
		}
		filtered = append(filtered, line)
	}

	text = strings.Join(filtered, "\n")
	text = blankLineRunRE.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	if !keepDataURIs {
		text = dataURIRE.ReplaceAllStringFunc(text, truncateDataURI)
	}

	return text
}

func isEmptyHeading(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}

	return strings.Trim(trimmed, "# ") == ""
}

func truncateDataURI(value string) string {
	if len(value) <= 96 {
		return value
	}

	prefix, _, found := strings.Cut(value, ",")
	if !found {
		return value
	}

	return prefix + ",..."
}
