---
affected_files:
  - path: ingestion_verify.go
  - path: ingestion_verify_test.go
cycle_number: 3
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: GOTOOLCHAIN=go1.26.6 go test -count=1 ./...
reviewed_at: '2026-08-22T09:05:03Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
arbiter_required: true
arbiter_override:
  arbiter: architect-alphonso
  category: custom
  explanation: >-
    All three cycle-three findings are confirmed and require one bounded
    terminal correction in the verifier and its tests before approval.
  checklist:
    is_pre_existing: false
    is_correct_context: true
    is_in_scope: true
    is_environmental: false
    should_follow_on: false
  decided_at: '2026-08-22T09:13:16Z'
wp_id: WP01
review_artifact_override_at: "2026-08-22T09:46:15Z"
review_artifact_override_actor: "codex:gpt-5.6-sol:architect-alphonso:architect"
review_artifact_override_wp_id: "WP01"
review_artifact_override_reason: "[custom] Binding arbiter verification passed on 68269b6 under ruling fa72675; evidence: WP01-public-envelope-verification/arbiter-verification.md. This is the authorized terminal disposition, not ordinary review cycle 4."
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

## Binding Arbiter Ruling

**Disposition**: all three findings are confirmed. WP01 is authorized for one
bounded terminal correction; this is not an ordinary review cycle 4. The WP
remains unapproved, and every dependent remains held until the correction passes
the direct verification defined below.

### Reproduction judgment

An external Go consumer reproduced the behavior against final product commit
`ce659cf` without importing package internals:

1. nil source, primary, and derivative byte slices verify when their empty
   lengths and SHA-256 identities are made internally consistent; the same is
   true after JSON omission or `null` is unmarshaled;
2. warning metadata containing encoded POSIX, Windows-drive, recursively encoded,
   or labeled absolute paths verifies; and
3. length-disjoint slices with overlapping capacity, plus a zero-length slice
   retaining capacity, verify even though reslicing and mutation changes another
   byte-bearing object.

No cycle-three allegation is review overreach. Each is an observable v1 contract
defect within WP01's verifier boundary.

### Binding structural decisions

#### Required bytes: absent is not empty

- `Source.Bytes`, `Primary.Bytes`, and every derived artifact's `Bytes` MUST be
  non-nil.
- A present, non-nil zero-length slice is valid when every other v1 invariant is
  satisfied. Empty source and output values therefore retain the canonical
  empty SHA-256 identity; primary empty bytes are valid UTF-8 and do not fabricate
  content.
- JSON `bytes: ""` MUST round-trip to a non-nil empty slice and may verify.
  Missing `bytes` and `bytes: null` both unmarshal to nil and MUST produce a
  redacted `shape` finding at the exact byte field.
- This is a verifier shape rule. It does not require a model, schema, or contract
  version change.

#### Public metadata: bounded recursive decoding

- The verifier MUST inspect bare or path-like public metadata at every recursive
  percent-decoded representation, including the original form. Use one shared,
  bounded helper with a maximum of 16 decoding rounds; malformed encodings or a
  still-encoded value after round 16 fail closed.
- At every inspected form, reject invalid UTF-8, controls, credential/data-URI/
  authorization markers, query or fragment authority, file URLs, traversal, and
  portable absolute paths. Portable path recognition includes POSIX roots,
  Windows drive roots, UNC roots, and absolute paths following bounded labels or
  punctuation such as `path=`, `path:`, or `location(...)`.
- A syntactically explicit `http` or `https` URL with a non-empty host, no userinfo,
  query, fragment, or file authority remains a valid public locator. Its URL path
  is not reclassified as a host-local path merely because it begins with `/`;
  recursively revealed query, fragment, credentials, controls, or forbidden
  markers still fail closed.
- Findings MUST contain only the stable field path and generic reason. No decoded
  value, pointer, path, or sentinel may appear in a finding.

