# Inkbite Shipped Specification

## Status and authority

This document describes the public behavior present in the repository. The
approved detailed-ingestion authorities are the
[public API contract](kitty-specs/inkbite-ingestion-contract-01M0M3HW/contracts/public-api.md)
and [v1 JSON Schema](kitty-specs/inkbite-ingestion-contract-01M0M3HW/contracts/ingestion-envelope-v1.schema.json).
When this document and executable behavior differ, tests and those approved
contracts identify the defect; this file does not grant authority to rewrite
the approved contracts.

The detailed contract identifier is `inkbite.ingestion/v1`. Future
incompatible wire behavior requires another contract version and translation,
not silent mutation of v1.

## Product boundary

Inkbite converts supported document sources to normalized Markdown. The
detailed operation additionally returns owned source bytes, the primary
Markdown artifact, zero or more independently retainable derivatives,
deterministic provenance, and ordered warnings.

The shipped conversion path does not provide OCR, inference, image captioning,
audio transcription, full visual-layout reconstruction, hidden external
fallbacks, automatic model or component downloads, or durable persistence.
Reduced-scope extraction is intentional for DOCX, PPTX, PDF, and XLS.

## Legacy compatibility surface

The original result is exactly two comparable fields:

```go
type Result struct {
	Markdown string
	Title    string
}
```

It remains valid to construct `Result` positionally, compare it with `==`, use
it as a map key, and call `TextContent`. The external compile fixture is
[`test/contract/legacy_compatibility_test.go`](test/contract/legacy_compatibility_test.go).

The original converter interface remains sufficient:

```go
type Converter interface {
	Name() string
	Priority() float64
	Accepts(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) bool
	Convert(context.Context, io.ReadSeeker, StreamInfo, ConvertOptions) (Result, error)
}
```

`Engine.RegisterConverter`, `Engine.RegisteredConverters`, `Engine.Convert`,
`Engine.ConvertPath`, `Engine.ConvertReader`, and `Engine.ConvertURI` retain
their signatures. All legacy conversion entry points dispatch through the same
engine pipeline as detailed ingestion and project the legacy Markdown/title
result.

The shipped legacy options are:

```go
type ConvertOptions struct {
	KeepDataURIs bool
	EnableHTTP   bool
	MaxHTTPBytes int64
	PDFBackend   string
}
```

`PDFBackend` accepts `auto` and `purego`. No other PDF backend is shipped.

## Detailed ingestion

```go
func (e *Engine) Ingest(
	ctx context.Context,
	source any,
	hints *StreamInfo,
	options IngestOptions,
) (IngestionEnvelope, error)

func VerifyEnvelope(envelope IngestionEnvelope) VerificationReport
```

`Ingest` is additive; it does not enlarge `Result` or require legacy
converters to implement a new interface. When a converter only implements
`Converter`, the engine creates source identity, primary artifact, provenance,
and empty derivative/warning collections around the legacy result.

`DetailedConverter` is an optional capability that may return raw derivatives,
safe facts, safe warnings, backend, and explicitly selected component labels.
The engine assigns artifact IDs and identities, canonicalizes order and
references, applies output budgets, and verifies the final envelope.

`VerifyEnvelope` recomputes identities and lengths; validates contract version,
shape, ordering, references, and policy consistency; and returns ordered typed
findings. It performs no I/O, conversion, persistence, component invocation, or
network request.

## Source kinds and ownership

The engine accepts local path strings, `[]byte`, `io.Reader`, `io.ReadSeeker`,
and `file:`, `data:`, `http:`, or `https:` URI strings. The resulting public
source kinds are `bytes`, `reader`, `file`, `data_uri`, and `remote`.

Successful envelopes own exact copies of source and output bytes. They do not
alias caller buffers or mutable converter scratch storage. An error returns the
zero envelope; no partial success is authoritative.

Context cancellation is checked at cooperative boundaries. A caller-owned
non-cooperative `Read` or `Seek` remains synchronously joined until it returns;
Inkbite does not claim completed cancellation while its own worker continues in
the background.

## Default policy

`DefaultIngestionPolicy` materializes these exported constants:

| Constant | Value |
| --- | ---: |
| `DefaultMaxSourceBytes` | `33554432` |
| `DefaultMaxPrimaryBytes` | `33554432` |
| `DefaultMaxArtifacts` | `256` |
| `DefaultMaxArtifactBytes` | `8388608` |
| `DefaultMaxTotalArtifactBytes` | `33554432` |
| `DefaultMaxContainerEntries` | `256` |
| `DefaultMaxContainerEntryBytes` | `8388608` |
| `DefaultMaxExpandedBytes` | `33554432` |
| `DefaultMaxContainerDepth` | `4` |
| `DefaultMaxExpansionRatio` | `1000` |
| `Remote.Enabled` | `false` |
| `Component` | `""` |

