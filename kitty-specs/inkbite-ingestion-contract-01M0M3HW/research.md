# Research: Inkbite Ingestion Contract

**Mission**: `inkbite-ingestion-contract-01M0M3HW`  
**Date**: 2026-08-22  
**Status**: Complete for planning  
**Evidence register**: [research/evidence-log.csv](research/evidence-log.csv)  
**Source register**: [research/source-register.csv](research/source-register.csv)

## Research Question

How should Inkbite expose exact source identity, normalized content, rich derived artifacts, and conversion provenance to Nano Kitty while preserving existing callers and enforcing one fail-closed resource and authority boundary across every source and container form?

## Baseline Findings

The current public contract is intentionally small: converters return a result containing Markdown and title, and the engine exposes path, reader, URI, and general conversion entry points. That simplicity is valuable and already documented, but it provides no structured source identity, artifact inventory, provenance, warning, or verification surface [E-001].

All source forms are converted into an in-memory seekable reader. HTTP is disabled by default and capped at 32 MiB when enabled, but paths, readers, byte slices, and data URIs are not bounded before allocation [E-002]. Generic ZIP conversion has entry, byte, and recursion limits, while EPUB and OOXML-backed formats have separate readers without the same aggregate policy [E-003, E-004]. Therefore, a policy added only to HTTP or generic ZIP would leave equivalent content with different authority.

The PDF converter already extracts embedded image bytes and metadata. Today it renders them only as base64 data URIs inside Markdown when requested, which proves the bytes exist but does not provide a retainable artifact contract [E-005]. Optional-component documentation already states the desired authority rule: normal conversion must never download or silently activate OCR [E-006].

## Decisions

### R-001 — Add a versioned detailed-ingestion operation

**Decision**: Introduce an additive detailed operation that returns a versioned ingestion envelope. Preserve the existing result structure, converter interface, engine entry points, converter ordering, and default CLI output.

**Rationale**: Adding methods to a public interface breaks third-party implementations. Adding fields to an exported struct may break external unkeyed literals, and adding slice-bearing artifact fields would also make the currently comparable result non-comparable. A parallel privileged implementation would violate the single conversion-path constraint. The detailed operation should therefore orchestrate the existing registry and project legacy text results into a richer engine-owned envelope [E-001, E-010].

**Rejected alternatives**:

- Add required methods to the converter interface: breaks implementers at compile time.
- Add byte/provenance fields directly to the legacy result and rely on callers using keyed literals: not a safe compatibility assumption.
- Build a Nano-specific adapter that bypasses the engine: creates a second policy and converter authority.

### R-002 — Return owned source and artifact bytes

**Decision**: A successful envelope owns an exact source artifact, one primary normalized-content artifact, and zero or more derived artifacts. Every byte-bearing object declares a `sha256:<lowercase hexadecimal>` identity, length, media type, stable role, and deterministic relationship to its source or parent.

**Rationale**: Nano Kitty must be able to retain source and derivative bytes before disposable state is removed. A temporary path or inline-only data URI is not a durable handoff. SHA-256 is a standard content-identity primitive; verification will recompute it over exact bytes rather than trusting metadata. Digest equality proves byte integrity, not origin, authorship, or authority [E-005, E-011].

**Ownership rule**: Public byte slices do not alias caller buffers, converter scratch storage, or another result. Inkbite does not persist them; the host owns durable storage, promotion, and cleanup.

### R-003 — Keep canonical provenance deterministic

**Decision**: Canonical provenance records contract version, exact source identity, canonical type metadata and each value's origin, winning converter, selected backend or explicitly selected component, effective policy, ordered warnings/attempt categories, and ordered output identities. Wall-clock time, memory addresses, absolute local paths, raw data URIs, credentials, and backend stack traces are excluded from canonical identity.

**Rationale**: Reproducibility requires the fields that affect behavior while excluding host-local or nondeterministic observations. Current stream metadata loses whether a value came from source facts, caller hints, or sniffing, so provenance must preserve that distinction [E-002].

### R-004 — Enforce one ingestion policy at common boundaries

