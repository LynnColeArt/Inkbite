package ipynbconv

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/LynnColeArt/Inkbite"
)

const priority = 10

var (
	ipynbExtensions = map[string]struct{}{
		".ipynb": {},
	}
	ipynbMIMETypes = map[string]struct{}{
		"application/json":         {},
		"application/x-ipynb+json": {},
	}
)

type notebook struct {
	Cells    []cell `json:"cells"`
	Metadata struct {
		LanguageInfo struct {
			Name string `json:"name"`
		} `json:"language_info"`
	} `json:"metadata"`
	NBFormat int `json:"nbformat"`
}

type cell struct {
	CellType string          `json:"cell_type"`
	Source   json.RawMessage `json:"source"`
	Outputs  []output        `json:"outputs"`
}

type output struct {
	OutputType string          `json:"output_type"`
	Text       json.RawMessage `json:"text"`
	Data       map[string]any  `json:"data"`
}

// Converter extracts markdown and code from Jupyter notebooks.
type Converter struct{}

// New returns an IPYNB converter.
func New() *Converter {
	return &Converter{}
}

func (c *Converter) Name() string {
	return "ipynb"
}

func (c *Converter) Priority() float64 {
	return priority
}

func (c *Converter) Accepts(
	_ context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) bool {
	if _, ok := ipynbExtensions[info.Extension]; ok {
		return true
	}
	if _, ok := ipynbMIMETypes[info.MIMEType]; !ok {
		return false
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return false
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return false
	}

	var probe struct {
		Cells    []json.RawMessage `json:"cells"`
		NBFormat int               `json:"nbformat"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}

	return probe.NBFormat > 0 && len(probe.Cells) > 0
}

func (c *Converter) Convert(
	_ context.Context,
	r io.ReadSeeker,
	info inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) (inkbite.Result, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return inkbite.Result{}, err
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return inkbite.Result{}, err
	}

	var nb notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return inkbite.Result{}, err
	}

	language := strings.TrimSpace(nb.Metadata.LanguageInfo.Name)
	var parts []string
	for _, cell := range nb.Cells {
		source := strings.TrimSpace(rawText(cell.Source))
		switch cell.CellType {
		case "markdown":
			if source != "" {
				parts = append(parts, source)
			}
		case "code":
			codeBlock := "```"
			if language != "" {
				codeBlock += language
			}
			codeBlock += "\n" + source + "\n```"
			parts = append(parts, codeBlock)

			outputText := strings.TrimSpace(extractOutputs(cell.Outputs))
			if outputText != "" {
				parts = append(parts, "```text\n"+outputText+"\n```")
			}
		default:
			if source != "" {
				parts = append(parts, source)
			}
		}
	}

	return inkbite.Result{
		Markdown: strings.Join(parts, "\n\n"),
	}, nil
}

func rawText(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}

	var asString string
	if err := json.Unmarshal(value, &asString); err == nil {
		return asString
	}

	var asSlice []string
	if err := json.Unmarshal(value, &asSlice); err == nil {
		return strings.Join(asSlice, "")
	}

	return ""
}

func extractOutputs(outputs []output) string {
	var parts []string
	for _, output := range outputs {
		text := strings.TrimSpace(rawText(output.Text))
		if text != "" {
			parts = append(parts, text)
			continue
		}

		if value, ok := output.Data["text/plain"]; ok {
			switch typed := value.(type) {
			case string:
				if typed != "" {
					parts = append(parts, typed)
				}
			case []any:
				var builder strings.Builder
				for _, item := range typed {
					if s, ok := item.(string); ok {
						builder.WriteString(s)
					}
				}
				if builder.Len() > 0 {
					parts = append(parts, builder.String())
				}
			}
		}
	}

	return strings.Join(parts, "\n")
}
