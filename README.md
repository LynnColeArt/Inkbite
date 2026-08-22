# Inkbite

Inkbite is a Go library and command-line program that converts supported,
text-heavy documents to deterministic Markdown. The library also exposes the
additive `inkbite.ingestion/v1` envelope for callers that need exact source
bytes, independently retainable derivatives, conversion provenance, stable
warnings, and pure verification.

Inkbite favors useful structure over visual reconstruction. It is not a
MarkItDown parity port, a sandbox, an OCR engine, or a durable storage system.

## Security boundary

Treat every source byte and every extracted value as untrusted content.
Conversion parses attacker-controlled files inside the caller's process; it
does not make Markdown safe to render as HTML, execute embedded content, or
follow links. Apply the host application's escaping, content-security, and
retention policies after conversion.

Remote acquisition is disabled by default. Enabling it requires both explicit
request authority and a caller-supplied HTTP transport. Normal conversion does
not install components, download models or schemas, run OCR, invoke inference,
or follow derivative references.

A `sha256:<hex>` identity proves that bytes match a recorded digest. It does
not prove authorship, origin, safety, trust, or permission to execute or fetch
anything.

## Implemented formats

The built-in registry contains `ipynb`, `xlsx`, `xls`, `docx`, `pptx`, `pdf`,
`csv`, `epub`, `rss`, `zip`, `html`, and `text` converters. JSON and generic
XML use the text path unless a more specific converter accepts them.

DOCX, PPTX, PDF, and XLS support is intentionally reduced in scope. There is
no OCR, full layout reconstruction, chart understanding, image captioning,
audio transcription, or automatic external fallback in the conversion path.

## Build and test

```bash
make build
make ci
```

The repository's required Go checks can also be run directly:

```bash
go test ./...
go test -race ./...
go vet ./...
```

These commands are exercised by the repository CI and contract gates.

## Source-only releases

The release workflow publishes deterministic archives of the exact committed
tracked-source manifest plus `checksums.txt`. It does not publish executables,
object files, vendored modules, module-cache content, or dependency source.
Local and CI binary builds verify compilation only; they are not publication
inputs. Run `make dist VERSION=<version>` to produce
`inkbite_<version>_source.tar.gz` and `inkbite_<version>_source.zip` through the
same canonical packaging authority used by CI and tag releases.

Default Inkbite binaries link GPL-3.0-only xlsReader, are not MIT-only, and are not qualified for redistribution by this workflow.

A binary-release strategy requires a separate specification and independent
license review that closes the applicable GPL and transitive-license
obligations.

## CLI

Convert a local path:

```bash
inkbite ./report.pdf
```

The default success contract is Markdown on stdout, with one trailing newline
when needed, empty stderr, and exit status zero. It never emits an envelope,
provenance metadata, or binary artifacts implicitly. This snapshot is locked
by `TestRunConvertDefaultPathBehavior`.

Write Markdown to a file:

```bash
inkbite -o output.md ./paper.docx
```

Provide explicit type hints:

```bash
inkbite --extension .xml --mime-type text/xml --charset utf-8 ./sample.dat
```

The CLI exposes an explicit remote opt-in flag:

```bash
inkbite --http https://example.org/feed.xml
```

Without `--http`, an HTTP or HTTPS target fails before a request is issued.
The standalone CLI does not expose the caller-supplied transport capability
required by the hardened library policy, so `--http` alone also fails closed;
hosts that need remote ingestion use `WithHTTPClient` in the Go API.
The CLI snapshots for success, unsupported input, malformed input, cancellation,
local paths, and disabled remote access live in
[`cmd/inkbite/main_test.go`](cmd/inkbite/main_test.go).

List the registered converters:

```bash
inkbite --list-formats
```

## Legacy Go API

The original two-field `Result`, `Converter`, registration methods, and every
`Convert*` entry point remain source compatible:

```go
package main

import (
	"context"
	"fmt"

	inkbite "github.com/LynnColeArt/Inkbite"
	"github.com/LynnColeArt/Inkbite/builtins"
)

func main() {
	engine := inkbite.New()
	builtins.RegisterDefaultConverters(engine)

	result, err := engine.Convert(
		context.Background(),
		"./document.pdf",
		nil,
		inkbite.ConvertOptions{},
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Markdown)
}
```

The external-package compile fixture in
[`test/contract/legacy_compatibility_test.go`](test/contract/legacy_compatibility_test.go)
also constructs `Result` positionally, compares it with `==`, uses it as a map
key, registers a custom legacy-only converter, and calls `Convert`,
`ConvertPath`, `ConvertReader`, and `ConvertURI`.

