# Inkbite Spec

## Purpose

Build Inkbite, a Go implementation optimized for extracting useful surface context as Markdown.

This is not a parity rewrite of the upstream Python package. The Go port should prioritize:

- fast, deterministic extraction
- simple deployment
- useful context for LLM ingestion and indexing
- graceful degradation on complex documents

The Go port should not spend early effort on high-fidelity layout reproduction, OCR, or niche document edge cases.

## Product Definition

### Working Definition

Inkbite is a stream-based document-to-Markdown extractor with:

- a Go library API
- a CLI
- pluggable format converters
- support for local files, readers, and selected URI schemes

The output goal is readable, normalized Markdown that preserves major structure:

- titles and headings
- paragraphs
- lists
- links
- tables, with DOCX and PDF table extraction treated as first-class targets
- section boundaries

### Explicit Non-Goals For MVP

- exact output parity with Python MarkItDown
- OCR
- PDF layout reconstruction
- DOCX comments/equations/track-changes support
- PowerPoint chart/image caption intelligence
- Outlook `.msg`
- audio transcription
- multimodal image captioning
- plugin loading via native Go `plugin`
- Azure Document Intelligence integration

## Success Criteria

The MVP is successful if it can:

1. Convert common documents into useful Markdown for retrieval or prompt context.
2. Handle stdin, local paths, `file:`, `data:`, and optional `http(s):` inputs through one API.
3. Produce deterministic normalized output across runs.
4. Avoid crashes on unsupported or malformed files and return clear errors.
5. Cover the bulk of common ingest use cases with a single static binary and no required external helpers.

## Scope

### MVP Format Matrix

| Format | MVP Support | Notes |
| --- | --- | --- |
| Plain text | Yes | Preserve text, normalize line endings |
| HTML | Yes | Convert DOM to Markdown |
| CSV | Yes | Markdown table |
| JSON/XML | Yes | Treat as text unless RSS/Atom |
| RSS/Atom | Yes | Extract feed and entry content |
| IPYNB | Yes | Cells to Markdown/code fences |
| ZIP | Yes | Recurse into supported files |
| EPUB | Yes | Metadata plus spine content |
| XLSX | Yes | Sheets to Markdown tables |
| XLS | Maybe | Optional, lower priority |
| DOCX | Yes, reduced | Headings, paragraphs, links, and basic tables |
| PPTX | Yes, reduced | Slide titles, text, notes, simple tables |
| PDF | Yes, reduced | Surface text plus best-effort digital table extraction |
| Images | Maybe | Metadata only, if cheap |
| Audio | No | Defer |
| Outlook `.msg` | No | Defer |
| YouTube URL | No | Defer for MVP |
| Wikipedia/Bing special handlers | No | Defer for MVP |

### Format Quality Bar

For DOCX, PPTX, and PDF, "supported" means:

- extract enough text to be useful as context
- preserve simple tables when they can be detected reliably
- preserve obvious section boundaries when possible
- return plain Markdown, even if some structure is lost
- prefer incomplete-but-readable output over brittle heuristics

## Architecture

### Top-Level Design

The system should mirror the clean parts of upstream MarkItDown:

- a dispatcher that accepts a source and produces `Result`
- a registry of converters ordered by priority
- a `StreamInfo` struct carrying type hints
- a small normalization pipeline after conversion

### Proposed Package Layout

```text
cmd/inkbite/
    main.go

converters/
    text/
    html/
    csv/
    rss/
    ipynb/
    zip/
    epub/
    xlsx/
    xls/
    docx/
    pptx/
    pdf/

internal/
    fetch/
    markdown/
    normalize/
    sniff/
    ooxml/
    testdata/

engine.go
errors.go
options.go
result.go
source.go
stream_info.go
```

### Public API

