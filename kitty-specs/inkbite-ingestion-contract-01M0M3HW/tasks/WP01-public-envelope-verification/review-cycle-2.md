---
affected_files:
  - ingestion_verify.go
  - ingestion_verify_test.go
cycle_number: 2
mission_slug: inkbite-ingestion-contract-01M0M3HW
reviewed_at: '2026-08-22T08:37:00Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP01
---

# WP01 Review Cycle 2 — Changes Requested

The four cycle-one findings are corrected in `73dd7e4`: query and fragment metadata is rejected without echoing the sentinel, every non-empty storage overlap is detected while adjacent and independent equal slices remain valid, the absolute 256-artifact ceiling is enforced independently of request policy, and all four closed schema enums have ordered mirrors. The owned-file, compatibility, repetition, race, full-gate, mutation, and fixed-base coverage evidence is also sound.

The mandatory contract round-trip found the following remaining blockers.

## Blocking findings

### 1. Absolute host paths remain valid public metadata

`safePublicText` (`ingestion_verify.go`) rejects credentials, query strings, fragments, data URIs, authorization syntax, and control characters, but accepts absolute paths. A black-box envelope with `Warnings[0].Location = "/home/user/private/source.pdf"` returns `Valid: true`. The same predicate protects metadata facts, relation occurrences, component metadata, and warning details, so those surfaces have the same gap.

This contradicts `data-model.md`'s `WarningRecord.location` invariant ("No source bytes, credentials, or absolute paths"), research decision R-003 (absolute local paths are excluded from canonical provenance), NFR-011, and WP01 T003/T004.

Remediation: structurally reject POSIX and platform-independent absolute/volume paths on every public metadata surface where paths may appear, while retaining valid relative logical locations. Add black-box regressions for warning locations, fact values, relation occurrences, and component/provenance values. Findings must remain value-redacted.

### 2. The verifier accepts byte lengths forbidden by the committed v1 schema

`contracts/ingestion-envelope-v1.schema.json` fixes both source and artifact `byte_length.maximum` at `33554432`. `verifySource` and `verifyArtifact` instead allow the effective request policy to widen those values. Independently constructed envelopes containing `33554433` source bytes, primary Markdown bytes, or derivative bytes all return `Valid: true` when their policy fields are raised.

This is the same schema/Go valid-set divergence corrected for `artifacts.maxItems` in cycle one. It violates the contract round-trip gate, FR-014, and WP01 T004. It also exposes a planning contradiction: `plan.md` says callers may request a larger bounded source, while the committed v1 schema forbids it.

Remediation: obtain an explicit mission-level decision. Either enforce the v1 schema's absolute byte ceilings independently of per-request policy, or revise/version the contract through an approved scope exception. Add source, primary, and derivative tests at `33554432` and `33554433`; do not silently leave schema and `VerifyEnvelope` with different valid-envelope sets.

### 3. Primary Markdown is not verified as canonical UTF-8

An otherwise valid envelope whose primary bytes are `{0xff}` returns `Valid: true` after its identity and length are recomputed. FR-003 explicitly requires canonical UTF-8 primary Markdown, and WP01 T004 requires shape verification.

Remediation: reject non-UTF-8 primary bytes and add a black-box boundary test covering valid multi-byte UTF-8 and invalid byte sequences. Keep verification pure and value-redacted.

### 4. Required artifact relationships may be empty

An otherwise valid primary artifact with a non-nil but empty `Relations` slice returns `Valid: true`. The same applies to derived artifacts. Research R-002 requires every byte-bearing output to declare a deterministic relationship to its source or parent; FR-004 requires each derivative's source relationship; WP01 T004 explicitly requires relationship validation.

Remediation: require the primary and every derived artifact to carry at least one valid, non-self relationship and add tests for empty relations, self-reference, and the canonical source relationship. If an empty relation set is intentionally part of v1, make that a mission-level contract decision and reconcile the approved prose and schema before resubmission.

### 5. Percent-encoded traversal bypasses `safe_name` validation

`Source.SafeName = "%2e%2e/secret.pdf"` returns `Valid: true`: `safeName` checks raw path segments before considering the URL-decoded path. This contradicts the spec edge case forbidding traversal in artifact names/source locators and the `SourceArtifact.safe_name` invariant.

Remediation: validate the decoded canonical path form as well as the raw form, reject encoded separators and traversal segments, and add source/artifact regressions for encoded dot segments and separators. Do not decode into an authority-bearing path or echo the rejected value.

## Reproduced evidence

- Red `51caec1`: query/fragment, partial-overlap, and 257-artifact counterexamples fail as intended; adjacent ranges, independent equal bytes, the 256 boundary, and complete enum mirrors behave correctly.
- Green `73dd7e4`: all four cycle-one correction suites pass.
- Focused stability: WP01 suites pass `-count=100`; focused race passes `-race -count=10` on Go 1.26.6.
- Full gates: uncached normal and race suites, vet, build, module verification, gofmt, and diff checks pass. `govulncheck` reports zero reachable vulnerabilities. Staticcheck reports only unchanged `converters/pdf/pdf_test.go:181 U1000`.
- Compatibility: an external module implementing only legacy `Converter`, constructing `Result` positionally, comparing it, using it as a map key, calling `TextContent`, and typing all four legacy engine entry points compiles and passes.
- Coverage: immutable-base changed production coverage independently reproduces as `261/291 = 89.690722%`.
- Mutation evidence: deleting each query/fragment, overlap, or absolute artifact-count guard makes its dedicated regression fail, and restored final bytes pass.
- Isolation: the final diff contains exactly the eight owned files. `result.go`, `cmd/inkbite/main.go`, and `builtins/defaults.go` remain untouched.

## WP anti-pattern checklist

1. Dead code — **N/A**: WP01 is the declared public foundation for dependent WPs; no unplanned module exists, and externally callable compatibility was proven.
2. Synthetic-fixture test — **PASS**: tests invoke the production verifier/error/serialization paths; guard deletion causes the intended failures.
3. Silent empty return — **PASS**: nil/empty returns are limited to documented nil-receiver, no-attempt, or rejected-safe-detail paths.
4. FR coverage — **FAIL**: FR-003, FR-004, FR-014, and metadata-hygiene behavior remain incomplete as reproduced above.
5. Frozen surface — **PASS**.
6. Locked decision — **FAIL**: accepted absolute paths and encoded traversal contradict approved metadata rules; accepted 32 MiB + 1 values contradict the committed v1 schema.
7. Shared-file ownership — **PASS**: no shared or out-of-scope product file changed.
8. Production fragility — **PASS**: no panic, effectful verifier path, or bare-raise-equivalent was introduced.

Downstream note: WP02, WP03, WP04, WP07, and WP09 depend on WP01. Keep them unactivated until the contract decision and verifier corrections are independently reviewed and approved; any lane based on this snapshot must rebase afterward.
