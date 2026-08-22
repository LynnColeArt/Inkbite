---
affected_files:
  - converters/pdf/pdf.go
  - converters/pdf/detailed_test.go
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: GOTOOLCHAIN=go1.26.6 go test ./converters/pdf -run TestWP07ReviewProbeReferenceFollowsFinalCanonicalID -count=1
reviewed_at: '2026-08-22T13:15:43Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP07
---

# WP07 Review Cycle 1 — Changes Required

Reviewer: Reviewer Renata

Reviewed red: `5f8dd1c`

Reviewed final product commits: `c176881`, `766f43c`

## Blocking issue — PDF Markdown reference IDs are assigned before the engine's different canonical sort

`detailedArtifactOrderKey` in `converters/pdf/pdf.go:292-306` orders converter drafts with plain NUL-delimited strings. `appendDetailedImageReferences` at `converters/pdf/pdf.go:314-342` then assumes that order is final and emits positional `artifact-NNNNNN` references. The engine does not preserve that order: `sealIngestionEnvelope` independently sorts with the length-prefixed, relationship-aware `canonicalArtifactOrderKey` and only afterward assigns final IDs (`ingestion.go:279-303`).

Those comparators are not order-equivalent. An independent disposable public-`Ingest` probe supplied the PDF production helpers with a JPEG at page 1/object 1 and a PNG at page 2/object 2. PDF ordering emitted:

- JPEG/page 1 as `inkbite-artifact:artifact-000001`;
- PNG/page 2 as `inkbite-artifact:artifact-000002`.

After engine sealing, the returned envelope contained:

- `artifact-000001` = PNG, occurrence `page-000002/object-000002`;
- `artifact-000002` = JPEG, occurrence `page-000001/object-000001`.

Thus the first Markdown image, visibly described as JPEG/page 1, resolves exactly once but to the wrong derivative. `VerifyEnvelope` still reports the envelope valid because it establishes artifact/relation integrity but does not semantically bind the Markdown description to the referenced artifact. This violates T030, FR-004, FR-006, the plan's PDF reference decision, and the independent-test requirement that a referenced derivative map unambiguously to the intended artifact.

The retained tests do not expose the defect: the public reference test has only one PNG artifact, the helper ordering test stops before engine sealing, and the 100-run test repeats the same single-artifact fixture.

Required remediation:

1. Establish one canonical artifact-order/ID assignment authority. PDF must not predict positional IDs using a comparator that can diverge from engine sealing.
2. Render detailed Markdown references from the engine's final semantic-to-ID mapping, or expose/reuse an order primitive whose equivalence to engine sealing is structural rather than duplicated.
3. Add retained public-`Ingest` regressions with at least JPEG+PNG and reversed extraction order. For every reference occurrence, assert that the referenced final artifact's MIME, page, object, dimensions, and occurrence match the visible Markdown record.
4. Add repeated-identical-byte fixtures at distinct page/object occurrences. Retention as distinct artifacts is acceptable, but every occurrence must keep the correct relationship and reference; if deduplication is introduced, it must preserve all occurrence relationships and emit visible evidence.
5. Mutation-prove that perturbing either converter order or final engine order cannot silently retarget a valid reference.

## Reproduced passing evidence

- Historical red `5f8dd1c` fails for the intended absent PDF detailed-artifact behavior; final `c176881`/`766f43c` passes its authored cases.
- Legacy dispatch is correctly separated: public `Convert` uses legacy `Converter.Convert`, detailed `Ingest` uses `DetailedConverter.ConvertDetailed`, and legacy `KeepDataURIs=false/true` snapshots remain exact.
- Detailed artifacts are cloned with exact capacity, engine-sealed as independently owned bytes, budget-accounted, identity-checked, and one-byte mutations fail verification.
- Page, object, width, height, bits-per-component, and image-mask attributes are canonicalized with converter origin and invalid/duplicate values fail closed.
- Optional image-extraction degradation is visible; typed limit and cancellation failures return zero envelopes. Text-only PDFs have no phantom artifacts or references.
- Repeated distinct occurrences are retained rather than silently deduplicated by the current representation; exact semantic duplicates fail closed in the engine.
- The final aggregate WP diff is exactly the authorized eight files: the four PDF-owned files plus `engine.go`, `ingestion.go`, `ingestion_test.go`, and `conversion_integration_test.go`. Module, license, contract/schema, CLI, and frozen public model files are unchanged.
- Go 1.26.6 gates pass: PDF `-count=20`, PDF race `-count=5`, full normal/race, vet, build, module verification/tidy-diff, formatting, diff check, staticcheck, CLI `-count=20`, and Windows PDF compile.
- `govulncheck ./...` reports zero reachable vulnerabilities. No WP07 diff introduces network, download, OCR, model, subprocess, or dependency authority.
- Raw coverage is 80.4% for `converters/pdf`; the implementer handoff's fixed-base changed-production result is 83.720930%, above the 80.0% gate.
- Clone-ownership, legacy/detailed dispatch, and reference-index mutations each fail their dedicated tests and restore green.
- Final lane product bytes are restored; only the runtime-generated untracked `.spec-kitty/` residue remains.

## WP anti-pattern checklist

1. Dead code — **PASS**: the PDF detailed capability is reached through public `Ingest`.
2. Synthetic fixtures — **FAIL for reference fidelity**: current fixtures exercise real production paths but only one artifact crosses the full PDF-to-engine boundary, so they cannot detect positional-ID retargeting.
3. Silent empty return — **PASS**: optional extraction omission carries a stable warning; terminal policy, integrity, limit, and cancellation paths fail typed.
4. FR coverage — **FAIL**: FR-004 and FR-006 lack a multi-artifact end-to-end reference-to-intended-artifact assertion.
5. Frozen surface — **PASS**.
6. Locked decision — **FAIL**: the emitted reference may resolve to a different derivative than its visible page/object record.
7. Shared-file ownership — **PASS**: the four engine/integration files are explicitly authorized in the handoff; no unapproved product file changed.
8. Production fragility — **PASS**: no panic or hidden effect path was introduced.

Downstream note: WP08 and WP09 must not treat PDF reference semantics as accepted until the comparator/ID authority is corrected and independently reviewed.
