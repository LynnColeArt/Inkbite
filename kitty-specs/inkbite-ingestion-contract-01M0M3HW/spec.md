# Mission Specification: Inkbite Ingestion Contract

**Mission Branch**: `feat/inkbite-ingestion-contract`
**Created**: 2026-08-22
**Status**: Draft
**Input**: Establish a reproducible, bounded rich-document ingestion contract that Nano Kitty can compose without hidden inference, downloads, or network authority.

## Intent Summary

- **Primary actor**: a host application such as Nano Kitty that needs to ingest an untrusted document and retain its useful outputs.
- **Trigger**: the host submits a bounded local, in-memory, data-URI, or explicitly authorized remote source.
- **Observable outcome**: Inkbite returns normalized Markdown, exact source identity, separately retainable derived artifacts, and deterministic conversion provenance.
- **Invariant**: the host can retain the original source and every returned derivative before disposable execution state is removed, then verify each retained byte sequence by digest without rerunning conversion.
- **Main exception path**: malformed, unsupported, oversized, recursively expansive, or unauthorized remote input fails clearly and yields no success envelope or implicit external side effect.

## User Scenarios & Testing

### User Story 1 - Reproducible ingestion envelope (Priority: P1)

As a host application, I want one successful conversion to describe both the normalized content and the exact source from which it came so that I can persist and later verify the result without depending on disposable converter state.

**Why this priority**: A trustworthy source-to-output binding is the minimum useful integration boundary. Without it, a host cannot prove what was ingested or safely reuse the output.

**Independent Test**: Convert the same supported byte sequence through byte, reader, and local-file entry points and verify that all runs return the same normalized content, source digest, primary-artifact digest, winning converter identity, and canonical provenance.

**Acceptance Scenarios**:

1. **Given** identical supported source bytes with equivalent type hints, **when** they are ingested repeatedly, **then** the canonical result and every content digest are identical.
2. **Given** a successful result, **when** the host independently hashes the returned source and primary Markdown bytes, **then** those hashes equal the declared identities.
3. **Given** the original source path or reader is no longer available, **when** the host inspects the successful envelope, **then** it still has the exact source bytes and sufficient metadata to retain and verify them.

---

### User Story 2 - Retain rich derived artifacts (Priority: P2)

As a host application, I want embedded or generated media returned as distinct artifacts rather than hidden inside presentation text so that I can store, inspect, transform, or omit each artifact deliberately.

**Why this priority**: Nano Kitty's rich-media work requires first-class byte artifacts with stable identity. Inline-only data loses provenance and makes retention policy difficult to enforce.

**Independent Test**: Ingest a deterministic PDF containing an image object and verify that the normalized Markdown and extracted image are distinct, ordered artifacts with exact media types, roles, relationships, byte lengths, and digests.

**Acceptance Scenarios**:

1. **Given** a supported document with extractable embedded media, **when** conversion succeeds, **then** each derivative is returned once with stable ordering and independently verifiable bytes.
2. **Given** a derivative referenced by normalized Markdown, **when** the host resolves that reference, **then** it maps unambiguously to one artifact in the same result.
3. **Given** a converter that emits no derivatives, **when** conversion succeeds, **then** the artifact collection is empty rather than populated with synthetic placeholders.

---

### User Story 3 - Apply one explicit ingestion policy (Priority: P3)

As a security-conscious host, I want the same limits and remote-access policy applied before and during conversion so that local files, readers, data URIs, remote responses, and nested containers cannot bypass resource or authority boundaries.

**Why this priority**: Inkbite processes untrusted and recursively structured documents. Limits that apply only to HTTP or only to one archive converter leave equivalent inputs with different trust semantics.

**Independent Test**: Run a table of local, in-memory, data-URI, remote, archive, and document-container inputs immediately below and above each configured boundary and verify that every over-limit or unauthorized case fails with a stable category and no success envelope.

**Acceptance Scenarios**:

1. **Given** remote access is not explicitly enabled, **when** an HTTP or HTTPS source is submitted, **then** no request is issued and the result is a remote-disabled error.
2. **Given** a source or expanded container exceeds a configured limit, **when** it is ingested, **then** conversion stops at that boundary with a limit error and no partial success result.
3. **Given** a remote request redirects or resolves to a disallowed address class, **when** policy evaluation occurs, **then** the request is denied before response bytes are trusted.
4. **Given** optional OCR, vision, or another installed component exists, **when** it was not explicitly selected for this conversion, **then** it is not invoked.

---

### User Story 4 - Adopt without breaking existing callers (Priority: P4)

