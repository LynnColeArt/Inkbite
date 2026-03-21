package ipynbconv

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/LynnColeArt/Inkbite"
)

func TestIPYNBConversion(t *testing.T) {
	input := `{
  "nbformat": 4,
  "metadata": {
    "language_info": {
      "name": "python"
    }
  },
  "cells": [
    {
      "cell_type": "markdown",
      "source": ["# Notebook Title\n", "Some context"]
    },
    {
      "cell_type": "code",
      "source": ["print('hi')\n"],
      "outputs": [
        {
          "output_type": "stream",
          "text": ["hi\n"]
        }
      ]
    }
  ]
}`

	converter := New()
	result, err := converter.Convert(context.Background(), bytes.NewReader([]byte(input)), inkbite.StreamInfo{
		Extension: ".ipynb",
	}, inkbite.ConvertOptions{})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	for _, fragment := range []string{
		"# Notebook Title",
		"```python\nprint('hi')\n```",
		"```text\nhi\n```",
	} {
		if !strings.Contains(result.Markdown, fragment) {
			t.Fatalf("expected %q in markdown, got %q", fragment, result.Markdown)
		}
	}
}