The additive v1 contract also exports `V1MaxSourceBytes` and
`V1MaxPrimaryBytes` at `268435456`. Callers must opt in with a complete finite
policy; the values do not widen either 32 MiB default. The absolute per-derived
artifact ceiling remains `V1MaxArtifactBytes == 33554432`. Go verification and
the envelope schema mirror these three values.

The zero `IngestOptions.Policy` selects these defaults. A nonzero policy is
validated as supplied; missing fields are not filled independently.

Container accounting covers ZIP, EPUB, DOCX, PPTX, and XLSX before trusted
expansion. Source, primary output, derivative count, per-derivative bytes, and
aggregate derivative bytes are also bounded.

## Remote and component authority

Remote acquisition is disabled by default. A remote request requires
`Policy.Remote.Enabled` for detailed ingestion (or `ConvertOptions.EnableHTTP`
for legacy conversion) and a caller-supplied HTTP client installed with
`WithHTTPClient`. Redirect destinations and resolved address classes are
re-evaluated, and response bytes remain bounded.

The standalone CLI exposes `--http` but no transport-injection option, so that
flag supplies only the request opt-in and remote acquisition still fails closed.

`IngestionPolicy.Component` is an explicit selection label. An installed
managed component is not selected merely because it exists. Managed component
installation is a separate CLI command; normal conversion never downloads or
installs anything.

## Identity, artifacts, and references

Content identity is `sha256:<64 lowercase hexadecimal characters>` over exact
bytes. Identity proves byte equality only; it is not proof of origin,
authorship, safety, or execution authority.

The primary artifact ID is `artifact-000000`. Derivatives receive deterministic
envelope-local IDs beginning at `artifact-000001`. The currently shipped
derivative role is `embedded_image` for extracted PDF images. References use
`inkbite-artifact:<artifact-id>` and resolve only inside the same envelope.

The public relation kinds are `derived_from`, `embedded_in`, and
`referenced_by`. A relation or Markdown reference never authorizes a network or
filesystem lookup.

## Visible degradation and failures

Warnings are canonical ordered `WarningRecord` values with safe category,
converter, location, and detail fields. They never include raw source bytes,
data-URI payloads, credentials, authorization headers, or backend stack traces.

The shipped PPTX degradation category `optional_extraction_failed` records a
referenced notes part that could not be parsed; the optional notes are omitted
while the legacy Markdown projection remains unchanged. Converter fallback may
also produce stable warnings without exposing backend error text.

Public failure categories are `unsupported`, `malformed`, `limit`, `policy`,
`integrity`, `cancellation`, and `converter`. Callers use typed errors or
`errors.Is`, not string parsing.

## Normalized Markdown and CLI

The shared normalizer uses `\n` line endings, trims trailing whitespace,
collapses excessive blank lines, removes empty headings, and truncates large
inline data URIs unless `KeepDataURIs` is enabled.

The default CLI accepts a local source target, dispatches through the legacy
engine projection, and writes only Markdown to stdout. It adds one terminal
newline when nonempty Markdown lacks one. Success uses empty stderr and exit
status zero. Unsupported, malformed, cancellation, and disabled-remote cases
use empty stdout and a nonzero exit status. `cmd/inkbite/main.go`, the legacy
`Result`, and default policy values are compatibility-frozen surfaces.

## Built-in converter order

`builtins.RegisterDefaultConverters` registers:

1. IPYNB
2. XLSX
3. XLS
4. DOCX
5. PPTX
6. PDF
7. CSV
8. EPUB
9. RSS
10. ZIP
11. HTML
12. text

Registration completes before sharing an engine across goroutines. Conversion
is concurrent-safe after configuration; concurrent registry mutation is not a
supported operation.

## Components and adoption

[INKBITE_COMPONENTS_SPEC.md](INKBITE_COMPONENTS_SPEC.md) records the exact
shipped component-management boundary. It does not promise OCR conversion or a
future provider.

[ADOPTED_COMPONENTS.md](ADOPTED_COMPONENTS.md) distinguishes design
inspiration, copied code, and linked dependencies and records exact revisions,
licenses, local modifications, notices, and distribution obligations.

## Executable conformance evidence

- External legacy and detailed API compatibility:
  [`test/contract/legacy_compatibility_test.go`](test/contract/legacy_compatibility_test.go)
- Go/JSON Schema roundtrip, defaults, documentation vocabulary, and links:
  [`test/contract/ingestion_contract_test.go`](test/contract/ingestion_contract_test.go)
- Exact default CLI output and failure snapshots:
  [`cmd/inkbite/main_test.go`](cmd/inkbite/main_test.go)
- Approved host durability flow:
  [quickstart](kitty-specs/inkbite-ingestion-contract-01M0M3HW/quickstart.md)
