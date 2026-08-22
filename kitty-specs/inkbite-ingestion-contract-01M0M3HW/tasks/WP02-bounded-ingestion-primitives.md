---
work_package_id: WP02
title: Bounded Ingestion Primitives
dependencies:
- WP01
requirement_refs:
- FR-002
- FR-007
- FR-008
- FR-011
- FR-015
- FR-016
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T006
- T007
- T008
- T009
agent: "codex:gpt-5.6-sol:implementer-ivan:implementer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/ingestion/
create_intent:
- internal/ingestion/bounded.go
- internal/ingestion/bounded_test.go
- internal/ingestion/budget.go
- internal/ingestion/budget_test.go
- internal/ingestion/sanitize.go
- internal/ingestion/sanitize_test.go
- internal/ingestion/context.go
- internal/ingestion/context_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/ingestion/bounded.go
- internal/ingestion/bounded_test.go
- internal/ingestion/budget.go
- internal/ingestion/budget_test.go
- internal/ingestion/sanitize.go
- internal/ingestion/sanitize_test.go
- internal/ingestion/context.go
- internal/ingestion/context_test.go
role: implementer
tags:
- limits
- budgets
- redaction
task_type: implement
---

# Work Package Prompt: WP02 – Bounded Ingestion Primitives

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan`. Treat WP01 public values as immutable. This package creates pure/internal primitives only; it must not wire sources, engines, or converters.

## Objective

Centralize bounded reads, copied bytes, exact digests, request-local ledgers, cancellation checkpoints, and safe metadata canonicalization so every later source and container uses the same authority.

## Subtasks

### T006 — Bounded reads and owned bytes

Red-first at-limit and limit-plus-one readers. Detect overflow by reading one extra byte, hash/copy the accepted bytes, distinguish cancellation/integrity/limit, and avoid unbounded `io.ReadAll`.

### T007 — Request-local budgets

Implement independent source, primary output, derivative count/per-item/aggregate, container entry/per-entry/aggregate/depth/ratio ledgers. Use checked arithmetic; declared sizes are hints while actual bytes are authoritative. No package global mutable request state.

### T008 — Safe canonical metadata

Normalize safe logical names, archive paths, URL displays, media/extension facts, and origins. Reject NUL, ambiguous traversal, absolute/drive/UNC/backslash paths, control characters, and sensitive payloads. Retain enough origin information for deterministic provenance.

### T009 — Cancellation and guard mutation tests

Add cheap context checkpoints for loops/reads and table-driven mutation/deletion tests for every central guard. Cancellation returns no successful object and wraps the context error.

## Review Gates

- [ ] Defaults match public policy exactly.
- [ ] Exact boundary arithmetic is tested, including overflow.
- [ ] Returned values own bytes.
- [ ] Sentinel URL credentials/data/source bytes never appear in errors.
- [ ] Race tests prove request isolation.

## Required Gates

```bash
gofmt -w internal/ingestion/*.go
go test ./internal/ingestion -count=20
go test -race ./internal/ingestion -count=5
go test ./...
go vet ./...
git diff --check
```

Report red/green commits, boundary table, mutation proof, raw coverage, and exact scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
- 2026-08-22T09:47:35Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Assigned agent via action command
- 2026-08-22T10:09:20Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review: red 517b701; green 9338e00 plus cancellation hardening f375d57; exact eight-file internal/ingestion scope; Go 1.26.6 focused 20x/race 5x and full normal/race/vet/build/mod/static/security gates pass; 92.4% raw coverage; bounded, budget, sanitize, and cancellation guard deletions each red then restored.
- 2026-08-22T10:10:26Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T10:21:10Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T10:22:48Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T10:31:32Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Cycle 2 ready: red f7cc3cb/d744067/03cfbb3; green ca42126/a2c200b; bounded scratch/window/seal and ordered origin SSOT corrected; origin add/remove/reorder/change plus bounded deletion mutations red and restored; Go 1.26.6 focused 20x/race 5x/full normal+race/vet/build/mod/static/vuln pass; 92.6% coverage; exact four-file correction within original eight-file scope.