As an existing Inkbite user, I want the richer contract to preserve the current conversion and CLI experience so that applications can adopt provenance and artifacts incrementally.

**Why this priority**: The library and CLI already have users and format coverage. A foundation contract should extend that surface rather than force unrelated migrations.

**Independent Test**: Compile and run the existing public API and CLI tests unchanged, then add a new consumer that reads the richer envelope without a second conversion path.

**Acceptance Scenarios**:

1. **Given** a caller that only reads `Result.Markdown`, `Result.Title`, or `TextContent`, **when** it upgrades, **then** it continues to compile and observe the same normalized text semantics.
2. **Given** the default CLI invocation, **when** conversion succeeds, **then** stdout remains normalized Markdown and no binary artifact is emitted implicitly.
3. **Given** a custom converter implementing the existing converter contract, **when** it returns a legacy-shaped result, **then** an additive detailed-ingestion operation supplies source identity and engine-owned provenance without requiring that converter to re-read the source.

### Edge Cases

- Empty but otherwise valid source bytes have a stable digest but do not fabricate extracted content.
- User hints that contradict detected content are recorded and handled deterministically; they cannot change the digest of the actual source bytes.
- A reader that returns short reads, a terminal error, or a cancellation never yields a successful partial envelope.
- Duplicate embedded artifacts are either retained as distinct source occurrences with distinct relationship metadata or intentionally deduplicated by an explicit, deterministic rule; they are never silently overwritten.
- Artifact names and source locators containing path traversal, control characters, credentials, fragments, or data-URI payloads are sanitized or omitted from public metadata without changing byte identities.
- Unsupported archive members do not weaken aggregate entry, depth, or expanded-byte accounting.
- Declared archive sizes that disagree with bytes actually read are rejected when they cross policy bounds.
- Redirect chains are bounded and each destination is re-evaluated under remote policy.
- Converter fallback records only safe converter identities and error categories; it does not expose source content or secrets in error strings.
- Context cancellation before, during, or after dispatch returns cancellation and no success envelope.

## Requirements

### Functional Requirements

| ID | Title | User Story | Priority | Status |
|----|-------|------------|----------|--------|
| FR-001 | Canonical ingestion envelope | As a host, I want an additive detailed-ingestion operation to return one versioned envelope containing normalized content, source identity, artifacts, and provenance without changing the existing `Result` shape. | High | Open |
| FR-002 | Exact source artifact | As a host, I want the exact acquired source bytes, byte length, media information, and a SHA-256 identity returned so I can retain and verify the original input. | High | Open |
| FR-003 | Primary normalized artifact | As a host, I want normalized Markdown represented as a primary artifact with canonical UTF-8 bytes, media type, role, length, and SHA-256 identity. | High | Open |
| FR-004 | Derived artifact inventory | As a host, I want each extracted derivative returned with bytes, stable role, media type, safe logical name, source relationship, length, and SHA-256 identity. | High | Open |
| FR-005 | Deterministic provenance | As an auditor, I want provenance to identify the contract version, winning converter, selected backend or component, canonical stream metadata and whether each value came from the source, caller, or sniffing, effective policy, source identity, and output identities without nondeterministic fields. | High | Open |
| FR-006 | Stable ordering and references | As a host, I want artifacts and their references ordered and resolved deterministically so serialization and digest comparison are reproducible. | High | Open |
| FR-007 | Uniform source limits | As an operator, I want a single explicit policy to bound acquired bytes for paths, readers, byte slices, data URIs, and remote responses before converter dispatch. | High | Open |
| FR-008 | Container limits | As an operator, I want policy to bound recursion depth, entry count, per-entry bytes, total expanded bytes, and expansion ratio for ZIP and ZIP-based document containers. | High | Open |
| FR-009 | Explicit remote authority | As an operator, I want HTTP(S) disabled by default and, when enabled, governed by scheme, redirect, address-class, response-size, and caller-supplied transport policy. | High | Open |
| FR-010 | Explicit optional components | As an operator, I want OCR, vision, model inference, and component downloads to remain inactive unless the conversion request explicitly selects an already installed component. | High | Open |
| FR-011 | Typed failure outcome | As a host, I want unsupported, malformed, limit, policy, integrity, cancellation, and converter-failure outcomes distinguishable without parsing backend error text. | High | Open |
| FR-012 | Backward-compatible library use | As an existing caller, I want the current `Result` text fields, converter interface, and conversion entry points to remain source compatible. | High | Open |
| FR-013 | Backward-compatible CLI use | As a CLI user, I want the default command to retain Markdown stdout and existing flags, with no implicit binary or metadata output. | Medium | Open |
| FR-014 | Public verification | As a host, I want a public verification operation that recomputes source and artifact identities and rejects malformed, missing, duplicate, or mismatched envelope data without invoking conversion or external services. | High | Open |
| FR-015 | Bounded outputs | As an operator, I want policy to bound primary output bytes, artifact count, per-artifact bytes, and aggregate artifact bytes before a successful envelope is returned. | High | Open |
| FR-016 | Visible degradation | As a host, I want skipped unsupported members, failed optional extraction, and intentional deduplication represented by stable warnings or typed failure rather than silently omitted from an authoritative result. | High | Open |
| FR-017 | Concurrent conversion contract | As a host, I want one fully configured engine usable by concurrent conversions without request data, artifacts, policy, or provenance crossing between calls. | Medium | Open |

