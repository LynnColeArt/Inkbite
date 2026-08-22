---
affected_files:
  - ingestion.go
  - converters/pdf/artifact_limits_test.go
mission_slug: inkbite-ingestion-contract-01M0M3HW
review_kind: arbiter_terminal_correction
reviewed_at: '2026-08-22T13:56:24Z'
reviewer_agent: codex:gpt-5.6-sol:arbiter:arbiter
verdict: correction_required
wp_id: WP07
---

# WP07 binding arbiter ruling

All three review findings are valid and cumulative. Cycle 1 required the engine,
not the PDF converter, to assign final canonical artifact IDs. Cycle 2 required
the complete local URI to end at the artifact ID. Cycle 3 proves the remaining
scanner is asymmetric: it validates the token end but rewrites a matching
substring without validating the token start.

This ruling authorizes one terminal correction, not an ordinary fourth review
cycle. Product scope is exactly `ingestion.go` and
`converters/pdf/artifact_limits_test.go`; any wider edit requires renewed
arbitration.

## Binding grammar

- A rewritable local reference begins at byte zero or immediately after one of
  the deliberately supported ASCII Markdown openers: space, tab, CR, LF, `(`,
  `[`, `<`, single quote, or double quote.
- It is exactly `inkbite-artifact:artifact-NNNNNN` and ends at EOF or immediately
  before the already-bound ASCII closing delimiters: space, tab, CR, LF, `)`,
  `]`, `>`, single quote, or double quote.
- A prefix in embedded identifier text, a longer scheme/URL/path, or ordinary
  extracted prose is not a token and remains byte-for-byte literal.
- A malformed reference that starts in a legitimate token context fails with
  `ErrIntegrityFailure` and a zero envelope. No best-effort prefix rewrite is
  permitted.
- Engine sealing remains the sole final-ID authority. Legacy `Convert`, direct
  PDF conversion, default CLI output, and literal legacy text remain unchanged.

## Required red-first proof

Use public `Engine.Ingest` with at least two real artifacts whose canonical order
differs from extraction order so a rewrite is observable. Retain cases for:

1. embedded identifier text;
2. a longer URI and a path containing the prefix;
3. adjacent prose on both sides;
4. the generated parenthesized Markdown destination;
5. an exact token at EOF;
6. multiple legitimate references; and
7. every cycle-2 invalid continuation.

Mutation-prove the start and end guards independently. After the terminal green
commit, an independent arbiter verification must reproduce the historical red,
the terminal green, the complete cycle-1/2/3 matrix, focused count/race, full
normal/race, vet, build, module, formatting/diff, staticcheck, vulnerability,
CLI/platform, coverage >=80%, API/frozen/scope, and no-network/OCR/model gates.
If all pass, structured arbiter approval may supersede the retained rejection
artifacts without rewriting them. Any failure leaves WP07 planned and requires
human escalation.
