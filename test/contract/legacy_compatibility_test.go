package contract_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

type legacyConverter struct {
	calls atomic.Int32
}

var _ inkbite.Converter = (*legacyConverter)(nil)

func (*legacyConverter) Name() string      { return "external-legacy" }
func (*legacyConverter) Priority() float64 { return 1 }

func (*legacyConverter) Accepts(context.Context, io.ReadSeeker, inkbite.StreamInfo, inkbite.ConvertOptions) bool {
	return true
}

func (c *legacyConverter) Convert(
	_ context.Context,
	reader io.ReadSeeker,
	_ inkbite.StreamInfo,
	_ inkbite.ConvertOptions,
) (inkbite.Result, error) {
	c.calls.Add(1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return inkbite.Result{}, err
	}
	return inkbite.Result{Markdown: "legacy:" + string(data), Title: "Legacy title"}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestLegacyResultShapeRemainsComparable(t *testing.T) {
	result := inkbite.Result{Markdown: "body", Title: "title"}
	if result != (inkbite.Result{Markdown: "body", Title: "title"}) {
		t.Fatalf("Result equality changed: %+v", result)
	}
	values := map[inkbite.Result]string{result: "retained"}
	if got := values[inkbite.Result{Markdown: "body", Title: "title"}]; got != "retained" {
		t.Fatalf("Result map-key lookup = %q, want retained", got)
	}
	if got := result.TextContent(); got != "body" {
		t.Fatalf("TextContent() = %q, want body", got)
	}
}

func TestExternalModuleCompilesUnkeyedComparableResult(t *testing.T) {
	dir := t.TempDir()
	module := "module externalconsumer\n\n" +
		"go 1.25.13\n\n" +
		"require github.com/LynnColeArt/Inkbite v0.0.0\n\n" +
		"replace github.com/LynnColeArt/Inkbite => " + filepath.ToSlash(repoRoot(t)) + "\n"
	source := `package externalconsumer

import (
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
)

func TestLegacyResultSourceShape(t *testing.T) {
	result := inkbite.Result{"body", "title"}
	if result != (inkbite.Result{"body", "title"}) {
		t.Fatal("legacy Result is no longer comparable")
	}
	values := map[inkbite.Result]string{result: "retained"}
	if values[inkbite.Result{"body", "title"}] != "retained" {
		t.Fatal("legacy Result is no longer a map key")
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatalf("write external go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "compatibility_test.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write external compatibility test: %v", err)
	}
	command := exec.Command("go", "test", "-mod=mod", "./...")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("external compatibility compile failed: %v\n%s", err, output)
	}
}

func TestLegacyConverterAndEveryEngineEntryPointRemainSourceCompatible(t *testing.T) {
	converter := &legacyConverter{}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected transport call")
	})
	engine := inkbite.New(inkbite.WithHTTPClient(&http.Client{Transport: transport}))
	engine.RegisterConverter(converter)
	if got := engine.RegisteredConverters(); len(got) != 1 || got[0].Name() != converter.Name() {
		t.Fatalf("RegisteredConverters() = %#v, want external legacy converter", got)
	}

	ctx := context.Background()
	info := &inkbite.StreamInfo{Extension: ".legacy"}
	opts := inkbite.ConvertOptions{
		KeepDataURIs: true,
		EnableHTTP:   false,
		MaxHTTPBytes: 1024,
		PDFBackend:   "purego",
	}
	path := filepath.Join(t.TempDir(), "source.legacy")
	if err := os.WriteFile(path, []byte("path"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	fileURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()

	tests := []struct {
		name string
		call func() (inkbite.Result, error)
		want string
	}{
		{name: "Convert", call: func() (inkbite.Result, error) {
			return engine.Convert(ctx, []byte("bytes"), info, opts)
		}, want: "legacy:bytes"},
		{name: "ConvertPath", call: func() (inkbite.Result, error) {
			return engine.ConvertPath(ctx, path, info, opts)
		}, want: "legacy:path"},
		{name: "ConvertReader", call: func() (inkbite.Result, error) {
			return engine.ConvertReader(ctx, strings.NewReader("reader"), info, opts)
		}, want: "legacy:reader"},
		{name: "ConvertURI", call: func() (inkbite.Result, error) {
			return engine.ConvertURI(ctx, fileURI, info, opts)
		}, want: "legacy:path"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.call()
			if err != nil {
				t.Fatalf("%s error = %v", test.name, err)
			}
			if result != (inkbite.Result{Markdown: test.want, Title: "Legacy title"}) {
				t.Fatalf("%s result = %+v", test.name, result)
			}
		})
	}
}

func TestDetailedIngestionAdaptsLegacyConverterAndVerificationIsPure(t *testing.T) {
	converter := &legacyConverter{}
	engine := inkbite.New()
	engine.RegisterConverter(converter)
	source := []byte("retained source")
	info := &inkbite.StreamInfo{Extension: ".legacy", Filename: "source.legacy"}

	envelope, err := engine.Ingest(context.Background(), source, info, inkbite.IngestOptions{})
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}
	if got, want := string(envelope.Primary.Bytes), "legacy:retained source"; got != want {
		t.Fatalf("primary bytes = %q, want %q", got, want)
	}
	if envelope.Provenance.Converter != converter.Name() {
		t.Fatalf("provenance converter = %q, want %q", envelope.Provenance.Converter, converter.Name())
	}
	source[0] = 'X'
	if got := string(envelope.Source.Bytes); got != "retained source" {
		t.Fatalf("owned source changed after caller mutation: %q", got)
	}

	calls := converter.calls.Load()
	report := inkbite.VerifyEnvelope(envelope)
	if !report.Valid || len(report.Findings) != 0 {
		t.Fatalf("VerifyEnvelope() = %+v, want valid", report)
	}
	if converter.calls.Load() != calls {
		t.Fatalf("VerifyEnvelope() invoked conversion: calls %d -> %d", calls, converter.calls.Load())
	}

	mutated := envelope
	mutated.Primary.Bytes = bytes.Clone(envelope.Primary.Bytes)
	mutated.Primary.Bytes[0] ^= 0xff
	if report := inkbite.VerifyEnvelope(mutated); report.Valid {
		t.Fatalf("VerifyEnvelope() accepted mutated primary: %+v", report)
	}
}

func Example_legacyConversion() {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)
	result, err := engine.Convert(
		context.Background(),
		[]byte("hello"),
		&inkbite.StreamInfo{Extension: ".txt"},
		inkbite.ConvertOptions{},
	)
	if err != nil {
		fmt.Println("error")
		return
	}
	fmt.Println(result.Markdown)
	// Output: hello
}

func Example_detailedIngestion() {
	engine := inkbite.New()
	engine.RegisterConverter(&legacyConverter{})
	policy := inkbite.DefaultIngestionPolicy()
	policy.Remote.Enabled = false
	envelope, err := engine.Ingest(
		context.Background(),
		[]byte("source"),
		&inkbite.StreamInfo{Extension: ".legacy"},
		inkbite.IngestOptions{Policy: policy},
	)
	if err != nil {
		fmt.Println("error")
		return
	}
	report := inkbite.VerifyEnvelope(envelope)
	fmt.Println(envelope.ContractVersion, string(envelope.Primary.Bytes), report.Valid)
	// Output: inkbite.ingestion/v1 legacy:source true
}
