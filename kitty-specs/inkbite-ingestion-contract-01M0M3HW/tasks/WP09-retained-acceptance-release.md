---
work_package_id: WP09
title: Retained Acceptance and Release Qualification
dependencies:
- WP01
- WP02
- WP03
- WP04
- WP05
- WP06
- WP07
- WP08
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
- FR-013
- FR-014
- FR-015
- FR-016
- FR-017
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T038
- T039
- T040
- T041
- T042
agent: "codex:gpt-5.6-sol:reviewer-renata:implementer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: reviewer-renata
authoritative_surface: test/acceptance/
create_intent:
- test/acceptance/retained_ingestion_test.go
- test/acceptance/reproducibility_test.go
- test/acceptance/security_boundaries_test.go
- scripts/verify-ingestion-contract.sh
- scripts/changed-coverage.sh
execution_mode: code_change
model: ''
owned_files:
- test/acceptance/retained_ingestion_test.go
- test/acceptance/reproducibility_test.go
- test/acceptance/security_boundaries_test.go
- scripts/verify-ingestion-contract.sh
- scripts/changed-coverage.sh
- .github/workflows/ci.yml
- Makefile
- .github/workflows/release.yml
- scripts/dist.sh
- README.md
- ADOPTED_COMPONENTS.md
- CHANGELOG.md
role: implementer
tags:
- acceptance
- release
- portability
task_type: implement
---

# Work Package Prompt: WP09 – Retained Acceptance and Release Qualification

## ⚡ Do This First: Load Agent Profile

Load `reviewer-renata`, then act as a release-gate implementer. Inspect all final public behavior but repair no product file; defects return to the owning WP. Preserve first governed failures.

## Objective

Prove a host can verify, persist, discard all engine/session/temp state, reload, and reverify exact source/Markdown/PDF derivative values; then qualify the frozen tree across deterministic, boundary, race, security, API, license, coverage, portability, and packaging gates.

## Subtasks

### T038 — Retained-ingestion journey

Through exported APIs only, ingest text and PDF, verify, persist canonical envelope plus every byte-bearing object in a host-owned store, close/drop all runtime values and original temp state, reload fresh values, reverify exact identities/bytes/relations, and clean only disposable state after durability.

### T039 — Aggregate matrices

Run 100 canonical conversions for text/PDF/office/nested ZIP; one-byte/missing/duplicate/cross-envelope mutations; every limit at/+1; remote/address/redirect; hidden model/component/download counters; cancellation; secret redaction; and 100 concurrent requests under race.

### T040 — Local quality/API/license gates

Add reproducible Make/scripts for gofmt check, vet, normal/race, build, staticcheck, govulncheck, module/dependency, MIT/adoption, API diff/downstream compile, fixed immutable-base changed production coverage >=80.0% unrounded, and coverage mutation self-test.

### T041 — Cross-platform CI

Keep least-privilege Linux/macOS/Windows verification plus race, vulnerability, and package jobs. Add acceptance/quality/API/license gates. Fresh `core.autocrlf=true` checkout must preserve PDF/binary fixture sizes/hashes. Packaging waits for all release gates.

### T042 — Terminal frozen-tree evidence

Run each mandatory command once in order, record exact tool versions/bases/numerator/denominator, audit generated/no-diff and exact scope, build deterministic archives twice, and preserve any failure. No tag/release/push/publish/remote mutation.

### Cycle 3 release-boundary amendment

The official artifacts qualified by this mission are reproducible source-only archives. They must contain the exact committed tracked-source manifest required by the release contract and must contain no linked executable, object file, vendored module tree, module-cache material, or third-party dependency source. CI, tag-release workflows, and the legacy distribution script must delegate to one canonical packaging authority and may upload only the qualified source archives plus their checksum manifest. Local and CI binary builds remain verification-only and are never publication inputs.

This amendment resolves the cycle-2 licensing rejection without changing runtime or XLS behavior. The default converter graph links GPL-3.0-only `xlsReader`; no binary produced from that graph is qualified for MIT-only redistribution. A binary-release strategy requires a separate specification and independent license review.

