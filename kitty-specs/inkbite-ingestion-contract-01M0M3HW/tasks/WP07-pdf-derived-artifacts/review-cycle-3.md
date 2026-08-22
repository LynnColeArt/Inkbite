---
affected_files:
  - path: ingestion.go
  - path: converters/pdf/artifact_limits_test.go
cycle_number: 3
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: GOTOOLCHAIN=go1.26.6 go test ./converters/pdf -run TestWP07ReviewDoesNotRewriteEmbeddedSubstring -count=1
reviewed_at: '2026-08-22T13:54:14Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP07
review_artifact_override_at: "2026-08-22T14:07:24Z"
review_artifact_override_actor: "operator"
review_artifact_override_wp_id: "WP07"
review_artifact_override_reason: "Arbiter approval: terminal verification 921d6b9 independently reproduces red ccaef26 and green 50b5f24; exact start/end artifact-token grammar, canonical ID mapping, mutations, full/race/security/coverage/scope gates pass; retained review cycles 1-3 remain historical evidence"
---

# WP07 Review Cycle 3 — Changes Required

## Blocking issue — the grammar has no token-start boundary and rewrites embedded substrings

Cycle three correctly closes the post-ID grammar: `artifactReferenceDelimiter` admits only the committed ASCII delimiters, and URI, punctuation, alphanumeric, and Unicode continuations fail public `Ingest` with `ErrIntegrityFailure` and a zero envelope. However, `rewriteDetailedArtifactReferences` still locates the next occurrence with `strings.Index(remaining, "inkbite-artifact:")` and never validates the byte before that prefix (`ingestion.go:376-404`).

An independent disposable public-`Engine.Ingest` probe returned two real detailed artifacts in extraction order JPEG then PNG, forcing canonical sealing to map provisional ordinal 1 to final `artifact-000002`. Its detailed Markdown contained the literal substring:

```text
literal-prefixinkbite-artifact:artifact-000001
```

The final primary Markdown became:

```text
literal-prefixinkbite-artifact:artifact-000002
```

Legacy `Convert` preserved its literal text, but detailed ingestion silently rewrote ordinary embedded content that was not an exact standalone token. The same defect applies inside longer schemes/URLs, identifiers, and source text extracted from a PDF. This violates the requested exact-token grammar, FR-003's canonical Markdown fidelity, and the cycle-three acceptance requirement that substrings are not rewritten.

Required remediation:

1. Define a closed token-start grammar as well as the existing token-end grammar. At minimum, start-of-input and the deliberate Markdown/reference openers used by generated PDF output must be accepted; alphanumeric, identifier, URI/path, and other embedding predecessors must not trigger rewriting.
2. Prefer recognizing the complete local-reference token or Markdown destination structurally instead of applying a raw substring replacement to arbitrary converter text.
3. Add retained public-`Ingest` regressions where canonical reordering makes a rewrite observable. Cover embedded identifier text, a longer URI/path containing the prefix, adjacent prose, legitimate generated `(...)` references, exact token at EOF, and multiple legitimate references.
4. Require non-token substrings to remain byte-for-byte literal. Continue requiring malformed tokens that begin in a legitimate token context to fail with `ErrIntegrityFailure` and a zero envelope.
5. Mutation-prove both the start and end boundary guards independently, while preserving literal legacy `Convert` and default CLI output.

## Reproduced passing evidence

- Historical red `e688ac0` fails all ten new continuation cases for the intended reason; final `822a590` passes its authored exact-token end-boundary table.
- Exact/EOF and the accepted generated/Markdown delimiters `) ] > ' " SPACE TAB LF CR` succeed. Path, query, fragment, percent-encoded, colon, backslash, authority-like, punctuation, alphanumeric, Unicode, zero, out-of-range, short, and wrong-prefix cases fail typed with zero envelopes.
- Cycle-one canonical ID mapping remains correct for mixed JPEG/PNG artifacts; visible MIME, page, object, dimensions, bits/component, byte count, occurrence, and relationship target resolve to the intended final artifact.
- Cycle-two strictness and repeated-identical-byte distinct occurrence/reference behavior pass 100 repetitions.
- Legacy direct/engine `Convert`, `KeepDataURIs=false/true`, default CLI, text-only behavior, artifact limits, cancellation, visible degradation, ownership, identity mutation, and 100-run/concurrent determinism remain green.
- The default-accept delimiter mutation makes every continuation regression red and restores cleanly.
- Cycle-three correction scope is exactly `ingestion.go` and `converters/pdf/artifact_limits_test.go`; aggregate WP product scope remains exactly the four PDF-owned files and four authorized engine/integration files.
- Go 1.26.6 gates pass: focused count100, PDF count20/race5, full normal/race, vet, build, module verify/tidy-diff, gofmt, diff, staticcheck, CLI count20, Windows/Darwin compile, and frozen/API/scope checks.
- `govulncheck ./...` reports zero reachable vulnerabilities. PDF raw coverage is 81.5%; handoff fixed-base coverage is `1393/1532 = 90.926893%`, with cycle-three production coverage `6/6 = 100%`.
- Fixture SHA-256 remains `0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b`. No network, OCR, model, download, subprocess, dependency, module, license, contract, schema, or CLI authority changed.
- Disposable review probes and mutations were removed; final lane product bytes are restored.

## WP anti-pattern checklist

1. Dead code — **PASS**: reference resolution is live through public detailed ingestion.
2. Synthetic fixture — **PASS** for retained cases: they invoke real engine sealing and detailed conversion. The independent public probe exposes the missing start boundary.
3. Silent empty return — **PASS**: failures are typed with zero envelopes and optional extraction remains warning-visible.
4. FR coverage — **FAIL**: FR-003/FR-006 lack a retained assertion that embedded non-token content remains byte-exact.
5. Frozen surface — **PASS**.
6. Locked decision — **FAIL**: raw substring rewriting contradicts exact-token and canonical Markdown fidelity requirements.
7. Shared-file ownership — **PASS**: `ingestion.go` is explicitly authorized and the PDF test file is owned.
8. Production fragility — **PASS**: no panic, hidden effect, or transient race path was introduced.

Downstream note: WP08 and WP09 must not treat detailed PDF reference grammar as accepted until the start-boundary correction is independently reviewed.
