---
affected_files:
  - ingestion.go
  - converters/pdf/artifact_limits_test.go
mission_slug: inkbite-ingestion-contract-01M0M3HW
review_kind: arbiter_terminal_verification
reviewed_at: '2026-08-22T14:06:47Z'
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: approved
wp_id: WP07
verified_red: ccaef26
verified_green: 50b5f24
tree_hash: 6e7f8ba8e9bafc09013146cb1c52d7a6ec9066a2
---

# WP07 terminal arbiter verification

Verdict: **APPROVED**. The terminal correction at `50b5f24` satisfies the
binding ruling without product-scope expansion. No ordinary review cycle 4 was
opened and no product edit was made during verification.

## Independent red and green proof

- Historical red `ccaef26` reproduces the cycle-3 defect: embedded identifier,
  longer URI/path, and adjacent-prose occurrences are rewritten as if they were
  standalone local references.
- Green `50b5f24` passes the complete start/end grammar through public
  `Engine.Ingest` with real mixed JPEG/PNG artifacts whose extraction and final
  canonical orders differ.
- Embedded identifier, URI, path, and adjacent prose remain byte-for-byte
  literal. Generated parenthesized destinations, EOF tokens, multiple tokens,
  and every bound opener/closer rewrite to the correct final artifact.
- Short, wrong-prefix, zero, out-of-range, alphanumeric, path, query, fragment,
  percent-encoded, colon, backslash, authority, userinfo, punctuation, and
  Unicode continuations in a legitimate token context return
  `ErrIntegrityFailure` with a zero envelope.
- Cycle-1 mixed-media mapping and cycle-2 repeated-identical-byte occurrences
  pass at count 100 and race count 10.
- Independently disabling the token-start guard rewrites literal substrings;
  disabling the token-end guard admits every forbidden continuation. Both
  mutations fail causally and exact restoration is green.

## Gates and scope

Go 1.26.6 PDF count 20/race count 5, full normal/race, vet, build, module
verification and tidy no-diff, gofmt/diff, staticcheck, govulncheck (zero
reachable), CLI count 20, Windows/Darwin compile, API/frozen/no-authority, and
exact-scope checks pass. PDF raw coverage is 81.5%; the corrected root rewrite,
opener, and delimiter functions are each 100% covered. Terminal scope is exactly
`ingestion.go` and `converters/pdf/artifact_limits_test.go`; aggregate WP07 scope
is the eight previously authorized Go files.

Verified tree: `6e7f8ba8e9bafc09013146cb1c52d7a6ec9066a2`.
PDF fixture SHA-256:
`0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b`.

This verification authorizes the structured arbiter override of the three
retained rejection artifacts. They remain immutable historical evidence.