Authorized cycle-3 correction scope is limited to `scripts/verify-ingestion-contract.sh`, `test/acceptance/security_boundaries_test.go`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `scripts/dist.sh`, `README.md`, `ADOPTED_COMPONENTS.md`, and `CHANGELOG.md`. `go.mod`, `go.sum`, `Makefile`, `cmd/inkbite/main.go`, `builtins/defaults.go`, and `converters/xls/**` remain frozen.

## Review Gates

- [ ] Persistence uses fresh disk reads, not aliases; Inkbite remains storage-agnostic.
- [ ] All 17 FR, 13 NFR, 8 constraints, and 8 success criteria have inspectable evidence.
- [ ] Windows binary fixtures survive autocrlf checkout.
- [ ] API and legacy CLI compatibility pass.
- [ ] Known reachable vulnerabilities/licensing gaps/races are zero.

## Required Gates

```bash
go test ./test/acceptance -count=10
go test -race ./test/acceptance -count=3
go test ./...
go test -race ./...
make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342
go build ./...
go mod verify
govulncheck ./...
git diff --check ee5542edd1ac64b5f66dcb9d0056dd4815739342..HEAD
```

Report raw coverage arithmetic, fixture hashes, package reproducibility, all zero-effect counters, final ancestry/scope, and worktree cleanliness. Transition only on frozen green bytes.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
- 2026-08-22T14:40:40Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T14:58:02Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Blocked at first frozen governed gate: immutable-base git diff --check ee5542edd1ac64b5f66dcb9d0056dd4815739342..HEAD reports inherited out-of-scope trailing whitespace in mission checklists/contracts/data-model/plan/research/spec and retained WP04 review-cycle-1. Per WP09 release rules, no retry, suppression, product/planning repair, task completion, or for_review transition was performed. Owned commits 3421a3a/811e48a/b9c0042/2f91555 remain clean and ready after planning owner remediation.
- 2026-08-22T15:02:21Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Second frozen qualification stopped at mandatory command 5. Commands 1-4 passed once in order. make quality completed substantive checks (0 reachable vulnerabilities; API/license; 1815/2053=88.407209% changed coverage; 587-byte fixture sha256 0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b; reproducible package manifest sha256 f1e57b643f59a29851b54baf63cc5384fde8a185a8d67749d51c9ed6ac488641), then failed at scripts/verify-ingestion-contract.sh:165 with unbound local first from a retained RETURN trap. No edit/retry; mandatory commands 6-9 not run; tasks/status unchanged.
- 2026-08-22T15:09:44Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Ready for review on frozen 1a861c2ca88f9560b36384c810191357aa8e6735. Preserved failures: inherited whitespace freeze stopped before repair a7365de; second freeze stopped at RETURN-trap exit; red 3e042b9 and green 1a861c2 structurally confine cleanup to a subshell, with brace mutation reopening the regression. Final mandatory sequence ran exactly once from command 1 through 9, in order, with no post-freeze edits/retries: acceptance count10 PASS (Go reported 0.911s), acceptance race count3 PASS (2.416s), full normal PASS, full race PASS, make quality PASS, build PASS, module verify PASS, govuln PASS, immutable-base diff-check PASS. Go 1.26.6; staticcheck 2026.1/v0.7.0; govulncheck v1.6.0; git 2.43.0. Changed production coverage 1815/2053=88.407209% unrounded against ee5542edd1ac64b5f66dcb9d0056dd4815739342; mutation self-test red 1/5 green 5/5. PDF fixture 587 bytes sha256 0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b. Twice-built package manifest sha256 f1e57b643f59a29851b54baf63cc5384fde8a185a8d67749d51c9ed6ac488641. Remote/model/component/download effects all exactly zero in public matrices; 0 reachable vulnerabilities; direct license/API/legacy CLI checks pass. WP09 scope exact seven owned files plus authorized red regression in existing owned acceptance file; planning hygiene a7365de is root-owned exact seven Markdown files and no product change. Lane clean. Force acknowledges only inherited same-mission coordination residue; it does not bypass a product/release failure.
- 2026-08-22T15:10:37Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T15:15:53Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T15:19:31Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T15:28:21Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Cycle 2 ready on frozen 3dc3118d86343137389debd5e0f7b8a48e08b040. Preserved canonical cycle-1 review-lock failure and prior frozen failures. Red 8c82a80 proves unchanged lock rejected while created/unrelated dirt passed; green 3dc3118 permits only the exact pre-existing regular .spec-kitty/review-lock.json when status and SHA-256 remain unchanged, rejecting create/delete/modify/extra .spec-kitty and all other dirt; fingerprint mutation reopens the regression. Mandatory commands 1-9 ran exactly once in order on frozen bytes with no edits/retries: acceptance count10 PASS 2.49s; acceptance race count3 PASS 3.43s; full normal PASS 0.59s; full race PASS 2.16s; make quality PASS 19.32s; build PASS 0.48s; module verify PASS 1.29s; govuln PASS 3.51s; immutable-base diff-check PASS 0.01s. Go 1.26.6, staticcheck 2026.1/v0.7.0, govulncheck v1.6.0, git 2.43.0. Coverage 1815/2053=88.407209% unrounded against ee5542edd1ac64b5f66dcb9d0056dd4815739342; mutation self-test red 1/5 green 5/5; PDF fixture 587 bytes sha256 0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b; package manifest sha256 76a27641e158314a4f25b6dcf8771a01f353ea6b9e1b4c43601ffa19993dade3; zero reachable vulnerabilities and zero remote/model/component/download effects. a7365de and 1a861c2 are ancestors; lane scope from reviewed integration tip is exactly scripts/verify-ingestion-contract.sh plus test/acceptance/security_boundaries_test.go; worktree clean before transition. Force acknowledges only same-mission coordination residue after marking T038-T042, not a gate failure.
- 2026-08-22T15:29:27Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T15:37:36Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T15:52:58Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T16:09:59Z – codex:gpt-5.6-sol:reviewer-renata:implementer – shell_pid=2667806 – Cycle 3 source-only correction ready on frozen 2ee5bfcfe8487b38d9cc94894f44b0892a67057e under arbiter ruling cb482ca (and analysis bcc10ab), integrated without rewriting via merge 31f7cf8. Preserved cycle-2 rejection. Red 011c0d9 proves canonical five-platform linked-binary archives and absent release-surface authority violate the ruling. Green c27fea1/4203451/2ee5bfc emits deterministic exact tracked-source inkbite_VERSION_source.tar.gz and .zip plus checksums, independently inspects committed manifest/required files/modes/magic/dependency paths, rebuild-compares bytes, preserves review-lock cleanliness, delegates legacy dist, limits CI/tag uploads to exact source artifacts, and carries the exact GPL-linked-binary warning. Live mutation matrix rejects binary, vendor, missing required, extra entry, nondeterministic metadata, legacy divergence, broad CI/tag globs, and warning deletion. Mandatory commands 1-9 ran exactly once in order with no edits/retries: acceptance count10 PASS 103.52s; acceptance race count3 PASS 33.78s; full normal PASS 10.76s; full race PASS 12.24s; make quality PASS 18.58s; build PASS 0.49s; module verify PASS 1.30s; govuln PASS 3.67s; immutable-base diff-check PASS 0.01s. Go 1.26.6, staticcheck 2026.1/v0.7.0, govulncheck v1.6.0, git 2.43.0; coverage 1815/2053=88.407209%; coverage mutation red 1/5 green 5/5; fixture 587 bytes sha256 0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b; reproducible source-package manifest sha256 5830f8d2d497cd7ef52d417111337d3352ee02dc093e2a84267bdce527376fc9; zero reachable vulnerabilities and zero remote/model/component/download effects. Cycle-3 diff is exactly the eight authorized files; go.mod/go.sum/Makefile/CLI/builtins/XLS and all product remain frozen; lane clean. Ruff diff-scoped: no Python files, exit 0. No push/tag/release/publish/remote mutation was performed. Force acknowledges only same-mission planning coordination residue after T038-T042 marking, not a gate failure.