```go
type StreamInfo struct {
    MIMEType  string
    Extension string
    Charset   string
    Filename  string
    LocalPath string
    URL       string
}

type Result struct {
    Markdown string
    Title    string
}

type ConvertOptions struct {
    KeepDataURIs bool
    EnableHTTP   bool
    PDFBackend   string
}

type Converter interface {
    Name() string
    Priority() float64
    Accepts(ctx context.Context, r io.ReadSeeker, info StreamInfo, opts ConvertOptions) bool
    Convert(ctx context.Context, r io.ReadSeeker, info StreamInfo, opts ConvertOptions) (Result, error)
}

type Engine struct {
    // registry and shared dependencies
}

func New(opts ...Option) *Engine
func (e *Engine) Convert(ctx context.Context, src any, info *StreamInfo, opts ConvertOptions) (Result, error)
func (e *Engine) ConvertPath(ctx context.Context, path string, info *StreamInfo, opts ConvertOptions) (Result, error)
func (e *Engine) ConvertReader(ctx context.Context, r io.Reader, info *StreamInfo, opts ConvertOptions) (Result, error)
func (e *Engine) ConvertURI(ctx context.Context, uri string, info *StreamInfo, opts ConvertOptions) (Result, error)
```

### Source Handling

The engine should accept:

- local path string
- `[]byte`
- `io.Reader`
- `io.ReadSeeker`
- `file:` URI
- `data:` URI
- optional `http:` and `https:` URI

If the input stream is not seekable, the engine should buffer it into memory once and pass an `io.ReadSeeker` to converters.

### Type Detection

Detection order:

1. user-provided `StreamInfo`
2. extension-derived MIME guess
3. content sniffing
4. converter-specific checks

Use content sniffing to improve routing, but do not overfit routing logic around brittle heuristics.

## Converter Strategy

### Core Rules

- converters should be independent and deterministic
- `Accepts` must not consume the stream
- converters may assume they receive the stream from offset zero
- the engine resets the stream between attempts
- converters should return typed errors for unsupported vs failed conversion

### Markdown Normalization

Apply a final shared normalization pass:

- normalize line endings to `\n`
- trim trailing spaces
- collapse 3+ blank lines to 2
- remove empty heading lines
- truncate huge `data:` URIs unless `KeepDataURIs` is true

## Dependency Plan

Use stable, focused libraries where they clearly reduce effort. Avoid large dependency stacks unless they buy substantial quality.

### Recommended Dependencies

| Area | Candidate | Role |
| --- | --- | --- |
| MIME sniffing | `github.com/gabriel-vasile/mimetype` | content-based type detection |
| HTML to Markdown | `github.com/JohannesKaufmann/html-to-markdown/v2` | HTML conversion |
| XLSX | `github.com/qax-os/excelize` | sheet extraction |
| XLS | `github.com/shakinm/xlsReader` | basic legacy support with formatted cell recovery |
| DOCX | evaluate `github.com/gomutex/godocx` first | surface text extraction |
| PDF | pure Go backend | readable text and best-effort table extraction |

### Dependency Guidance By Format

#### HTML

Use an HTML-to-Markdown library plus a small wrapper layer for:

- data URI truncation
- link cleanup
- heading normalization

#### DOCX

Do not attempt Python's `mammoth` parity in MVP.

Preferred order:

1. spike on an existing Go DOCX reader
2. if the library is too limited, parse OOXML directly from the DOCX zip

MVP DOCX extraction targets:

- document title if obvious
- headings based on paragraph style when available
- paragraph text
- hyperlinks
- simple tables as a required feature

Explicitly ignore for MVP:

- comments
- equations
- images
- floating layout
- style-map customization

#### PPTX

Treat PPTX as OOXML, not as a presentation rendering problem.

MVP extraction targets:

- slide boundaries
- slide title
- body text in reading order
- notes text
- simple tables

Ignore for MVP:

- images
- chart reconstruction
- grouped shape layout fidelity
- smart art

#### PDF

PDF is the highest-risk format, so the design should separate backend choice from the engine.

Define:

```go
type PDFExtractor interface {
    Extract(ctx context.Context, r io.ReadSeeker) (string, error)
}
```

MVP strategy:

1. ship a pure Go text extractor backend
2. add best-effort table extraction for digital PDFs
3. keep the extractor self-contained inside the Go binary
4. default to the built-in backend

Do not implement in MVP:

- borderless table heuristics
- form reconstruction
- OCR
- full layout fidelity or page geometry parity

#### ZIP

