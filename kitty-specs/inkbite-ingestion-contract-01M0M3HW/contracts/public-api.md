# Public API Contract: Detailed Ingestion v1

**Contract ID**: `inkbite.ingestion/v1`
**Compatibility rule**: additive to the existing Inkbite result, converter, engine, and CLI surfaces.

## Operations

### Detailed ingestion

```go
func (e *Engine) Ingest(
    ctx context.Context,
    source any,
    hints *StreamInfo,
    options IngestOptions,
) (IngestionEnvelope, error)
```

Observable rules:

1. The zero-value options materialize the documented default policy.
2. Success owns exact source, primary Markdown, derivative, provenance, and warning values.
3. Failure returns the zero envelope and an `errors.Is`-compatible category.
4. No optional component, remote request, proxy, download, or inference occurs without explicit authority.
5. Existing `Convert*` methods use the same pipeline and project the legacy Markdown/title result.

### Reader cancellation

Reader cancellation is enforced at cooperative boundaries without changing the accepted `io.Reader` or `io.ReadSeeker` surface. A pre-canceled request performs no caller `Seek` or `Read`. When a caller source implements `io.Closer`, Inkbite may call `Close` on cancellation; the one-second termination guarantee applies when that concrete interruption cooperatively unblocks the in-flight method. An arbitrary caller-owned non-cooperative `Read` or `Seek` remains synchronously joined until it returns, after which cancellation is observed at the next checkpoint and no partial source or successful result is returned. Inkbite does not race an arbitrary reader in a detached worker or report terminal completion while an Inkbite-owned worker remains unjoined.

### Pure verification

```go
func VerifyEnvelope(envelope IngestionEnvelope) VerificationReport
```

Verification recomputes identities and lengths, validates version/shape/order/references/policy consistency, and returns ordered typed findings. It performs no I/O, conversion, component call, or persistence.

### Optional converter capability

```go
type DetailedConverter interface {
    Converter
    ConvertDetailed(
        ctx context.Context,
        reader io.ReadSeeker,
        info StreamInfo,
        options ConvertOptions,
        policy IngestionPolicy,
    ) (DetailedConversion, error)
}
```

Converters that implement only the existing interface remain valid. The engine creates the primary artifact and engine-owned provenance around their legacy result. The optional capability may add derivatives, warnings, and bounded safe facts; the engine remains the final validation and identity authority.

## Identity and references

- Identity: `sha256:<64 lowercase hexadecimal characters>`.
- Contract: `inkbite.ingestion/v1`.
- Artifact IDs: deterministic envelope-local values such as `artifact-000001`.
- Markdown derivative reference: `inkbite-artifact:<artifact-id>`.
- Artifact references never authorize a network or filesystem lookup.

## Compatibility fixtures

Release qualification compiles an external package that:

- implements the original `Converter` only;
- constructs the two-field legacy result positionally;
- compares legacy results with `==`;
- uses a legacy result as a map key;
- calls every existing conversion entry point; and
- runs the CLI's default Markdown-output path.

The detailed contract is versioned independently. Future incompatible envelope changes require a new contract version and translation, not mutation of v1 semantics.
