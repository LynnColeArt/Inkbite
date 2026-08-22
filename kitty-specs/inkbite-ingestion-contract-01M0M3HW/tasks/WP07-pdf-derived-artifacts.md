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
branch_strategy: Spec Kitty allocates the lane; merge completed work only to feat/inkbite-ingestion-contract.
subtasks:
- T029
- T030
- T031
- T032
agent: codex
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: PDF conversion
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
