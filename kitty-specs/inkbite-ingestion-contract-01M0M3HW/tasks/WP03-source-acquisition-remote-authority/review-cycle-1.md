---
affected_files: []
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command:
reviewed_at: '2026-08-22T11:11:09Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP03
---

# WP03 Review Cycle 1 — Changes Required

Reviewer: Reviewer Renata
Reviewed range: `5a1eff2cfe763e7ed20a8e63381305d6e4bf5ae1..f5c8a49faab488b49b49a25fe6cc1a18dae33f52`

## Blocking issue 1 — The IPv6 SSRF deny set admits current IANA non-global space

`IsAllowedRemoteAddress` returns `true` for at least these addresses:

- `100:0:0:1::1`, within IANA's `100:0:0:1::/64` Dummy IPv6 Prefix, whose `Destination`, `Forwardable`, and `Globally Reachable` properties are false;
- `2001:5::1`, an unexcepted address under IANA's `2001::/23` IETF Protocol Assignments umbrella, whose default `Destination`, `Forwardable`, and `Globally Reachable` properties are false unless a more-specific allocation says otherwise.

Independent disposable tests calling the production `IsAllowedRemoteAddress` path failed on both sentinels. This violates WP03's requirement to reject IANA non-global classes and the plan's maintained-registry, fail-closed SSRF decision. The existing matrix covers many familiar ranges but is not exhaustive enough to establish the stated invariant.

Fix the address policy so registry space is denied by default wherever required while handling explicitly globally reachable suballocations deliberately. Add auditable IPv4 and IPv6 registry-table tests, including umbrella-prefix exceptions, `100:0:0:1::/64`, unallocated/non-global `2001::/23` space, IPv4-mapped IPv6, and mixed DNS answers. The test must fail if either missing admission guard is deleted. Cite or snapshot the registry revision used so future additions can be reviewed rather than silently missed.

Primary evidence: IANA IPv6 Special-Purpose Address Space registry, last updated 2025-10-09: <https://www.iana.org/assignments/iana-ipv6-special-registry/iana-ipv6-special-registry.xhtml>.

## Blocking issue 2 — An in-flight blocking `io.Reader` without `io.Closer` ignores cancellation

`readSourceBounded` delegates a non-`io.Closer` directly to synchronous `internal/ingestion.ReadBounded`. Once that reader's `Read` method blocks, `Checkpoint` cannot observe cancellation until `Read` returns.

An independent disposable sentinel waited until a custom non-closing reader entered `Read`, canceled its context, and then timed out after 250 ms; the acquisition returned only after the test manually released the reader. The same structural gap applies to a blocking non-closing `io.ReadSeeker`. This contradicts T010, NFR-007, and the plan's explicit risk that non-cooperative sources must not outlive or leak past the public failure boundary. The existing `blockingReadCloser` test proves only the cooperative close path and therefore does not cover the full reader contract accepted by `acquireSource`.

Choose and document a cancellation-safe design for the public `io.Reader`/`io.ReadSeeker` surface. Add tests that synchronize on entry into `Read` before canceling for both closing and non-closing readers, require a typed cancellation with no partial source, and demonstrate the chosen design does not merely abandon an unjoined worker. Arbitrary `io.Reader` cannot be forcibly interrupted, so this needs an explicit contract/architecture decision rather than another timing-only test.

## Non-blocking governance note

The review invocation correctly resolved `reviewer-renata`, but the WP frontmatter still records `agent_profile: implementer-ivan` and `role: implementer`. Preserve the runtime's reviewer identity in the next handoff so the prompt and frontmatter agree.

## Evidence and gates

- Exact scope: PASS — only `source.go`, `source_test.go`, `options.go`, `internal/ingestion/remote.go`, and `internal/ingestion/remote_test.go` changed.
- Historical red: PASS — `e62f77c` fails on absent bounded-source and remote-authority production paths.
- Go 1.26.6 focused normal `-count=20`: PASS.
- Go 1.26.6 focused race `-count=5`: PASS.
- Full normal and full race suites: PASS.
- `go vet ./...`, `go build ./...`, `go mod verify`, `git diff --check`, and formatting: PASS.
- `govulncheck ./...`: PASS — zero reachable vulnerabilities.
- Coverage: PASS — root package 86.3%; `internal/ingestion` 89.5%.
- Public root API documentation comparison against the pre-WP parent: PASS — no exported root API drift.
- Frozen surfaces (`result.go`, `cmd/inkbite/main.go`, `builtins/defaults.go`), module files, and license: PASS — untouched.
- Staticcheck: inherited-only warning — `converters/pdf/pdf_test.go:181:6 makeSimplePDF` is pre-existing and outside WP03 scope.
- Deletion/mutation reachability: PASS for the existing disabled-authority, `Content-Length`, pinned-dial, and caller-provenance guards; each targeted production mutation is killed by its corresponding test.

## Anti-pattern checklist

1. Dead code: PASS — new acquisition and pinned-transport paths have live production callers.
2. Synthetic fixtures: PASS for existing tests — assertions call production paths; the missing adversarial cases above remain blockers.
3. Silent empty return: PASS — zero-value failure returns are paired with typed errors; no silent success fallback was introduced.
4. FR coverage: FAIL — FR-009's IANA non-global invariant and FR-007/NFR-007's accepted-reader cancellation boundary are incomplete.
5. Frozen surface: PASS.
6. Locked decisions: FAIL — the two defects contradict explicit fail-closed and cancellation decisions in the mission contracts.
7. Shared-file ownership: PASS — the five files are lane-c's exclusive write scope.
8. Production fragility: N/A — no panic/raise-style production path was introduced.

Verdict: **rejected**. Address both blocking issues and rerun the retained gates before requesting cycle-2 review.
