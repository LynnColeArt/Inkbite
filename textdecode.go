package inkbite

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

// DecodeText decodes bytes using an explicit charset hint when one is available.
// If decoding fails, it falls back to best-effort UTF-8 replacement.
func DecodeText(data []byte, charsetLabel string) string {
	if charsetLabel == "" || strings.EqualFold(charsetLabel, "utf-8") {
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	}

	reader, err := charset.NewReaderLabel(charsetLabel, bytes.NewReader(data))
	if err != nil {
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	}

	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(bytes.ToValidUTF8(data, []byte("\uFFFD")))
	}

	return string(bytes.ToValidUTF8(decoded, []byte("\uFFFD")))
}
