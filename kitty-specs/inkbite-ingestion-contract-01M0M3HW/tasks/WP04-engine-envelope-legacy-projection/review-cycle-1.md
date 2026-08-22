---
affected_files: []
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reviewed_at: '2026-08-22T12:16:57Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP04
review_artifact_override_at: "2026-08-22T12:44:07Z"
review_artifact_override_actor: "operator"
review_artifact_override_wp_id: "WP04"
review_artifact_override_reason: "Independent cycle-2 review supersedes retained cycle-1 rejection without rewriting it: red b999d8c and green a7a51a8 reproduced; both recorded blockers are closed. Exact-cap non-nil owned bytes, ID-independent full semantic artifact and relation ordering, alternating-order byte identity, positional-ID-independent duplicate rejection, and distinct 256-valid/257-invalid ceiling pass; clone, attribute-key, relation-endpoint, and verifier-duplicate mutations fail causally then restore green. Correction scope is exactly ingestion.go, ingestion_test.go, ingestion_verify.go, ingestion_verify_test.go; aggregate WP product scope is six expected Go files, with verifier crossing required by cycle-1 feedback. Go 1.26.6 focused count100/race10 and required count20/race5, 100 sequential/concurrent stress, full normal/race, vet, build, module, formatting, diff, API, frozen, scope, coverage 87.9%, and govuln zero reachable pass; staticcheck only inherited PDF-test U1000. Anti-patterns: dead code N/A; synthetic fixtures PASS; silent empty return PASS; FR coverage PASS; frozen surface PASS; locked decisions PASS; shared ownership PASS via cycle-1 coordination; production fragility N/A."
---

# WP04 Review Cycle 1 — Changes Required

Reviewer: Reviewer Renata  
Reviewed red: `1fa62d6`  
Reviewed final product commits: `baa2198`, `e2bc421`, `0ac8190`, `a66f156`, `227484e`

## Blocking issue 1 — Sealed byte objects expose unaccounted writable capacity

`ingestion.go:224`, `ingestion.go:226`, and `ingestion.go:248` use
`slices.Clone` for source, primary, and derivative bytes. Go explicitly
documents that `slices.Clone` may return additional unused capacity. A
disposable production-path probe ingested five-byte source, Markdown, and
derivative values and observed `len=5, cap=8` for all three returned slices.

The extra capacity is caller-accessible after reslicing but is absent from the
declared length, digest, and request-budget accounting. This contradicts the
required cap-clipped owned-byte boundary and leaves `TestIngestSealsExactOwnedArtifactsAndSelfVerifies`
green because that test checks only cross-object overlap, not `cap == len`.

Required remediation:

1. Clone source, primary, and every derivative into non-aliased storage and
   full-slice-clip every returned byte slice so `cap(Bytes) == len(Bytes)`,
   preserving the required non-nil present-empty representation.
2. Add a retained production-path matrix for empty, short, and boundary-sized
   source/primary/derivative values that asserts exact bytes, digest, length,
   budget accounting, caller/converter de-aliasing, sibling de-aliasing, and
   `cap == len`.
3. Mutation-prove that replacing the exact clone with `slices.Clone` or
   otherwise removing the capacity clip makes the retained test red.

## Blocking issue 2 — Artifact canonicalization has incomplete tie-breakers and the verifier accepts both orders

`artifactDraftKey` at `ingestion.go:340-342` sorts only occurrence, role,
safe name, and identity. It omits media type and canonical attributes. Two
artifacts can therefore share the sort key while differing in retained
contract data; stable sort then preserves converter-supplied order.

An independent alternating-order converter returned the same two artifacts
for the same source and configured engine. The artifacts shared occurrence,
role, safe name, bytes, identity, and media type, but carried different valid
canonical attributes. Consecutive `Ingest` calls emitted different JSON
envelopes, and both envelopes passed `VerifyEnvelope`.

The verifier does not close the defect: `artifactOrderKey` at
`ingestion_verify.go:675-686` begins with `relationKey`, whose `ToID` is the
already assigned positional artifact ID. Thus the IDs make either emitted
sequence appear sorted. This violates FR-005, FR-006, NFR-001, and the public
verification contract's canonical-order guarantee.

