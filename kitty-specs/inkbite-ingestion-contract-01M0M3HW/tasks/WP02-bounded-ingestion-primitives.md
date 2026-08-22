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
branch_strategy: Spec Kitty allocates the lane; merge completed work only to feat/inkbite-ingestion-contract.
subtasks:
- T006
- T007
- T008
- T009
agent: codex
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal bounded ingestion authority
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