#### Byte ownership: accessible capacity is the observable range

- Pairwise alias detection MUST compare every byte-bearing slice's accessible
  half-open range `[unsafe.SliceData(s), unsafe.SliceData(s)+cap(s))`, not its
  current length range. A zero-length slice with positive capacity participates
  in overlap detection. A slice with zero capacity has no accessible byte range.
- Arithmetic overflow or an otherwise unrepresentable range MUST fail closed.
  Unsafe pointer values remain entirely private to the helper and MUST NOT enter
  reports, logs, exported types, or serialization.
- Do not impose `cap == len`: that would falsely reject ordinary independently
  allocated Go slices with spare capacity. Independent allocations, independent
  equal-content slices, and adjacent same-backing slices clipped with full-slice
  expressions (for example `backing[:4:4]` and `backing[4:8:8]`) are valid because
  their accessible ranges do not overlap.
- The public verifier can prove only aliasing observable among slices inside the
  envelope. It cannot discover an external alias retained by a caller; the later
  engine sealing path remains responsible for cloning caller and converter bytes.

### Exact terminal-correction allowlist

Product changes are limited to:

- `ingestion_verify_test.go` — red-first public regression matrix; and
- `ingestion_verify.go` — the three structural guards above.

Do not modify the public model, policy, schema, converter interface, errors,
legacy result, engine, dependencies, mission contracts, or any other product
file. The canonical arbiter metadata in this review artifact is coordination
evidence, not part of the lane's product allowlist.

### Mandatory red-first proof matrix

1. **Presence**: reject nil source, primary, and derivative bytes at their exact
   paths; reject JSON missing/null for all three; accept Go and JSON present-empty
   values for all three when otherwise canonical.
2. **Decoded paths**: exercise POSIX, Windows-drive, UNC, once encoded, recursively
   encoded, and labeled absolute paths across warning category/location/detail,
   metadata fact value, relation occurrence, provenance/policy component, and
   attempt category. Preserve relative logical locations, safe explicit HTTP(S)
   URLs, harmless escaped spaces, and harmless literal-percent controls. Prove
   that depth 16 terminates and unresolved depth 17 fails closed.
3. **Capacity ownership**: reject source-primary, source-derivative,
   primary-derivative, derivative-sibling, adjacent-length/capacity-overlap, and
   zero-length-positive-capacity aliases. Demonstrate the corresponding reslice
   mutation crosses the object boundary. Accept independent spare-capacity
   slices, independent equal-content slices, full-slice-clipped adjacency, and
   non-nil zero-capacity empty slices.
4. Every rejection MUST be value-redacted and categorized at the intended stable
   path; each valid control MUST pass the complete verifier, not only a helper.

Commit the failing matrix alone first and show that only the newly authorized
tests fail against `ce659cf`. Commit the production correction separately.

### Mandatory gates and terminal handoff

- Go 1.26.6 focused verifier/model/policy/error suites at `-count=100` and focused
  race at `-race -count=10`;
- uncached full `go test ./...`, `go test -race ./...`, `go vet ./...`,
  `go build ./...`, `go mod verify`, `gofmt`, and `git diff --check`;
- pinned `staticcheck` and `govulncheck`, documenting only genuinely inherited
  findings;
- immutable-base changed-production coverage of at least 80.0% unrounded;
- deletion mutations proving each nil-presence, recursive-decoding/depth, and
  capacity-range guard is causal;
- external legacy compatibility/API diff, exact two-file terminal-correction
  audit, exact eight-file aggregate WP ownership audit, frozen-surface audit, and
  no dependency/license delta.

After implementation, perform one fresh direct independent verification against
this ruling. If every matrix row and gate passes, the arbiter may record the
terminal evidence and force WP01 to approved with a structured arbiter note. If
any row fails, WP01 remains planned and escalates to the human operator; do not
open an ordinary fourth review cycle and do not activate a dependent.