## Detailed ingestion API

Use `Engine.Ingest` when the host needs retained source and output evidence:

```go
policy := inkbite.DefaultIngestionPolicy()
policy.Remote.Enabled = false

envelope, err := engine.Ingest(
	ctx,
	sourceBytes,
	&inkbite.StreamInfo{Filename: "brief.pdf"},
	inkbite.IngestOptions{Policy: policy},
)
if err != nil {
	return err
}
if report := inkbite.VerifyEnvelope(envelope); !report.Valid {
	return fmt.Errorf("invalid Inkbite envelope: %v", report.Findings)
}
```

This excerpt is exercised through the public API by
`TestDetailedIngestionAdaptsLegacyConverterAndVerificationIsPure` and
`TestGoEnvelopeRoundTripsThroughApprovedJSONSchema`.

`VerifyEnvelope` is pure: it performs no I/O, conversion, persistence,
component call, or network request. Success owns copies of source and output
bytes. Failure returns the zero envelope and an `errors.Is`-compatible public
category.

### Default ingestion policy

`DefaultIngestionPolicy` returns these materialized values:

| Constant | Value |
| --- | ---: |
| `DefaultMaxSourceBytes` | 32 MiB |
| `DefaultMaxPrimaryBytes` | 32 MiB |
| `DefaultMaxArtifacts` | 256 |
| `DefaultMaxArtifactBytes` | 8 MiB |
| `DefaultMaxTotalArtifactBytes` | 32 MiB |
| `DefaultMaxContainerEntries` | 256 |
| `DefaultMaxContainerEntryBytes` | 8 MiB |
| `DefaultMaxExpandedBytes` | 32 MiB |
| `DefaultMaxContainerDepth` | 4 |
| `DefaultMaxExpansionRatio` | 1000 |
| `Remote.Enabled` | `false` |
| `Component` | empty |

A completely zero `IngestOptions.Policy` materializes these defaults. A
partially populated policy is not merged with defaults; it is validated as the
caller's explicit policy.

### Artifacts, references, and warnings

The primary artifact is UTF-8 Markdown. Derived artifacts are separately owned
byte values with deterministic IDs, identities, relations, and ordering. A
Markdown reference such as
`inkbite-artifact:artifact-000001` resolves only against the same envelope; it
does not grant filesystem or network authority.

Warnings expose permitted degradation without raw payloads or backend error
text. For example, a malformed referenced PPTX notes part is omitted with the
stable `optional_extraction_failed` category and its canonical archive-member
location. Consumers should retain and inspect ordered warnings with the rest of
the envelope.

### Host-owned durability sequence

Inkbite returns values; the host owns persistence and cleanup. The required
sequence is:

`ingest -> verify -> persist -> discard -> reload -> verify`

Persist source bytes, primary Markdown, every derivative, envelope metadata,
and a host receipt atomically. Discard disposable conversion state only after
the host reloads the retained values and `VerifyEnvelope` succeeds again.
Inkbite does not choose a database, storage path, retention period, or deletion
time.

## Managed component commands

The CLI ships explicit component-management commands:

```bash
inkbite components list
inkbite config show
inkbite doctor
inkbite install ocr
```

These tested commands manage and inspect installation state. The separately
documented `--provider paddleocr` option is an explicit installer path, not a
conversion example. Component commands do not add OCR to
`ConvertOptions`, and normal conversion never invokes an installed OCR helper.
The explicit PaddleOCR install command creates a Python environment and may
download its pinned packages; ordinary conversion never runs that install
path. See [INKBITE_COMPONENTS_SPEC.md](INKBITE_COMPONENTS_SPEC.md) for the
shipped boundary.

## Contract and adoption records

- [Public detailed-ingestion API](kitty-specs/inkbite-ingestion-contract-01M0M3HW/contracts/public-api.md)
- [Envelope v1 JSON Schema](kitty-specs/inkbite-ingestion-contract-01M0M3HW/contracts/ingestion-envelope-v1.schema.json)
- [Verified-retention quickstart](kitty-specs/inkbite-ingestion-contract-01M0M3HW/quickstart.md)
- [Adopted components and license obligations](ADOPTED_COMPONENTS.md)

Inkbite's project-authored source is licensed under [MIT](LICENSE). Direct
dependencies retain their own licenses and distribution obligations; consult
`ADOPTED_COMPONENTS.md` before building or distributing a combined work.
