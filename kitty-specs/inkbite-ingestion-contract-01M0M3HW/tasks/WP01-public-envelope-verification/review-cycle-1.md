---
affected_files: []
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command:
reviewed_at: '2026-08-22T08:09:20Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP01
---

# WP01 Review Cycle 1 — Changes Requested

Independent review of lane commit `7a7de3a` against coordination base `cb889c4` found four blocking contract defects. The implementation remains confined to the eight owned files and the general repository gates are green, but `VerifyEnvelope` accepts envelopes that violate the approved v1 contract.

## Blocking findings

### 1. Secret-bearing query and fragment metadata is accepted as safe

`safeName` and `safePublicText` validate URL userinfo but do not reject `RawQuery` or `Fragment` (`ingestion_verify.go:424`, `ingestion_verify.go:437`). An external black-box consumer set:

- `Source.SafeName = "brief.pdf?token=TOPSECRET#private"`; and
- `Warnings[0].Location = "https://example.test/document?token=TOPSECRET#private"`.

`VerifyEnvelope` returned `Valid: true` with zero findings. This contradicts `data-model.md:63`, NFR-011, R-003/R-005, and WP01 T003/T004.

Remediation: make verification reject authority-bearing or sensitive query/fragment material wherever public names, locations, warnings, facts, or other metadata can carry it. Add black-box regressions covering source/artifact names and warning/fact/location values, including query, fragment, userinfo, data URI, authorization, and control-character sentinels. Findings must remain redacted.

### 2. Partially overlapping byte slices evade ownership verification

`verifyOwnership` compares only `&slice[0]` equality (`ingestion_verify.go:342`). A source slice and primary slice taken from different offsets of the same backing array therefore pass. An external consumer constructed source `backing[:6]` and primary `backing[1:8]`; after recomputing both identities and lengths, `VerifyEnvelope` returned `Valid: true`.

This violates C-007, the SourceArtifact/ContentArtifact ownership invariants, and WP01 T004. It also permits mutation of one accepted object's retained bytes to mutate another accepted object.

Remediation: detect every non-empty storage-range overlap, not only identical starting addresses, and add regression coverage for exact aliases, prefix/suffix overlap, interior overlap, adjacent non-overlap, and independent equal-content slices. Keep verification pure and non-mutating.

### 3. The verifier accepts a shape forbidden by the committed v1 schema

`contracts/ingestion-envelope-v1.schema.json:14` fixes `artifacts.maxItems` at 256. `verifyArtifacts` instead accepts any count allowed by the envelope's mutable `Policy.MaxArtifacts`. An otherwise valid envelope with 257 canonically ordered artifacts and `Policy.MaxArtifacts = 257` returned `Valid: true`.

Remediation: reconcile the public verifier with the committed v1 wire contract. Either enforce the schema's absolute v1 ceiling independently of the effective per-request policy, or obtain an explicit mission-level contract decision and update the schema/plan before resubmission. Add a boundary test proving the chosen rule at 256 and 257. Do not silently let Go values and the contract schema describe different valid-envelope sets.

### 4. Closed contract enums have no byte-for-byte SSOT mirror assertion

The contract pins closed sets for source kinds (`schema:37`), relation kinds (`schema:46`), metadata origins (`schema:59`), and attempt outcomes (`schema:84`). Current tests exercise only subsets and contain no assertion mirroring the complete schema sets against the runtime constants. This fails the required contract round-trip SSOT check and permits vocabulary drift that the golden fixture cannot detect.

Remediation: add one explicit test-side mirror for all four closed sets and compare it byte-for-byte/in-order with the runtime v1 values. Keep the planning contract immutable unless a separately approved contract correction is required.

## Reproduced evidence

- Red proof: detached `c27b1d7`, `GOTOOLCHAIN=go1.26.6 go test -count=1 .` failed on the missing envelope/policy/verifier symbols as intended.
- Green focused: verifier/serialization/policy/error suites passed with `-count=100`; focused race passed with `-race -count=10`.
- Full gates: uncached `go test -count=1 ./...`, uncached `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...`, `go mod verify`, `git diff --check`, and `gofmt -l` passed.
- Supply chain: `govulncheck ./...` reported zero reachable vulnerabilities.
- Static analysis: only unchanged pre-existing `converters/pdf/pdf_test.go:181 U1000` remains; no changed-file staticcheck finding.
- Coverage: owned changed production files cover `258/299` statements = `86.287625%`, above the 80.0% unrounded threshold.
- Compatibility: an external consumer implementing legacy `Converter`, using unkeyed/equality/map-key `Result`, compiled and passed; frozen `result.go`, `cmd/inkbite/main.go`, and `builtins/defaults.go` are untouched.
- Isolation: exact diff contains only `converter.go`, `errors.go`, `ingestion_model.go`, `ingestion_policy.go`, `ingestion_verify.go`, and their three owned test files.

## WP anti-pattern checklist

1. Dead code — **N/A**: WP01 intentionally establishes externally callable public foundation types/functions consumed by the declared downstream WPs; no unplanned module was added.
2. Synthetic-fixture test — **PASS** for the implemented public verifier/serialization/error paths; the new black-box counterexamples demonstrate the missing assertions.
3. Silent empty return — **PASS**: no undocumented silent-success return path was introduced.
4. FR coverage — **FAIL**: FR-014 verification acceptance is incomplete for metadata hygiene, overlapping ownership, and schema bounds.
5. Frozen surface — **PASS**.
6. Locked decision — **FAIL**: accepting query/fragment secrets contradicts the approved no-sensitive-public-metadata decision, and accepting 257 artifacts contradicts the v1 schema.
7. Shared-file ownership — **PASS**: no shared or out-of-scope file changed.
8. Production fragility — **PASS**: no new panic/bare-raise-equivalent path was introduced.

Downstream note: WP02, WP03, WP04, WP07, and WP09 depend on WP01. They must remain unactivated until WP01 is corrected, independently re-reviewed, and approved; if any downstream lane has based work on this rejected snapshot, rebase it after the corrected WP01 lands.
