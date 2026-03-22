# Inkbite Tasks

## Purpose

This task list decomposes [INKBITE_SPEC.md](/home/lynn/projects/markitdown/INKBITE_SPEC.md) into implementation work ordered from most foundational to least foundational.

The ordering principle is:

1. build the core engine and contracts first
2. add reusable infrastructure next
3. implement low-risk converters before high-risk converters
4. defer optional and fidelity-heavy work until the MVP is stable

## Tier 0: Project Foundation

### 1. Initialize the Go module and repo layout

- create `go.mod`
- create the package layout from the spec
- add a minimal `cmd/inkbite/main.go`
- add a basic `Makefile` or `justfile` if desired
- add `.gitignore`

Why first:

This is the base for every other task.

### 2. Define the core public types

- implement `StreamInfo`
- implement `Result`
- implement `ConvertOptions`
- define the `Converter` interface
- define typed errors for unsupported format, failed conversion, and invalid source

Why next:

These types shape every converter and every API boundary.

### 3. Build the engine skeleton

- implement `Engine`
- implement converter registration with priority ordering
- implement the conversion dispatch loop
- enforce stream reset semantics between converter attempts
- add the final markdown normalization pass hook

Why next:

Converters should be built on top of a stable engine, not alongside it.

### 4. Build source ingestion and stream handling

- support local paths
- support `[]byte`
- support `io.Reader`
- support `io.ReadSeeker`
- buffer non-seekable readers into memory
- add `file:` URI support
- add `data:` URI support
- add optional `http(s):` support behind a flag

Why next:

This is foundational plumbing shared by all converters.

### 5. Implement MIME and extension detection

- add extension-based MIME guesses
- add content sniffing
- merge explicit hints with inferred hints
- define the exact precedence rules

Why next:

Reliable routing is required before format support can scale.

### 6. Implement markdown normalization

- normalize line endings
- trim trailing spaces
- collapse excessive blank lines
- remove empty headings
- truncate oversized data URIs unless explicitly preserved

Why next:

This keeps converter implementations simpler and output more consistent.

## Tier 1: Test Harness And Developer Workflow

### 7. Set up baseline tests

- add unit tests for `StreamInfo`
- add unit tests for source parsing
- add unit tests for `file:` and `data:` URI parsing
- add unit tests for normalization
- add unit tests for engine dispatch ordering

Why here:

This locks in engine behavior before converters start stacking up.

### 8. Add fixture and golden-test infrastructure

- create a `testdata/` strategy
- add helper functions for loading fixtures
- add semantic assertion helpers
- add golden output support where useful

Why here:

It prevents ad hoc testing per converter later.

### 9. Add CI basics

- run `go test ./...`
- run `go vet ./...`
- optionally add `golangci-lint`

Why here:

This keeps the implementation tight as the codebase grows.

## Tier 2: Lowest-Risk Converters

### 10. Implement the plain text converter

- accept common text MIME types
- detect charset where possible
- decode content safely
- emit normalized text as Markdown

Why first among converters:

It is the simplest end-to-end proof that the engine works.

### 11. Implement the HTML converter

- parse HTML
- remove script and style content
- convert DOM to Markdown
- preserve headings, links, and lists
- handle body-vs-document fallback cleanly

Why next:

Several later converters can reuse HTML-to-Markdown behavior.

### 12. Implement the CSV converter

- parse CSV safely
- pad or truncate ragged rows
- emit Markdown tables

Why next:

It is simple and establishes table rendering conventions.

### 13. Implement JSON and generic XML as text

- route JSON and non-feed XML through text extraction
- avoid overengineering structure in MVP

Why next:

This broadens coverage cheaply.

## Tier 3: Structured Text Formats

### 14. Implement the RSS/Atom converter

- detect RSS vs Atom
- extract title, summary, content, and publish/update timestamps
- pass embedded HTML through the HTML converter path

Why next:

It is a modest jump in structure with limited risk.

### 15. Implement the IPYNB converter

- parse notebook JSON
- emit markdown cells as markdown
- emit code cells in fenced blocks
- optionally include outputs only if text-only and cheap

Why next:

It is deterministic and useful for LLM context.

### 16. Implement the EPUB converter

- read the ZIP container
- parse `container.xml`
- parse OPF metadata
- walk the spine order
- convert XHTML/HTML chapters via the HTML converter

Why next:

It reuses both ZIP and HTML-oriented logic in a contained way.

## Tier 4: Recursive And Tabular Document Formats

### 17. Implement the ZIP converter

- iterate archive entries
- skip unsupported files cleanly
- recurse via the engine into supported files
- render section headers per file
- guard against pathological archives

Why here:

It depends on multiple earlier converters to be useful.

### 18. Implement the XLSX converter

- read workbook sheets
- emit one Markdown section per sheet
- convert sheet rows into tables
- define empty-cell handling and sheet title formatting

Why here:

It is a high-value format with strong Go library support.

### 19. Add optional XLS support

- evaluate library quality
- add legacy spreadsheet extraction if it is low-cost
- leave out if maintenance burden is too high

