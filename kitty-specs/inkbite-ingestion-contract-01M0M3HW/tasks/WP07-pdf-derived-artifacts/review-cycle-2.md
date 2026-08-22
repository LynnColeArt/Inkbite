---
affected_files:
  - ingestion.go
  - converters/pdf/artifact_limits_test.go
cycle_number: 2
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: GOTOOLCHAIN=go1.26.6 go test -overlay=<probe-overlay.json> . -run TestWP07ReferenceProbeRejectsNonLocalContinuation -count=1
reviewed_at: '2026-08-22T13:49:00Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP07
---

# WP07 Review Cycle 2 — Changes Required

## Blocking issue — artifact ordinal rewriting accepts URI continuations that do not resolve to the exact artifact ID

The cycle-one canonical-ID mismatch is corrected: raw PDF extraction index is retained, the engine assigns IDs only after final canonical ordering, and it maps each raw index to that final ID. However, `rewriteDetailedArtifactReferences` accepts a 15-byte `artifact-NNNNNN` prefix whenever the next byte is not alphanumeric, `_`, or `-` (`ingestion.go:376-410`). That boundary predicate admits URI syntax that continues the reference beyond the artifact ID.

An independent disposable public-`Ingest` overlay registered one real `DetailedConverter` artifact and supplied these Markdown destinations:

- `inkbite-artifact:artifact-000001/extra`
- `inkbite-artifact:artifact-000001?query`
- `inkbite-artifact:artifact-000001#fragment`
- `inkbite-artifact:artifact-000001%2fextra`
- `inkbite-artifact:artifact-000001:extra`

Every case returned a successful, self-verified envelope. Rewriting changed the 15-byte prefix but retained the suffix, so the complete local URI was not exactly `inkbite-artifact:<final-artifact-id>` and therefore did not resolve to the artifact identified by the prefix. The retained hardening table covers short IDs, wrong prefixes, an alphanumeric continuation, zero, and out-of-range values, but not path, query, fragment, percent-encoded, or colon continuations.

This violates FR-004/FR-006 and the contract rule that a Markdown derivative reference is exactly `inkbite-artifact:<artifact-id>` and resolves without another authority.

Required remediation:

1. Admit only an exact local artifact-reference token. Reject path, query, fragment, percent-encoded, authority-like, colon, backslash, and other URI continuation forms instead of rewriting a valid prefix within them.
2. Add public `Engine.Ingest` regressions for the five cases above plus benign exact-reference boundaries used by generated PDF Markdown. Each invalid case must return `ErrIntegrityFailure` and a zero envelope.
3. Preserve legacy literal behavior: `Convert`, direct legacy PDF conversion, and default CLI output must not interpret or rewrite `inkbite-artifact:` text.

## Cycle-one correction evidence

- Historical red `2c00adb` was reproduced from an isolated archive: `TestPDFDetailedReferencesResolveAfterCanonicalEnvelopeOrdering` failed because visible record 0 disagreed with `artifact-000001`.
- Exact green `0d560dc` passed the mixed JPEG/PNG and repeated-identical-byte public ingestion tests.
- Exact hardening `840b419` passed the retained malformed/short/alphanumeric-continuation/zero/out-of-range and legacy-literal test.
- The mixed fixture maps every generated reference to the final artifact's MIME, page, object, dimensions, bits/component, byte count, occurrence, and relation target. The swapped-reference mutation remains structurally valid but is caught by the semantic fidelity assertion.
- The engine is now the sole final artifact-ID authority; the PDF converter emits extraction-order provisional ordinals and no longer implements a competing canonical comparator.

## Preserved cycle-one evidence

- Legacy `Convert`, `KeepDataURIs=false/true`, and CLI behavior remain isolated from detailed rewriting.
- Artifact count/item/aggregate and primary limits, cancellation, visible optional-extraction degradation, safe canonical attributes, byte ownership, mutation verification, 100-run determinism, and concurrent detailed/legacy conversion remain covered.
- No PDF path introduces network, OCR, model, component download, subprocess, or dependency authority.
- Cycle-two product scope is exactly `converters/pdf/pdf.go`, `converters/pdf/detailed_test.go`, `converters/pdf/artifact_limits_test.go`, and the authorized shared `ingestion.go`. The aggregate WP product scope remains the four owned PDF files plus the four authorized engine/integration files. Coordination history is separate residue.
- Public model/schema/contract, module/dependency, license, CLI, and other frozen surfaces are unchanged; cycle-two public API delta is empty.

## Gate record — Go 1.26.6

- PASS: PDF `-count=20`, PDF race `-count=5`, and cycle-two focused cases `-count=100`.
- PASS: uncached full normal and race suites.
- PASS: vet, staticcheck, build, module verification, gofmt/diff checks, CLI `-count=20`, and Windows/Darwin PDF cross-compilation.
- PASS: `govulncheck ./...` reports zero reachable vulnerabilities.
- PASS: PDF raw coverage is 81.5%; the handoff records fixed-base changed-production coverage `1391/1530 = 90.915033%`.
- FAIL: disposable continuation probe above; all five invalid full-reference forms sealed successfully.

## WP anti-pattern checklist

1. Dead code — PASS. Detailed PDF conversion and engine reference resolution have live production callers.
2. Synthetic fixture — PASS. Mixed formats, identical bytes, and reference mapping execute the real PDF converter through public `Ingest`.
3. Silent empty return — PASS. Optional extraction degradation remains warning-visible; terminal failures return no envelope.
4. FR coverage — FAIL. FR-004/FR-006 lack fail-closed coverage for non-exact local reference continuations.
5. Frozen surface — PASS. No frozen public contract/model/module/license/CLI file changed.
6. Locked decision — FAIL. A rewritten prefix can remain inside a different, non-resolving local URI.
7. Shared-file ownership — PASS. `ingestion.go` and the earlier engine/integration crossings are explicitly authorized and recorded.
8. Production fragility — PASS. No panic/bare-raise or transient race path was introduced.

## Re-review acceptance

Retain the corrected final-ID mapping and add deterministic fail-closed grammar tests showing exact generated PDF references succeed while every continued URI form returns a typed integrity failure with a zero envelope. Re-run the present fidelity, legacy, race, coverage, API, and frozen-surface gates unchanged.
