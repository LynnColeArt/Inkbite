# Quickstart: Retain a Verified Inkbite Envelope

This guide shows the intended host boundary for `inkbite.ingestion/v1`. It is a design contract for this mission; implementation arrives through the mission work packages.

## 1. Configure the engine once

```go
engine := inkbite.New()
builtins.RegisterDefaultConverters(engine)
```

Complete converter registration before sharing the engine across goroutines.

## 2. Ingest with an explicit policy

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
```

The returned source and artifact byte slices are owned by the envelope. Mutating `sourceBytes` after return does not change them.

## 3. Verify before retention

```go
report := inkbite.VerifyEnvelope(envelope)
if !report.Valid {
    return fmt.Errorf("inkbite envelope failed verification: %v", report.Findings)
}
```

Verification performs no I/O or conversion. A matching digest proves exact bytes, not trusted authorship or execution authority.

## 4. Retain atomically under host authority

Persist these values together:

- `envelope.Source.Bytes` under `envelope.Source.Identity`;
- `envelope.Primary.Bytes` under `envelope.Primary.Identity`;
- each derived artifact's exact bytes and identity;
- the envelope metadata and provenance; and
- the host's own retention receipt binding those objects.

Only after the host reloads and re-verifies the retained values should it remove disposable converter or worker state. Inkbite does not grant cleanup or select the durable store.

## 5. Resolve derivatives without network access

Detailed PDF Markdown may contain a reference such as:

```markdown
![PDF image page 1 Im1](inkbite-artifact:artifact-000001)
```

Resolve `artifact-000001` only against the same envelope's artifact collection. The scheme never authorizes HTTP, filesystem, or component access.

## Compatibility

Existing code remains valid:

```go
result, err := engine.Convert(ctx, sourceBytes, nil, inkbite.ConvertOptions{})
if err != nil {
    return err
}
fmt.Println(result.Markdown)
```

The CLI continues to print Markdown by default. This mission does not add implicit artifact export, remote access, OCR, captioning, or inference.

## Expected failure categories

Callers use `errors.Is` or typed findings for:

- invalid or unsupported source;
- malformed document;
- source, expansion, output, or artifact limit;
- remote disabled or destination denied;
- optional component unavailable or unauthorized;
- integrity failure; and
- cancellation or deadline.

No category returns a successful partial envelope.
