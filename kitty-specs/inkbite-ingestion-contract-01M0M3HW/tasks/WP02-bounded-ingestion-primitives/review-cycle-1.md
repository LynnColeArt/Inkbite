---
affected_files: []
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command:
reviewed_at: '2026-08-22T10:21:09Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP02
review_artifact_override_at: "2026-08-22T10:35:17Z"
review_artifact_override_actor: "operator"
review_artifact_override_wp_id: "WP02"
review_artifact_override_reason: "Cycle-2 independent review supersedes retained cycle-1 rejection: both recorded blockers are closed and independently reproduced. Red f7cc3cb/d744067/03cfbb3; green ca42126/a2c200b/a57324b. Reader len/cap, cap-clipped sealing, checked scratch arithmetic, ordered FactOrigin/schema SSOT and add/remove/reorder/change mutations pass. Hostile readers, budgets, sanitization, cancellation, Go 1.26.6 count/race/full/vet/build/module/govuln, 92.6% coverage, exact scope/frozen/public/module/license gates pass; staticcheck only inherited PDF-test U1000. Anti-pattern checklist: dead code N/A deliberate staged foundation; synthetic fixtures PASS; silent empty returns PASS; FR coverage PASS; frozen surface PASS; locked decisions PASS; shared ownership PASS; production fragility N/A."
---

# WP02 Review Cycle 1

Verdict: changes requested. The implementation is otherwise well-scoped and the normal, race, security, compatibility, and arithmetic gates are green, but the following blockers must be corrected before approval.

## Issue 1 — The bounded reader exceeds the configured allocation and accessible-capacity boundary

`internal/ingestion/bounded.go:77` always allocates a 32 KiB scratch buffer, even when the configured limit is smaller, and line 89 passes a two-index slice whose capacity remains 32 KiB. An independent hostile-reader probe at `limit=8` observed `len(p)=9` and `cap(p)=32768`. This contradicts the approved `plan.md:31` requirement that input/output allocation remain within configured limits plus the one-byte overflow probe. It also exposes a larger writable window than the advertised `limit+1` read window.

The same path returns accepted short reads without clipping accessible capacity: a three-byte input under a 64-byte limit produced `len(Bytes)=3, cap(Bytes)=64`. That weakens the exact-owned-byte boundary and is inconsistent with the cap-clipped behavior already provided by `OwnBounded` and `OwnedBytes.Clone`.

Required remediation:

1. Size scratch storage to no more than `min(chunkSize, limit+1)`, with checked handling for `math.MaxInt64` so the `+1` calculation cannot overflow.
2. Pass a three-index slice to `io.Reader` so `cap(p) == len(p)` for every read window, including the final overflow probe.
3. Cap-clip the accepted slice when sealing an `OwnedBytes` value so `cap(Bytes) == len(Bytes)` without changing its exact digest or length.
4. Add retained tests for zero, small, chunk-boundary, short-EOF, exact-limit, limit-plus-one, and `math.MaxInt64` policy cases. The tests must assert both the reader-visible window and returned-byte capacity.

## Issue 2 — The new closed metadata-origin vocabulary lacks its required contract mirror

`internal/ingestion/sanitize.go:23-28` introduces a second runtime representation of the contract's closed origin enum (`caller`, `source`, `sniff`, `converter`; `contracts/ingestion-envelope-v1.schema.json:59`). `TestCanonicalFactsAndOrigins` exercises all four values, but it does not mirror the complete accepted set against a literal contract copy. Adding a fifth accepted origin to `validOrigin` would leave the suite green. This fails the runtime-review contract round-trip/SSOT rule and reopens the exact vocabulary-drift class previously closed for the public WP01 enums.

Required remediation:

1. Give internal origin validation one enumerable single source of truth used by `validOrigin`.
2. Add a byte-for-byte ordered mirror assertion against the schema literal `["caller","source","sniff","converter"]`.
3. Mutation-prove that adding, removing, reordering, or changing one accepted origin makes the mirror test red, then restore the final tree.

## Reproduced evidence

- Red commit `517b701`: expected compile failure on missing WP02 symbols.
- Final commits `9338e00` and `f375d57`: focused `count=20`, race `count=5`, full tests, full race, vet, build, module verification, and diff check passed.
- Pinned Go 1.26.6 `govulncheck`: zero reachable vulnerabilities.
- Raw `internal/ingestion` coverage: 92.4%.
- Independent adversarial probes passed for negative/oversized reader counts, zero/at-limit/+1 reads, exact digests and clone ownership, all budget dimensions, checked aggregate overflow, failed-admission atomicity, container depth release, request concurrency/isolation, recursive path encoding, path/URL secret redaction, and cancellation categories/no-partial-object.
- Exact WP02 scope is the declared eight `internal/ingestion` files; frozen legacy files, module files, dependency/license files, and public API are unchanged. The wider lane diff contains only the approved WP01 dependency.
- Full staticcheck reports only the inherited pre-existing `converters/pdf/pdf_test.go:181` U1000 finding.

## WP-level anti-pattern checklist

1. Dead code — N/A for this deliberately staged internal foundation; downstream WP03–WP07 packages are its declared production consumers, and WP02 is forbidden from wiring them early.
2. Synthetic-fixture tests — PASS; tests invoke the production primitives.
3. Silent empty returns — PASS; zero values occur only with typed fail-closed errors.
4. FR coverage — PASS at this foundation's assigned boundary: exact bytes/digests, source/container/output limits, typed failures, and no partial success are exercised.
5. Frozen surface — PASS.
6. Locked decisions / `MUST NOT` clauses — PASS.
7. Shared-file ownership — PASS; no WP02 file is shared.
8. Production fragility — N/A; no panic/raise path was introduced.

Because WP03, WP04, WP05, WP06, WP07, and WP09 depend on WP02, do not activate or rebase them onto this lane until the corrected WP02 is approved.