Required remediation:

1. Define one complete, ID-independent canonical ordering (or an explicitly
   documented fail-closed duplicate rule) over every retained field that can
   distinguish derivative artifacts, including canonical attributes and media
   type as applicable.
2. Use that semantic ordering before assigning `artifact-NNNNNN` IDs and make
   `VerifyEnvelope` independently reject any noncanonical order instead of
   deriving order from assigned IDs.
3. Add a retained alternating-order production-path test whose artifacts tie
   under the current key but differ in valid retained metadata. Require
   byte-identical envelopes on repeated calls and prove a sorter/verifier
   mutation red.

## Reproduced passing evidence

- Historical red `1fa62d6`: focused WP04 tests fail to compile because
  `Engine.Ingest` and `IngestOptions` do not yet exist.
- Exact product scope: only `conversion_integration_test.go`, `engine.go`,
  `ingestion.go`, and `ingestion_test.go` changed.
- One shared `runIngestionPipeline` / dispatch / seal path serves `Ingest` and
  all legacy entry points; no JSON round trip or second registry exists.
- Stable converter priority, reset-before-accept/convert, ordered unsupported
  and failed attempts, fallback warnings, detailed capability dispatch, and
  legacy-only adaptation pass.
- Caller/source/sniff/converter fact origins and caller precedence pass.
- Exact legacy Markdown/title projection passes for `Convert`, `ConvertPath`,
  `ConvertReader`, and `ConvertURI`.
- Failure and cancellation return zero envelopes; the binding WP03 cooperative
  and synchronously joined cancellation semantics remain intact.
- Retained 100 sequential and 100 concurrent mixed fallback/cancellation/alias
  stress passes under race, including ten repeated stress executions.
- Deleting self-verification makes the invalid-warning/media-type tests red;
  deleting the detailed remote-policy override makes the remote-authority test
  red. Both mutations were restored and their tests are green.
- Go 1.26.6 focused `-count=20`, focused race `-count=5`, full normal and race
  suites, vet, build, module verification, formatting, and diff checks pass.
- Root-package statement coverage is 87.6%; WP04 handoff changed-production
  coverage reports 86.301370%.
- `govulncheck` reports zero reachable vulnerabilities. Full `staticcheck`
  reports only the inherited unchanged `converters/pdf/pdf_test.go:181 U1000`.
- Public `Ingest` and `DetailedConverter` signatures round-trip the API
  contract; legacy `Result` and `Converter` remain unchanged. Frozen legacy,
  CLI, registry-default, module, dependency, license, and attribution surfaces
  are untouched.

## WP-level anti-pattern checklist

1. Dead code — **N/A**: `Engine.Ingest` is the contract's intentional public
   entry point; the new private pipeline has live `Ingest` and legacy callers.
2. Synthetic-fixture testing — **PASS** for retained tests: they invoke the
   production pipeline. The two missing adversarial cases above remain
   blockers.
3. Silent empty return — **PASS**: zero values are paired with typed errors;
   no swallowed-success path was introduced.
4. FR coverage — **FAIL**: returned capacity ownership and complete canonical
   artifact ordering are not covered and are observably incorrect.
5. Frozen surface — **PASS**.
6. Locked decisions — **FAIL**: both blockers contradict exact ownership,
   deterministic provenance/order, and public verification requirements.
7. Shared-file ownership — **PASS**: the four product files are WP04's
   exclusive lane-d scope.
8. Production fragility — **N/A**: no panic/raise-style production path was
   introduced.

## Governance and downstream note

The runtime correctly resolved `reviewer-renata`, but WP04 frontmatter still
records `agent_profile: implementer-ivan` and `role: implementer`. Correct the
handoff metadata in the next cycle so the prompt and runtime identity agree.

WP05, WP06, WP07, WP08, and WP09 depend on WP04. Keep them inactive and rebase
or resynchronize their lanes only after a corrected WP04 is independently
approved.

Verdict: **rejected**.
