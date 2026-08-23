package contract_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	inkbite "github.com/LynnColeArt/Inkbite"
)

func TestDefaultIngestionPolicyMatchesPublicContract(t *testing.T) {
	got := inkbite.DefaultIngestionPolicy()
	want := inkbite.IngestionPolicy{
		MaxSourceBytes:         32 << 20,
		MaxPrimaryBytes:        32 << 20,
		MaxArtifacts:           256,
		MaxArtifactBytes:       8 << 20,
		MaxTotalArtifactBytes:  32 << 20,
		MaxContainerEntries:    256,
		MaxContainerEntryBytes: 8 << 20,
		MaxExpandedBytes:       32 << 20,
		MaxContainerDepth:      4,
		MaxExpansionRatio:      1000,
		Remote:                 inkbite.RemotePolicy{Enabled: false},
	}
	if got != want {
		t.Fatalf("DefaultIngestionPolicy() = %+v, want %+v", got, want)
	}
}

func TestV1AbsoluteByteCeilingsMatchSchemaAndDocumentation(t *testing.T) {
	const (
		largeCeiling    = int64(256 << 20)
		artifactCeiling = int64(32 << 20)
	)
	if inkbite.V1MaxSourceBytes != largeCeiling || inkbite.V1MaxPrimaryBytes != largeCeiling || inkbite.V1MaxArtifactBytes != artifactCeiling {
		t.Fatalf("v1 Go ceilings = source:%d primary:%d artifact:%d", inkbite.V1MaxSourceBytes, inkbite.V1MaxPrimaryBytes, inkbite.V1MaxArtifactBytes)
	}

	var schema map[string]any
	if err := json.Unmarshal(readRepoFile(t, "kitty-specs", "inkbite-ingestion-contract-01M0M3HW", "contracts", "ingestion-envelope-v1.schema.json"), &schema); err != nil {
		t.Fatal(err)
	}
	definitions := schema["$defs"].(map[string]any)
	maximum := func(name string) int64 {
		definition := definitions[name].(map[string]any)
		properties := definition["properties"].(map[string]any)
		byteLength := properties["byte_length"].(map[string]any)
		return int64(byteLength["maximum"].(float64))
	}
	if got := maximum("source"); got != inkbite.V1MaxSourceBytes {
		t.Fatalf("schema source maximum = %d, want %d", got, inkbite.V1MaxSourceBytes)
	}
	if got := maximum("primary"); got != inkbite.V1MaxPrimaryBytes {
		t.Fatalf("schema primary maximum = %d, want %d", got, inkbite.V1MaxPrimaryBytes)
	}
	if got := maximum("artifact"); got != inkbite.V1MaxArtifactBytes {
		t.Fatalf("schema artifact maximum = %d, want %d", got, inkbite.V1MaxArtifactBytes)
	}

	for name, required := range map[string][]string{
		"README.md":       {"V1MaxSourceBytes", "V1MaxPrimaryBytes", "V1MaxArtifactBytes", "256 MiB", "32 MiB"},
		"INKBITE_SPEC.md": {"V1MaxSourceBytes", "V1MaxPrimaryBytes", "V1MaxArtifactBytes", "268435456", "33554432"},
	} {
		content := string(readRepoFile(t, name))
		for _, token := range required {
			if !strings.Contains(content, token) {
				t.Errorf("%s lacks v1 ceiling mirror token %q", name, token)
			}
		}
	}
}

func TestGoEnvelopeRoundTripsThroughApprovedJSONSchema(t *testing.T) {
	converter := &legacyConverter{}
	engine := inkbite.New()
	engine.RegisterConverter(converter)
	envelope, err := engine.Ingest(
		context.Background(),
		[]byte("schema source"),
		&inkbite.StreamInfo{Extension: ".legacy", Filename: "schema.legacy"},
		inkbite.IngestOptions{},
	)
	if err != nil {
		t.Fatalf("Ingest() error = %v", err)
	}

	wire, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	schemaData := readRepoFile(t, "kitty-specs", "inkbite-ingestion-contract-01M0M3HW", "contracts", "ingestion-envelope-v1.schema.json")
	if err := validateAgainstSchema(schemaData, wire); err != nil {
		t.Fatalf("Go envelope does not satisfy approved schema: %v\nJSON: %s", err, wire)
	}

	var roundTripped inkbite.IngestionEnvelope
	if err := json.Unmarshal(wire, &roundTripped); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(roundTripped, envelope) {
		t.Fatalf("JSON round trip changed envelope\ngot:  %#v\nwant: %#v", roundTripped, envelope)
	}
	if report := inkbite.VerifyEnvelope(roundTripped); !report.Valid {
		t.Fatalf("round-tripped envelope verification = %+v", report)
	}
}