### Non-Functional Requirements

| ID | Title | Requirement | Category | Priority | Status |
|----|-------|-------------|----------|----------|--------|
| NFR-001 | Reproducibility | For a fixed source, canonical hints, policy, converter set, and component versions, 100 repeated conversions must produce byte-identical normalized content, artifact ordering, identities, and canonical provenance. | Reliability | High | Open |
| NFR-002 | Integrity coverage | Automated tests must independently recompute 100% of source and artifact SHA-256 identities and must detect a one-byte mutation in every returned byte-bearing object. | Security | High | Open |
| NFR-003 | Bounded acquisition | The default maximum acquired source size is 32 MiB for every source kind; a boundary test must pass at the limit and fail at limit plus one byte without converter invocation. | Security | High | Open |
| NFR-004 | Bounded expansion | Default container policy must permit no more than 256 entries, 8 MiB per entry, 32 MiB total expanded bytes, four recursive container levels, and an explicitly tested expansion-ratio ceiling. | Security | High | Open |
| NFR-005 | Remote fail-closed behavior | With remote access disabled, 100% of HTTP(S) cases issue zero transport calls; with it enabled, loopback, private, link-local, unspecified, and redirect-to-disallowed destinations are denied in IPv4 and IPv6 tests. | Security | High | Open |
| NFR-006 | No hidden inference | Tests with counting fakes must show zero optional-component, model, and download calls unless explicitly selected, including on fallback and error paths. | Privacy | High | Open |
| NFR-007 | Cancellation | Cancellation must return a typed failure and no successful envelope. Work at a cooperative interruption boundary, including deterministic blocking reader, remote, and converter fixtures, must terminate and join all Inkbite-owned workers within one second. An arbitrary caller-owned non-cooperative `io.Reader` or `io.ReadSeeker` remains synchronously joined until its in-flight method returns; cancellation is then observed at the next checkpoint and cannot yield partial success. | Reliability | High | Open |
| NFR-008 | Compatibility | All pre-mission public API, converter, CLI, and format tests must pass unchanged; a compile-time compatibility fixture must cover legacy `Result` and custom-converter use. | Compatibility | High | Open |
| NFR-009 | Race safety | The full repository race suite and 100-run concurrent conversion stress test must complete with no race reports, artifact aliasing, or cross-request policy leakage. | Reliability | High | Open |
| NFR-010 | Changed-code coverage | Changed production statements must have at least 80.0% unrounded fixed-base coverage, with security and verification branches covered by mutation or deletion evidence. | Quality | High | Open |
| NFR-011 | Sensitive metadata hygiene | Errors and public metadata must contain zero raw data-URI payloads, URL credentials, authorization headers, source bytes, component grants, or backend stack traces in sentinel tests. | Privacy | High | Open |
| NFR-012 | Portable bytes | Byte-bearing fixtures and artifact results must remain identical across Linux, macOS, and Windows checkouts and test runs. | Portability | Medium | Open |
| NFR-013 | Bounded results | Default policy must cap primary normalized output at 32 MiB, artifact count at 256, each derivative at 8 MiB, and aggregate derivative bytes at 32 MiB; each boundary must pass at the limit and fail at limit plus one. | Security | High | Open |

### Constraints

