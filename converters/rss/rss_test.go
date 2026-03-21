package rssconv

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestRSSConversion(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example Feed</title>
    <description><![CDATA[<p>Feed intro</p>]]></description>
    <item>
      <title>First Post</title>
      <pubDate>Fri, 21 Mar 2026 10:00:00 GMT</pubDate>
      <description><![CDATA[<p>Hello <strong>world</strong></p>]]></description>
    </item>
  </channel>
</rss>`

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader([]byte(input)), inkbite.StreamInfo{
		MIMEType: "application/rss+xml",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	for _, fragment := range []string{
		"# Example Feed",
		"## First Post",
		"Published on: Fri, 21 Mar 2026 10:00:00 GMT",
		"Hello **world**",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}
