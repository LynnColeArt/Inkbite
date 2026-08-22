# Issue Matrix: Inkbite Ingestion Contract

This mission was created from project intent and did not import external tracker
issues. This matrix is the durable disposition record for every material
finding raised by independent work-package review, acceptance, merge, and the
post-merge mission audit. `fixed` means the merged product or canonical mission
record contains the independently verified correction. `verified-already-fixed`
means the issue was a workflow/integration condition whose prescribed recovery
was completed and revalidated without changing the governed product contract.

| Issue ID | Original finding | Owner / requirement | Verdict | Evidence |
|---|---|---|---|---|
| `INKBITE-001` | WP01 cycle 1: secret-bearing metadata, partially overlapping storage, a 257-artifact schema mismatch, and missing closed-enum mirrors were accepted. | WP01 / FR-002, FR-014, NFR-002, NFR-011 | `fixed` | [`review-cycle-1.md`](tasks/WP01-public-envelope-verification/review-cycle-1.md); terminal [`arbiter-verification.md`](tasks/WP01-public-envelope-verification/arbiter-verification.md); `ingestion_verify_test.go`. |
| `INKBITE-002` | WP01 cycle 2: absolute paths, contract-ceiling overflow, invalid UTF-8 Markdown, empty relationships, and encoded traversal remained valid. | WP01 / FR-003, FR-004, FR-014, NFR-011, NFR-013 | `fixed` | [`review-cycle-2.md`](tasks/WP01-public-envelope-verification/review-cycle-2.md); terminal arbiter verification; schema/Go round-trip and verifier tests. |
| `INKBITE-003` | WP01 cycle 3: nil required bytes, recursively encoded absolute paths, and accessible-capacity aliases escaped verification. | WP01 / FR-002, FR-003, FR-014, NFR-002 | `fixed` | [`review-cycle-3.md`](tasks/WP01-public-envelope-verification/review-cycle-3.md); terminal arbiter verification at green `68269b6`; capacity-reslice mutation tests. |
| `INKBITE-004` | WP02 cycle 1: bounded reads exposed oversized scratch capacity and the `FactOrigin` vocabulary lacked a schema-mirrored SSOT. | WP02 / FR-007, NFR-003, NFR-011 | `fixed` | [`review-cycle-1.md`](tasks/WP02-bounded-ingestion-primitives/review-cycle-1.md); green commits `ca42126`, `a2c200b`, `a57324b`; `internal/ingestion` boundary and enum-mutation tests. |
| `INKBITE-005` | WP03 cycle 1: current IANA non-global IPv6 space was admitted; cancellation wording overpromised preemption of arbitrary non-cooperative Go readers. | WP03 / FR-009, NFR-005, NFR-007 | `fixed` | [`review-cycle-1.md`](tasks/WP03-source-acquisition-remote-authority/review-cycle-1.md); binding [`arbiter-cancellation-ruling.md`](tasks/WP03-source-acquisition-remote-authority/arbiter-cancellation-ruling.md); 2025-10-09 IANA mirror and synchronized ten-case cancellation matrix. |
| `INKBITE-006` | WP04 cycle 1: sealed byte slices exposed writable spare capacity and artifact canonicalization was not fully semantic or ID-independent. | WP04 / FR-001, FR-005, FR-006, FR-015 | `fixed` | [`review-cycle-1.md`](tasks/WP04-engine-envelope-legacy-projection/review-cycle-1.md); green `a7a51a8`; cap-clipping, duplicate, ordering, relation, and 256/257 tests. |
| `INKBITE-007` | WP06 cycle 1: malformed referenced PPTX notes were silently omitted from detailed ingestion. | WP06 / FR-016 | `fixed` | [`review-cycle-1.md`](tasks/WP06-ooxml-xlsx-accounting/review-cycle-1.md); red `e7287e9`, green `32bb35b`; public `Engine.Ingest` warning and legacy-hash regressions. |
| `INKBITE-008` | WP07 cycle 1: PDF Markdown references were assigned before the engine's different final artifact sort and could resolve to the wrong image. | WP07 / FR-004, FR-006 | `fixed` | [`review-cycle-1.md`](tasks/WP07-pdf-derived-artifacts/review-cycle-1.md); terminal [`arbiter-verification.md`](tasks/WP07-pdf-derived-artifacts/arbiter-verification.md); mixed JPEG/PNG semantic-reference tests. |
| `INKBITE-009` | WP07 cycle 2: artifact references accepted path, query, fragment, percent, colon, and other URI continuations. | WP07 / FR-004, FR-006 | `fixed` | [`review-cycle-2.md`](tasks/WP07-pdf-derived-artifacts/review-cycle-2.md); terminal arbiter verification; malformed-continuation zero-envelope matrix. |
| `INKBITE-010` | WP07 cycle 3: raw substring scanning had no token-start boundary and rewrote embedded identifiers and prose. | WP07 / FR-003, FR-006 | `fixed` | [`review-cycle-3.md`](tasks/WP07-pdf-derived-artifacts/review-cycle-3.md); binding [`arbiter-ruling.md`](tasks/WP07-pdf-derived-artifacts/arbiter-ruling.md); green `50b5f24` and start/end mutation tests. |
| `INKBITE-011` | WP09 cycle 1: the quality cleanliness gate rejected the canonical review lock created by the reviewer itself. | WP09 / release gate integrity | `fixed` | [`review-cycle-1.md`](tasks/WP09-retained-acceptance-release/review-cycle-1.md); red `8c82a80`, green `3dc3118`; exact path/status/SHA review-lock tests. |
| `INKBITE-012` | WP09 cycle 2: release gates certified linked binary archives despite the recorded GPL-3.0-only `xlsReader` distribution gap. | WP09 / C-001, SC-007 | `fixed` | [`review-cycle-2.md`](tasks/WP09-retained-acceptance-release/review-cycle-2.md); binding [`arbiter-license-ruling.md`](tasks/WP09-retained-acceptance-release/arbiter-license-ruling.md); source-only package/publication matrix and nine release mutations. |
| `INKBITE-013` | Acceptance initially used generic Python-style workspace/test paths in a Go repository. | Mission acceptance / SC-001–SC-008 | `verified-already-fixed` | Local software-dev mission path configuration was corrected to repository-root Go paths; canonical acceptance passed at `bd1efb8` with all criteria and negative invariants green. |
| `INKBITE-014` | Merge preflight could not parse two historical `affected_files` lists expressed as strings rather than path mappings. | Mission merge / auditability | `verified-already-fixed` | Primary `b32e631` and coordination `641fa64` normalized only the historical review metadata; merge validation then passed without changing product bytes. |
| `INKBITE-015` | Lane G was stale after shared `ingestion.go` changes from WP06 and WP07 and conflicted during canonical integration. | Cross-WP integration / shared engine pipeline | `verified-already-fixed` | Merge `f1e2151` preserved both request-budget propagation and detailed-vs-legacy dispatch intent; full tests passed before the canonical mission squash `7d0c3ad`. |
| `INKBITE-016` | The mission squash restored the acceptance matrix's pending scaffold after canonical acceptance had passed. | Post-merge evidence integrity | `fixed` | The accepted 46-criterion/8-negative-invariant matrix was restored byte-for-byte from accepted tree `bd1efb8`; the post-merge mission review reran contract, acceptance, race, security, package, and quality gates. |

Accounting: **16 findings; 13 fixed; 3 verified already fixed; 0 deferred;
0 unknown; 0 unowned.** No external issue number, remote tracker mutation, binary
publication, or release was created while curating this matrix.

## Canonical closeout checks

```bash
test -f kitty-specs/inkbite-ingestion-contract-01M0M3HW/issue-matrix.md
jq -e '.overall_verdict == "pass" and ([.criteria[].pass_fail] | all(. == "pass")) and ([.negative_invariants[].result] | all(. == "confirmed_absent"))' \
  kitty-specs/inkbite-ingestion-contract-01M0M3HW/acceptance-matrix.json
GOTOOLCHAIN=go1.26.6 go test ./test/contract ./test/acceptance
```

The final release verdict remains the responsibility of the post-merge mission
review. This matrix supplies its issue-reconciliation gate; it is not a
self-approval record.