| ID | Title | Constraint | Category | Priority | Status |
|----|-------|------------|----------|----------|--------|
| C-001 | MIT distribution | New project code remains compatible with Inkbite's MIT license; adopted code or design must be recorded with origin, license, modifications, and attribution obligations. Official release artifacts qualified by this mission are reproducible source-only archives: they contain no linked executable, vendored module tree, or third-party dependency source. A default binary links GPL-3.0-only `xlsReader` and must not be published or represented as MIT-only unless a later, independently reviewed release strategy satisfies the applicable GPL and transitive-license obligations. | Legal | High | Open |
| C-002 | No persistence owner | Inkbite returns bytes and verification metadata but does not choose Nano Kitty's durable storage location, cleanup timing, or retention policy. | Architecture | High | Open |
| C-003 | No automatic network expansion | Conversion may not download models, components, schemas, linked media, or secondary documents unless a distinct caller-granted capability explicitly allows that class of access. | Security | High | Open |
| C-004 | One conversion path | Rich results must extend the existing engine and converter registry; a second privileged ingestion implementation is forbidden. | Architecture | High | Open |
| C-005 | Streaming future compatibility | The contract may buffer within current limits, but identities, artifacts, and policy must not preclude a future streaming implementation. | Architecture | Medium | Open |
| C-006 | Canonical digest | Content identities use `sha256:<lowercase hexadecimal>` over exact bytes; display labels, paths, timestamps, and memory addresses are excluded from content identity. | Technical | High | Open |
| C-007 | Immutable result ownership | Returned byte slices must not alias caller-owned mutable buffers or internal scratch buffers across results. | Technical | High | Open |
| C-008 | Existing format semantics | The mission does not promise new document formats, OCR quality, image captioning, transcription, or high-fidelity layout reconstruction. | Product | Medium | Open |

### Key Entities

- **Ingestion Envelope**: the additive, versioned successful outcome that binds one exact source to normalized content, zero or more derived artifacts, and conversion provenance while leaving the legacy result contract intact.
- **Source Artifact**: an immutable copy of the acquired input with byte length, media information, safe origin metadata, and content identity.
- **Derived Artifact**: independently retainable bytes produced from the source, labeled by role, media type, safe logical name, deterministic relationship, length, and identity.
- **Conversion Provenance**: deterministic evidence describing which contract, converter, backend or explicitly selected component, stream metadata, and policy produced the artifacts.
- **Ingestion Policy**: the caller-visible acquisition, expansion, remote-authority, and optional-component limits applied to one conversion.
- **Failure Category**: a stable public classification that preserves actionable meaning without exposing sensitive backend detail.

## Assumptions and Non-Goals

- Nano Kitty will own durable storage, promotion, recovery, and cleanup; Inkbite's responsibility ends after returning a verified envelope.
- Source bytes are already buffered by the current engine, so exposing an immutable source artifact does not authorize unbounded acquisition.
- The first rich derivative proof uses an embedded PDF image because that capability already exists; the contract is format-neutral.
- Registration may remain a configuration-time operation that is not concurrent with conversion; after configuration, conversions must be safe to run concurrently.
- Existing converter priorities and normalized Markdown are preserved unless a separately reviewed correctness defect requires a change.
- Remote access remains opt-in. This mission hardens its policy but does not create a crawler, browser, URL discovery system, or authenticated connector.
- Optional inference/provider integration, image captioning, OCR activation, and Nano Kitty component orchestration are later missions built on this contract.
- Canonical provenance excludes wall-clock timestamps and host-local absolute paths. Operational logs may record time separately.

## Success Criteria

### Measurable Outcomes

- **SC-001**: A Nano-Kitty-like consumer can convert, independently verify, persist, discard all converter state, and later reload the exact source, Markdown, and PDF image derivative using only the returned envelope.
- **SC-002**: The reproducibility matrix completes 100 identical conversions for representative text, PDF-with-image, office-document, and nested-archive fixtures with byte-identical canonical results.
- **SC-003**: Every source-kind, output, artifact, and container-limit boundary has passing at-limit and failing over-limit tests, and no over-limit case invokes disallowed downstream work or returns a partial envelope.
- **SC-004**: Remote-policy tests prove zero calls while disabled and deny all enumerated private/address-class and redirect bypass cases while preserving an explicitly authorized public fixture path.
- **SC-005**: One-byte mutation, missing-artifact, duplicate-identity, invalid-reference, and cross-envelope substitution tests all fail public verification.
- **SC-006**: Existing API, custom-converter, CLI, and format suites remain green without callers adopting the new fields.
- **SC-007**: Release qualification reports no functional regression, concurrency defect, known reachable vulnerability, licensing violation, portability drift, or quality-threshold failure on the final source-only deliverable, and proves that official publication workflows cannot upload a linked executable or unqualified dependency bundle.
- **SC-008**: The mission ships public contract documentation and an adopted-components record sufficient for a host integrator to implement retention without reading Inkbite internals.