Why after XLSX:

It is lower value and less foundational.

## Tier 5: Reduced-Scope OOXML Office Formats

### 20. Build shared OOXML helpers

- add ZIP helpers for OOXML packages
- add XML namespace helpers
- add common text extraction utilities
- add shared relationship-path resolution logic where needed

Why first in this tier:

DOCX and PPTX both benefit from shared OOXML infrastructure.

### 21. Implement reduced-scope DOCX extraction

- spike on an existing Go DOCX reader
- if insufficient, parse OOXML directly
- extract paragraphs
- detect headings from paragraph style where possible
- extract hyperlinks
- extract simple tables as a required feature
- optionally infer a title

Explicitly defer:

- equations
- comments
- images
- track changes
- advanced styles

Why before PPTX:

DOCX usually has simpler and more important text structure for ingestion.

### 22. Implement reduced-scope PPTX extraction

- parse presentation and slide order
- extract slide titles
- extract visible text content
- extract speaker notes
- extract simple tables
- define a stable reading order policy

Explicitly defer:

- charts
- images
- smart art
- precise positioning
- grouped-shape fidelity

Why here:

It is useful, but more layout-sensitive than DOCX.

## Tier 6: Reduced-Scope PDF Support

### 23. Define the PDF backend interface

- implement a `PDFExtractor` abstraction
- wire backend selection into `ConvertOptions`
- support backend selection in the CLI

Why first:

This keeps PDF complexity out of the engine.

### 24. Implement a pure-Go PDF extraction backend

- evaluate a pure-Go library
- extract readable page text
- extract best-effort digital tables
- preserve page breaks loosely
- return clean plain Markdown text

Explicitly defer:

- OCR
- form reconstruction
- full layout-aware fidelity for complex tables and columns

Why next:

This gives MVP coverage without external dependencies.

### 25. Harden the pure-Go PDF backend

- improve text extraction quality on representative digital PDFs
- refine table heuristics without adding external dependencies
- document current quality limits clearly

Why next:

This keeps the binary self-contained while raising PDF usefulness.

### 26. Add PDF regression fixtures

- digital PDF fixtures only for MVP
- semantic assertions on key text, headings, table headers, and representative cell values
- no parity expectations for complex table layout or forms

Why next:

PDF quality needs tight regression coverage even at reduced scope.

## Tier 7: CLI Completion And Packaging

### 27. Finish the CLI

- add input path handling
- add stdin handling
- add output file support
- add extension, MIME type, and charset hints
- add `--keep-data-uris`
- add `--http`
- add `--pdf-backend`
- add `--list-formats`
- add version output

Why here:

The CLI should be completed once the engine and most core converters are stable.

### 28. Write end-user docs

- installation instructions
- supported format matrix
- explicit non-goals
- examples for file, stdin, and URI usage
- PDF backend guidance

Why here:

Docs should reflect the actual MVP, not the imagined one.

### 29. Produce a release artifact

- static builds for target platforms
- version stamping
- changelog or release notes

Why here:

Packaging is the last foundational step before broader use.

## Tier 8: Hardening

### 30. Add malformed-input regression tests

- broken ZIPs
- invalid HTML
- malformed EPUB
- malformed OOXML documents
- corrupted PDFs

Why here:

Robustness matters more once broad format coverage exists.

### 31. Add performance and memory guardrails

- benchmark large text files
- benchmark ZIP recursion
- benchmark XLSX extraction
- benchmark PDF extraction
- add limits where necessary

Why here:

Optimization is more meaningful after the shape of the code is stable.

### 32. Improve converter error reporting

- include converter name in failures
- distinguish unsupported vs failed conversion
- surface useful CLI messages without leaking internals

Why here:

This becomes more valuable as the number of converters grows.

## Tier 9: Optional Post-MVP Work

### 33. Add image metadata extraction

- extract lightweight metadata only if cheap and stable

### 34. Add special URL handlers

- Wikipedia
- YouTube
- Bing SERP

### 35. Add Azure Document Intelligence integration

- keep remote extraction out of scope for the core self-contained binary
- document why this remains deferred

### 36. Design a plugin story

- explicit registration
- in-process extension points only
- no subprocess or external-helper requirement in the core product

## Recommended First Execution Slice

If we want the smallest meaningful implementation slice, build this subset first:

1. tasks 1 through 8
2. task 10
3. task 11
4. task 12
5. task 14
6. task 15

That gives us a working engine with real end-to-end value before touching the harder document formats.

## Recommended MVP Completion Slice

The first MVP should include:

- tasks 1 through 18
- task 20
- task 21
- task 22
- task 23
- task 24
- task 26
- task 27
- task 28

Optional for MVP if time permits:

- task 19
- task 25

## Sequencing Notes

- do not start DOCX or PPTX before shared OOXML helpers are in place
- do not start PDF implementation before the backend interface exists
- do not build plugin infrastructure into the MVP
- prefer semantic fixture assertions over exact-output goldens for complex formats
- keep the reduced-scope promise visible throughout implementation