**Decision**: Define one immutable per-request policy covering:

- acquired source bytes;
- normalized output bytes;
- artifact count, per-artifact bytes, and aggregate artifact bytes;
- container recursion, entry count, per-entry bytes, total expanded bytes, and expansion ratio;
- remote schemes, redirects, destination address classes, transport, and response bytes;
- explicitly selected optional components; and
- cancellation/deadline behavior.

Acquisition limits apply before converter dispatch. Container accounting is shared by generic ZIP, EPUB, DOCX, PPTX, and XLSX paths. Declared sizes support early denial, while actual decompressed bytes, entry checksum completion, duplicate/colliding names, path form, nesting count, and file type remain authoritative. Output and artifact limits apply before sealing success [E-018].

**Rationale**: Current boundaries are split across source and format packages [E-002, E-003, E-004]. Common ownership closes encoding-based bypasses. Declared archive sizes are useful admission hints but actual bytes read remain authoritative.

### R-005 — Preserve HTTP opt-in and add destination admission

**Decision**: Remote access remains disabled by default. When explicitly enabled, only HTTP(S) is eligible; userinfo is rejected; redirect count is bounded; every initial and redirected destination is re-evaluated; ambient proxies and transparent compression are disabled unless separately authorized; and loopback, private, link-local, unspecified, multicast, and otherwise disallowed IPv4/IPv6 destinations fail closed unless a caller-supplied policy deliberately grants them. Approved DNS answers are pinned at the connection boundary while preserving the original hostname for TLS. URL credentials, sensitive query material, and fragments are excluded from safe metadata and diagnostics.

**Rationale**: A size cap limits response memory but does not prevent server-side request forgery. Go's HTTP client supports redirect and connection-time policy hooks, while IP address classes are normatively defined independently of hostname spelling. `IsPrivate` and `IsGlobalUnicast` are not sufficient access-control predicates [E-012, E-013, E-014, E-015]. A custom transport must not silently disable the engine's admission decision.

### R-006 — Make degradation visible

**Decision**: Unsupported container members, optional derivative failures, fallback attempts, and intentional deduplication are represented by stable warning categories and safe locations, or they fail the conversion according to policy. They are never silently omitted from an otherwise authoritative detailed envelope.

**Rationale**: Generic ZIP currently skips unsupported members and most member failures [E-003]. That is acceptable for a convenience Markdown projection but insufficient when a host treats the result as retained evidence.

### R-007 — Keep optional inference and component authority explicit

**Decision**: The detailed operation performs no download, subprocess, OCR, image captioning, model inference, or linked-resource fetch unless the caller selects an already installed capability and the policy permits it. Any selected component identity and version become provenance.

**Rationale**: This preserves Inkbite's self-contained core and the existing component doctrine [E-006]. Provider inference and rich-media interpretation belong to later Nano Kitty missions built on returned artifacts.

### R-008 — Provide pure public verification

**Decision**: Add a public verification operation that validates envelope version and shape, clones/ownership assumptions where observable, recomputes all byte identities and lengths, checks unique identities and references, and validates canonical ordering. It performs no conversion, network request, component call, or persistence.

**Rationale**: A host needs to validate a deserialized or retained envelope without trusting its fields and without re-entering effectful conversion.

### R-009 — Define the success/failure boundary

**Decision**: A result becomes successful only after source acquisition, converter dispatch, normalization, derivative collection, all policy accounting, provenance construction, and envelope verification complete. Cancellation, policy, limit, integrity, malformed-input, or required-extraction failure returns a typed error and no successful envelope. Cancellation is advisory rather than synchronous, so request completion must also join mission-owned workers before reporting terminal cleanup [E-019].

**Rationale**: This prevents partial authority. Existing typed and `errors.Is`-compatible errors are retained and extended rather than replaced by string parsing [E-007].

### R-010 — Configure, then convert concurrently

**Decision**: Converter registration remains a configuration-time activity. Once configuration is complete, the engine supports concurrent detailed and legacy conversions with request-local source, policy, attempts, warnings, artifacts, and provenance.

