---
name: inkbite
description: Convert files and supported URIs to Markdown with Inkbite. Use when Codex needs to run the Inkbite CLI, integrate the Go library, choose source hints or safety flags, inspect supported formats, or explain current conversion limits for PDF, DOCX, PPTX, XLS, XLSX, EPUB, ZIP, RSS, HTML, IPYNB, CSV, and text-heavy inputs.
---

# Inkbite

Use Inkbite for document-to-Markdown extraction when readable structure matters more than visual fidelity. Prefer it for local files, `file:` URIs, `data:` URIs, and explicitly allowed `http(s)` sources.

## Choose the Entry Point

- Use the CLI for one-off conversions, smoke tests, fixture inspection, and user-facing examples.
- Use the Go API when editing Go applications that should embed Inkbite directly.
- Use `go run ./cmd/inkbite` inside the repository if the binary is not already installed.
- Use the installed `inkbite` binary outside the repository when available.

## Run the CLI

- List formats: `go run ./cmd/inkbite --list-formats`
- Convert a local file: `go run ./cmd/inkbite ./report.pdf`
- Convert from stdin: `cat notes.html | go run ./cmd/inkbite`
- Write output to a file: `go run ./cmd/inkbite -o output.md ./paper.docx`
- Provide type hints: `go run ./cmd/inkbite --extension .xml --mime-type text/xml --charset utf-8 ./sample.dat`
- Allow remote retrieval only when explicit: `go run ./cmd/inkbite --http https://example.org/feed.xml`

Prefer local paths or `file:` URIs first. Treat remote retrieval as opt-in, because `http(s)` is disabled by default.

## Use the Go API

Register the built-in converters, then call the engine on a path, URI, bytes, or reader.

```go
engine := inkbite.New()
builtins.RegisterDefaultConverters(engine)

result, err := engine.Convert(ctx, "./document.pdf", nil, inkbite.ConvertOptions{})
```

Pass `ConvertOptions` intentionally:

- Set `EnableHTTP: true` only for explicit remote fetches.
- Set `KeepDataURIs: true` only when inline data URIs should survive normalization.
- Set `PDFBackend` to `auto` or `purego`. Do not assume any external PDF backend exists.

## Know the Current Format Scope

Treat the current built-in set as:

- Implemented: `ipynb`, `docx`, `pptx`, `pdf`, `xlsx`, `xls`, `csv`, `epub`, `rss`, `zip`, `html`, `text`
- Routed through text handling unless specialized elsewhere: JSON and generic XML

Expect these important limits:

- `pdf`: pure-Go extraction, readable text, and best-effort table heuristics; no OCR or full layout reconstruction
- `docx`: headings, paragraphs, links, and simple tables; not comments, equations, or tracked changes
- `pptx`: slide titles, body text, notes, simple tables, and hyperlinks; not chart intelligence or image understanding
- `xls`: basic legacy workbook extraction with formatted dates and numerics; formula handling remains limited
- `zip`: recursive conversion of supported entries with depth and size guardrails

## Handle Sources Deliberately

- Prefer a local path for ordinary file conversion.
- Use `file:` URIs when the caller already has URI-shaped input.
- Use `data:` URIs for inline content; expect normalization to truncate very large inline payloads unless `KeepDataURIs` is true.
- Use `http(s)` only when the user explicitly wants remote fetching and accepts the network boundary.

## Handle Failures

- Check `--list-formats` when format routing is unclear.
- Add `--extension`, `--mime-type`, or `--charset` hints when sniffing is ambiguous.
- Expect malformed packages and malformed PDFs to fail clearly rather than fall back to external tools.
- Treat unsupported-format errors as a signal to adjust hints, input shape, or converter coverage, not to shell out to external binaries.

## Validate Changes in This Repository

When modifying Inkbite itself, run:

- `go test ./...`
- `go vet ./...`
- `make build`

Add or update fixture-backed tests under `converters/*/testdata` when changing converter behavior.
