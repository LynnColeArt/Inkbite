---
affected_files:
  - ingestion_verify.go
  - ingestion_verify_test.go
cycle_number: 3
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: GOTOOLCHAIN=go1.26.6 go test -count=1 ./...
reviewed_at: '2026-08-22T09:05:03Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
arbiter_required: true
wp_id: WP01
---

# WP01 Review Cycle 3 — Changes Requested (Arbiter Required)

The five cycle-two findings are corrected in `ce659cf`, and all four cycle-one corrections remain green. Exact red `03ec931` reproduces the five intended failures; final green closes those authored cases. Independent contract and ownership probes nevertheless found three remaining blockers.

This is the third rejected review cycle. **Do not open an ordinary cycle 4. Route WP01 to the configured arbiter before any further implementation or dependent activation.**

## Blocking findings

### 1. Required byte objects can be omitted while verification succeeds

The v1 schema requires `source.bytes` and every artifact's `bytes` (`contracts/ingestion-envelope-v1.schema.json:31-33`, `:65-69`). `verifySource` and `verifyArtifact` recompute the empty digest but never distinguish a required empty byte string (`[]byte{}`) from an absent/null Go slice (`nil`) (`ingestion_verify.go:132-157`, `:207-240`). External black-box envelopes with either `Source.Bytes = nil` or `Primary.Bytes = nil`, zero length, the independently recomputed empty SHA-256 identity, and otherwise canonical fields both return `Valid: true`.

This violates the schema/Go valid-set agreement, FR-002/FR-003/FR-014, SC-005's missing-object requirement, and the data-model rule that every byte-bearing object verifies.

Remediation: reject nil bytes for the source, primary, and every derivative while retaining a deliberate, non-nil zero-length payload if v1 permits it. Add black-box Go and JSON omission/null regressions for all three surfaces and a schema round-trip assertion that distinguishes missing/null from present-empty bytes.

### 2. Encoded absolute paths bypass the shared public-metadata boundary

`safePublicText` checks only the raw text with `containsAbsolutePath` (`ingestion_verify.go:541-575`). Unlike `safeName`, it does not inspect a bounded decoded representation. Consequently warning metadata containing `%2fhome%2fuser%2fPRIVATE_PATH_SENTINEL` or `C:%5cUsers%5cuser%5cPRIVATE_PATH_SENTINEL` returns `Valid: true`. The same predicate protects metadata fact values, relation occurrences, component/provenance values, attempt categories, and warning fields.

This leaves the cycle-two absolute-path defect structurally open and contradicts the host-local-path exclusions in `data-model.md:63,90,144`, the canonical-provenance boundary, NFR-011, and WP01 T003/T004.

Remediation: normalize bare/path-like public metadata through a bounded recursive decode before portable absolute/volume-path admission, without turning legitimate authority-bearing URLs into local paths. Cover POSIX, Windows drive, UNC, percent-encoded, recursively encoded, and labeled-path representations across warning, fact, relation, and provenance/component surfaces. Findings must remain value-redacted.

### 3. Accessible slice capacity can still alias byte-bearing objects

`storageRangesOverlap` compares only `[data, data+len)` and returns false for zero-length slices (`ingestion_verify.go:387-410`). The existing adjacent-range test therefore accepts `source = backing[:4]` and `primary = backing[4:]`; however `cap(source) == 8`, so reslicing source to five bytes and mutating index four changes `primary[0]`. A zero-length `backing[:0]` with retained capacity likewise passes and can be resliced into the primary payload.

This is observable shared mutable storage and violates C-007 plus the SourceArtifact/ContentArtifact ownership invariants (`data-model.md:58,73`). The current adjacent test proves only non-overlapping lengths, not ownership.

Remediation: enforce non-overlap over every slice's accessible capacity, or require sealing to publish full-slice-capacity values (`cap == len`) and verify that invariant. Replace the adjacent control with full-slice expressions when adjacency is intentionally safe, and add post-verification reslice/mutation regressions for source-primary, source-derivative, primary-derivative, siblings, and zero-length retained-capacity cases.

## Reproduced evidence

- Red `03ec931`: all five cycle-two counterexample groups fail for the intended reasons.
- Green `ce659cf`: absolute raw paths, v1 32 MiB source/primary/derivative ceilings, UTF-8 primary bytes, non-self relationship coverage, and encoded safe-name traversal/separators pass their authored suites.
- Cycle-one corrections: sensitive query/fragment metadata, all non-empty length overlaps, the 256/257 artifact ceiling, and all four ordered closed-enum mirrors remain green.
- Mutation: deleting each of the five cycle-three guards makes its dedicated regression fail; restored final bytes are green.
- Stability: focused WP01 suites pass `-count=100`; focused race passes `-race -count=10` on Go 1.26.6.
- Full gates: uncached normal and race suites, vet, build, module verification, gofmt, and diff checks pass. `govulncheck` reports zero reachable vulnerabilities. Staticcheck reports only unchanged `converters/pdf/pdf_test.go:181 U1000`.
- Coverage: immutable-base changed production coverage independently reproduces as `321/356 = 90.168539%`, above 80.0% unrounded.
- Compatibility/isolation: the external legacy converter/Result/engine fixture passes; the diff is exactly the eight owned files. `result.go`, `cmd/inkbite/main.go`, and `builtins/defaults.go` plus dependency/license files remain untouched.

## WP anti-pattern checklist

1. Dead code — **N/A**: WP01 is the declared public foundation for dependent WPs; no unplanned module exists.
2. Synthetic-fixture test — **PASS** for the authored verifier/error/serialization paths; deletion mutations fail correctly. The independent external probes expose missing boundaries.
3. Silent empty return — **PASS**: nil/empty returns in production are documented error or no-attempt paths.
4. FR coverage — **FAIL**: FR-002, FR-003, FR-014, and the ownership boundary are incomplete for missing byte objects and capacity aliases.
5. Frozen surface — **PASS**.
6. Locked decision — **FAIL**: encoded host-local paths and shared accessible backing storage contradict approved trust and ownership constraints.
7. Shared-file ownership — **PASS**: only WP01-owned product files changed.
8. Production fragility — **PASS**: no panic, effectful verifier path, or bare-raise-equivalent was introduced.

Downstream note: WP02, WP03, WP04, WP07, and WP09 remain blocked. No dependent lane should activate or rebase onto WP01 until the arbiter selects a disposition and the resulting bytes receive the required independent verification.