ZIP should recurse into supported file types and produce sectioned output like:

```md
Content from zip file `foo.zip`

## File: docs/readme.txt
...

## File: reports/q1.xlsx
...
```

## CLI Spec

### Binary Name

Use `inkbite` as the primary binary name.

### Initial Flags

- `-o, --output`: write Markdown to file
- `-x, --extension`: extension hint
- `-m, --mime-type`: MIME hint
- `-c, --charset`: charset hint
- `--keep-data-uris`: keep inline data URIs
- `--http`: allow remote fetches
- `--pdf-backend`: `auto|purego`
- `--list-formats`: print supported formats
- `-v, --version`: print version

### CLI Behavior

- if no filename is given, read binary data from stdin
- if stdout encoding is limited, replace invalid runes rather than failing
- return non-zero exit code on conversion failure

## Testing Strategy

### Test Philosophy

Do not test for exact parity with Python output. Test for semantic usefulness and structural presence.

### Test Layers

1. unit tests for source parsing, sniffing, normalization, and URI handling
2. converter tests using small fixtures per format
3. golden tests for representative end-to-end files
4. regression tests for panic-proofing malformed inputs

### Fixture Reuse

Reuse a subset of upstream MarkItDown test files where helpful, but relax assertions to match the reduced scope.

Good candidates:

- simple DOCX
- simple PDF
- simple PDF tables
- XLSX
- EPUB
- HTML
- IPYNB
- ZIP

Avoid importing Python-only fidelity expectations such as:

- DOCX comment preservation
- PDF table layout parity
- PowerPoint chart rendering

### Acceptance Assertions

For reduced-scope formats, assert things like:

- key headings are present
- representative body text is present
- gross section order is preserved
- converter returns non-empty Markdown
- unsupported features do not crash conversion

## Milestones

### Milestone 0: Scaffold

- module setup
- engine and converter interface
- stream handling
- URI parsing
- MIME sniffing
- CLI skeleton

Exit criteria:

- text and HTML conversion work from file, stdin, and `data:` URI

### Milestone 1: Easy Wins

- CSV
- RSS/Atom
- IPYNB
- EPUB
- ZIP
- XLSX

Exit criteria:

- end-to-end ingest works for common text-heavy assets

### Milestone 2: Office Context

- DOCX reduced extractor
- PPTX reduced extractor
- optional XLS

Exit criteria:

- surface text from common office docs is usable for indexing

### Milestone 3: PDF Baseline

- pure Go PDF backend
- best-effort digital table extraction
- backend selection flag

Exit criteria:

- readable text extraction and table preservation on typical digital PDFs

### Milestone 4: Hardening

- performance pass
- malformed file handling
- larger fixture set
- docs and release process

## Risks

### Technical Risks

- PDF table extraction quality may be materially worse than Python on complex files.
- Go DOCX/PPTX libraries may not provide enough reading fidelity, forcing direct OOXML parsing.
- HTML-to-Markdown libraries may need customization to produce stable output.
- Recursive ZIP conversion can become expensive on large archives if limits are not enforced.

### Product Risks

- users may assume parity with Python MarkItDown and be surprised by reduced fidelity
- "supported" can become ambiguous unless docs clearly describe the reduced-scope contract

## Guardrails

- prefer deterministic output over clever heuristics
- prefer readable tables when confidence is high, and readable plain text when it is not
- keep converter contracts small and explicit
- do not add format-specific complexity unless it materially improves context quality
- improve built-in extraction before adding heavy heuristics

## Suggested First Build Order

1. engine, `StreamInfo`, normalization, CLI
2. text and HTML
3. CSV, RSS, IPYNB
4. ZIP, EPUB, XLSX
5. DOCX reduced
6. PPTX reduced
7. PDF reduced

## Open Decisions

1. Should remote HTTP fetching be enabled by default or opt-in?
2. Do we want binary compatibility with Python CLI flags, or just conceptual compatibility?

## Recommendation

Proceed with a Go-native MVP based on reduced-scope extraction.

Do not frame the first implementation as a full port. Frame it as:

"A Go document ingester focused on fast, useful surface-context extraction."
