---
work_package_id: WP07
title: First-Class PDF Derivatives
dependencies:
- WP01
- WP02
- WP04
requirement_refs:
- FR-003
- FR-004
- FR-006
- FR-010
- FR-015
- FR-016
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T029
- T030
- T031
- T032
agent: "codex:gpt-5.6-sol:implementer-ivan:implementer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: converters/pdf/
create_intent:
- converters/pdf/detailed_test.go
- converters/pdf/artifact_limits_test.go
execution_mode: code_change
model: ''
owned_files:
- converters/pdf/pdf.go
- converters/pdf/pdf_test.go
- converters/pdf/detailed_test.go
- converters/pdf/artifact_limits_test.go
role: implementer
tags:
- pdf
- artifacts
- compatibility
task_type: implement
---

# Work Package Prompt: WP07 – First-Class PDF Derivatives

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan`. Use WP01's optional capability and WP02 budgets through WP04. Do not change PDF dependencies or add OCR/network/model behavior.

## Objective

Expose existing embedded PDF image bytes as deterministic owned detailed artifacts with safe relationships/references, while preserving legacy inline/default Markdown behavior.

## Subtasks

### T029 — Return ordered image artifacts

Red-first fixture exposes embedded images currently available only to Markdown. Return exact bytes, MIME, safe name, role, byte count, identity, and stable page/object facts; clone parser buffers.

### T030 — Emit references and occurrences

Detailed Markdown uses non-network `inkbite-artifact:` references. Every occurrence resolves exactly once; repeated identical bytes may deduplicate only with deterministic occurrence relationships and visible evidence.

### T031 — Enforce limits and degradation

Apply primary, artifact count/per-item/aggregate budgets before success. Optional extraction failure is visible; policy-disallowed partial output is a typed failure. Add cancellation checkpoints around owned work.

### T032 — Preserve legacy and prove integrity

Lock `KeepDataURIs` false/true outputs, default CLI behavior, 100-run ordering, one-byte mutation denial, concurrent detailed/legacy conversion, and Windows fixture byte identity.

## Required Gates

```bash
gofmt -w converters/pdf/*.go
go test ./converters/pdf -count=20
go test -race ./converters/pdf -count=5
go test ./...
go test -race ./...
git diff --check
```

Report artifact ordering/relationships, legacy snapshots, limit/mutation evidence, fixture hash, raw coverage, and exact four-file scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
- 2026-08-22T12:45:26Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Assigned agent via action command
- 2026-08-22T13:07:41Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review: deterministic owned PDF image artifacts and inkbite-artifact references; legacy Convert/CLI KeepDataURIs behavior preserved through shared dispatch. Red 5f8dd1c; green c176881; hardening 766f43c. Go 1.26.6 PDF count20/race5, full normal/race, vet, build, module, staticcheck, govuln zero reachable, diff, Windows compile, CLI, 83.720930% fixed-base changed coverage, and three mutations pass. Exact scope: four WP PDF files plus authorized engine.go, ingestion.go, ingestion_test.go, conversion_integration_test.go.
- 2026-08-22T13:08:50Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T13:16:42Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T13:21:57Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