**Rationale**: The registry is cloned for dispatch, but explicit concurrency semantics are absent. Freezing configuration before concurrent use keeps the contract small while preventing request leakage [E-001].

### R-011 — Document civic adoption and licensing

**Decision**: Any borrowed component, code, schema, or substantial design records origin, upstream revision/version, license, adopted surface, local modifications, and attribution/notice obligations in an adopted-components file. Project-authored additions remain MIT-compatible.

**Rationale**: SPDX provides machine-readable license identifiers and the OSI records the MIT license as an approved open-source license [E-016, E-017]. This is both license hygiene and the civic-responsibility norm requested for the project.

## Default Policy Rationale

The first contract version retains current tested generic-ZIP values where they are already conservative and makes them uniform:

| Boundary | Default | Reason |
|---|---:|---|
| Acquired source | 32 MiB | Matches current remote cap and bounds all source encodings equally. |
| Primary normalized output | 32 MiB | Prevents successful output amplification beyond the source/expanded budget. |
| Container entries | 256 | Preserves the existing generic ZIP limit. |
| One expanded entry | 8 MiB | Preserves the existing generic ZIP limit. |
| Total expanded bytes | 32 MiB | Preserves the existing generic ZIP limit. |
| Nested container depth | 4 | Preserves the existing generic ZIP limit. |
| Derived artifacts | 256 | Aligns artifact cardinality with entry cardinality. |
| One derivative | 8 MiB | Aligns derivative retention with one expanded entry. |
| Aggregate derivatives | 32 MiB | Aligns retained derivatives with total expansion. |

The expansion-ratio ceiling remains a planning detail that must be selected before implementation and tested against both honest compressed documents and compression-bomb fixtures. It may not be omitted merely because absolute byte limits also exist.

## Evidence-to-Requirement Trace

| Decision | Requirements primarily served |
|---|---|
| R-001 | FR-001, FR-012, FR-013; NFR-008; C-004 |
| R-002 | FR-002, FR-003, FR-004; NFR-002; C-002, C-006, C-007 |
| R-003 | FR-005, FR-006; NFR-001, NFR-011 |
| R-004 | FR-007, FR-008, FR-015; NFR-003, NFR-004, NFR-013 |
| R-005 | FR-009; NFR-005, NFR-011; C-003 |
| R-006 | FR-011, FR-016 |
| R-007 | FR-010; NFR-006; C-003, C-008 |
| R-008 | FR-014; NFR-002 |
| R-009 | FR-011; NFR-007 |
| R-010 | FR-017; NFR-009 |
| R-011 | C-001; SC-008 |

## Risks and Planning Inputs

1. **Compatibility fixtures**: repository tests cannot prove all external unkeyed literals or third-party converters; planning must include a compile fixture that represents both patterns.
2. **Shared archive reader**: ZIP-backed formats currently use different libraries and access patterns. Planning must assign one accounting authority without forcing format converters into a high-coupling abstraction.
3. **Remote name resolution**: destination admission must address multiple DNS answers and redirects without introducing a time-of-check/time-of-use gap. The plan must identify the transport seam that dials only an admitted address.
4. **HTTP byte definition**: planning must state whether retained remote source bytes are content-encoded or decoded representation bytes and configure compression consistently; no implicit transport transformation may make the declared source identity ambiguous.
5. **Non-cooperative dependencies**: context cancellation cannot forcibly stop pure library calls. Planning must distinguish cooperative guarantees from a process-isolation feature, which is out of scope.
6. **Artifact references**: Markdown references need a stable non-network scheme or relation identifier; the plan must choose one canonical representation and golden-test it.
7. **Partial-result policy**: the convenience legacy projection may continue best-effort behavior, but detailed ingestion must surface every degradation. Planning must prevent the legacy projection from becoming a bypass for hosts requesting evidence.
8. **Expansion ratio**: select and justify a concrete default before task finalization.

## Open Questions

No user decision blocks planning. The expansion-ratio number, artifact reference encoding, and detailed-operation naming are technical design choices bounded by this research and must be resolved in `plan.md` before implementation.
