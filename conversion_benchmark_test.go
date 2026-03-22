package inkbite_test

import (
	"bytes"
	"context"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

func BenchmarkConvertLargeText(b *testing.B) {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	payload := bytes.Repeat([]byte("A long line of plain text for benchmarking.\n"), 32*1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Convert(context.Background(), payload, &inkbite.StreamInfo{
			Extension: ".txt",
			Filename:  "benchmark.txt",
		}, inkbite.ConvertOptions{}); err != nil {
			b.Fatalf("Convert() error = %v", err)
		}
	}
}
