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
branch_strategy: Spec Kitty allocates the terminal lane; merge completed work only to feat/inkbite-ingestion-contract before acceptance.
subtasks:
- T038
- T039
- T040
- T041
- T042
agent: codex
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: reviewer-renata
authoritative_surface: black-box acceptance and release gates
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