func TestPublicDocumentationNamesTheShippedContract(t *testing.T) {
	checks := map[string][]string{
		"README.md": {
			"inkbite.ingestion/v1",
			"DefaultIngestionPolicy",
			"VerifyEnvelope",
			"ingest -> verify -> persist -> discard -> reload -> verify",
			"ADOPTED_COMPONENTS.md",
		},
		"INKBITE_SPEC.md": {
			"func (e *Engine) Ingest",
			"func VerifyEnvelope",
			"DefaultMaxExpansionRatio",
			"optional_extraction_failed",
		},
		"INKBITE_COMPONENTS_SPEC.md": {
			"Shipped boundary",
			"normal conversion never installs or invokes OCR",
			"ConvertOptions has no OCR field",
		},
		filepath.Join("skills", "inkbite", "SKILL.md"): {
			"Engine.Ingest",
			"VerifyEnvelope",
			"Remote.Enabled",
		},
		"ADOPTED_COMPONENTS.md": {
			"Classification: inspiration",
			"Classification: copied code",
			"Classification: dependency",
			"GPL-3.0-only",
		},
	}
	for name, required := range checks {
		content := string(readRepoFile(t, strings.Split(name, string(filepath.Separator))...))
		for _, text := range required {
			if !strings.Contains(content, text) {
				t.Errorf("%s does not contain %q", name, text)
			}
		}
	}
}

func TestOwnedDocumentationRelativeLinksResolve(t *testing.T) {
	files := []string{
		"README.md",
		"INKBITE_SPEC.md",
		"INKBITE_COMPONENTS_SPEC.md",
		filepath.Join("skills", "inkbite", "SKILL.md"),
		"ADOPTED_COMPONENTS.md",
	}
	linkPattern := regexp.MustCompile(`\[[^]]+\]\(([^)]+)\)`)
	for _, name := range files {
		data := readRepoFile(t, strings.Split(name, string(filepath.Separator))...)
		for _, match := range linkPattern.FindAllStringSubmatch(string(data), -1) {
			target := strings.TrimSpace(strings.SplitN(match[1], "#", 2)[0])
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			resolved := filepath.Join(repoRoot(t), filepath.Dir(name), filepath.FromSlash(target))
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s link %q does not resolve: %v", name, match[1], err)
			}
		}
	}
}

func TestAdoptedComponentsPinsEveryDirectModule(t *testing.T) {
	type record struct {
		version string
		commit  string
		license string
	}
	records := map[string]record{
		"github.com/JohannesKaufmann/html-to-markdown/v2": {
			version: "v2.5.0", commit: "3006818b20a61b0a36eb86321aef57d3d017c27e", license: "MIT",
		},
		"github.com/dslipak/pdf": {
			version: "v0.0.2", commit: "636e0c026eb4fc360db4e964ac51005acd6286e3", license: "BSD-3-Clause",
		},
		"github.com/pdfcpu/pdfcpu": {
			version: "v0.12.1", commit: "148d18d48afbe63e1c55741280adba696306e5c2", license: "Apache-2.0",
		},
		"github.com/shakinm/xlsReader": {
			version: "v0.9.12", commit: "cb2bf4031fc7b9d539e3d07ab15219ff240630d7", license: "GPL-3.0-only",
		},
		"github.com/xuri/excelize/v2": {
			version: "v2.10.1", commit: "5ad5ab3af0054c55bdce09f1530085600e9f2e45", license: "BSD-3-Clause",
		},
		"golang.org/x/net": {
			version: "v0.55.0", commit: "7770ec48d03fec35e378665337b4faca93c38423", license: "BSD-3-Clause",
		},
	}
	goMod := string(readRepoFile(t, "go.mod"))
	adopted := string(readRepoFile(t, "ADOPTED_COMPONENTS.md"))
	for module, record := range records {
		if !strings.Contains(goMod, module+" "+record.version) {
			t.Errorf("go.mod does not select recorded direct module %s %s", module, record.version)
		}
		for _, evidence := range []string{module, record.version, record.commit, record.license} {
			if !strings.Contains(adopted, evidence) {
				t.Errorf("ADOPTED_COMPONENTS.md lacks %s evidence %q", module, evidence)
			}
		}
	}
}

func readRepoFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, parts...)...))
	if err != nil {
		t.Fatalf("read repository file %q: %v", filepath.Join(parts...), err)
	}
	return data
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func validateAgainstSchema(schemaData []byte, instanceData []byte) error {
	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("decode schema: %w", err)
	}
	var instance any
	if err := json.Unmarshal(instanceData, &instance); err != nil {
		return fmt.Errorf("decode instance: %w", err)
	}
	return validateSchemaValue(schema, schema, instance, "$")
}

func validateSchemaValue(root map[string]any, schema map[string]any, value any, path string) error {
	if reference, ok := schema["$ref"].(string); ok {
		const prefix = "#/$defs/"
		if !strings.HasPrefix(reference, prefix) {
			return fmt.Errorf("%s: unsupported schema reference %q", path, reference)
		}
		definitions, ok := root["$defs"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s: schema has no definitions", path)
		}
		resolved, ok := definitions[strings.TrimPrefix(reference, prefix)].(map[string]any)
		if !ok {
			return fmt.Errorf("%s: schema reference %q is missing", path, reference)
		}
		return validateSchemaValue(root, resolved, value, path)
	}
	if constant, ok := schema["const"]; ok && !reflect.DeepEqual(value, constant) {
		return fmt.Errorf("%s: value %#v does not equal const %#v", path, value, constant)
	}
	if allowed, ok := schema["enum"].([]any); ok {
		matched := false
		for _, candidate := range allowed {
			matched = matched || reflect.DeepEqual(value, candidate)
		}
		if !matched {
			return fmt.Errorf("%s: value %#v is not in enum %#v", path, value, allowed)
		}
	}

	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: got %T, want object", path, value)
		}
		properties, _ := schema["properties"].(map[string]any)
		for _, name := range stringSlice(schema["required"]) {
			if _, ok := object[name]; !ok {
				return fmt.Errorf("%s: missing required property %q", path, name)
			}
		}
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			for name := range object {
				if _, ok := properties[name]; !ok {
					return fmt.Errorf("%s: unexpected property %q", path, name)
				}
			}
		}
		names := make([]string, 0, len(object))
		for name := range object {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			propertySchema, ok := properties[name].(map[string]any)
			if !ok {
				continue
			}
			if err := validateSchemaValue(root, propertySchema, object[name], path+"."+name); err != nil {
				return err
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: got %T, want array", path, value)
		}
		if minimum, ok := schema["minItems"].(float64); ok && len(array) < int(minimum) {
			return fmt.Errorf("%s: array length %d is below %d", path, len(array), int(minimum))
		}
		if maximum, ok := schema["maxItems"].(float64); ok && len(array) > int(maximum) {
			return fmt.Errorf("%s: array length %d exceeds %d", path, len(array), int(maximum))
		}
		itemSchema, _ := schema["items"].(map[string]any)
		for index, item := range array {
			if err := validateSchemaValue(root, itemSchema, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: got %T, want string", path, value)
		}
		if minimum, ok := schema["minLength"].(float64); ok && len(text) < int(minimum) {
			return fmt.Errorf("%s: string length %d is below %d", path, len(text), int(minimum))
		}
		if pattern, ok := schema["pattern"].(string); ok {
			compiled, err := regexp.Compile(pattern)
			if err != nil || !compiled.MatchString(text) {
				return fmt.Errorf("%s: string %q does not match %q", path, text, pattern)
			}
		}
		if schema["contentEncoding"] == "base64" {
			if _, err := base64.StdEncoding.DecodeString(text); err != nil {
				return fmt.Errorf("%s: invalid base64: %w", path, err)
			}
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || math.Trunc(number) != number {
			return fmt.Errorf("%s: got %#v, want integer", path, value)
		}
		if err := validateNumberBounds(schema, number, path); err != nil {
			return err
		}
	case "number":
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s: got %#v, want number", path, value)
		}
		if err := validateNumberBounds(schema, number, path); err != nil {
			return err
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: got %T, want boolean", path, value)
		}
	}
	return nil
}

func validateNumberBounds(schema map[string]any, value float64, path string) error {
	if minimum, ok := schema["minimum"].(float64); ok && value < minimum {
		return fmt.Errorf("%s: %v is below %v", path, value, minimum)
	}
	if maximum, ok := schema["maximum"].(float64); ok && value > maximum {
		return fmt.Errorf("%s: %v exceeds %v", path, value, maximum)
	}
	return nil
}

func stringSlice(value any) []string {
	raw, _ := value.([]any)
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
