---
work_package_id: WP05
title: Generic ZIP and EPUB Accounting
dependencies:
- WP02
- WP04
requirement_refs:
- FR-008
- FR-011
- FR-015
- FR-016
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Spec Kitty allocates the lane; merge completed work only to feat/inkbite-ingestion-contract.
subtasks:
- T020
- T021
- T022
- T023
- T024
agent: codex
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: generic ZIP and EPUB conversion
create_intent:
- converters/zip/container_security_test.go
- converters/epub/container_security_test.go
execution_mode: code_change
model: ''
owned_files:
- converters/zip/zip.go
- converters/zip/zip_test.go
- converters/zip/container_security_test.go
- converters/epub/epub.go
- converters/epub/epub_test.go
- converters/epub/container_security_test.go
role: implementer
tags:
- zip
- epub
- archive
task_type: implement
---

# Work Package Prompt: WP05 – Generic ZIP and EPUB Accounting

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan`. Consume the WP02 ledger and WP04 request pipeline; do not create another budget or change public types.

## Objective

Close archive-expansion, path, duplicate, type, checksum, nesting, and silent-omission gaps in generic ZIP and EPUB while preserving current Markdown fidelity.

## Subtasks

### T020 — Route ZIP through shared accounting

Red-first forged header, limit+1, count, total, ratio, traversal, backslash, absolute, NUL, duplicate/collision, symlink/type, and CRC tests. Read accepted entries through EOF; actual bytes are authoritative.

### T021 — Preserve one ledger through nesting

Recursive/nested conversion reuses the request ledger; depth, aggregate bytes, and nested count cannot reset or double-account. Nested expansion remains disabled or tightly policy bounded.

### T022 — Make degradation visible

Detailed mode represents unsupported entries, failed member conversion, and deduplication in stable warning/order or terminal typed failure. Legacy best-effort Markdown remains compatible where the contract allows it.

### T023 — Apply the same authority to EPUB

Bound container discovery, OPF/navigation, and content reads; reject unsafe internal references; preserve reading order; make optional skips/failures visible.

### T024 — Cross-format hostile matrix

Generate compact deterministic hostile fixtures in tests. Prove at-limit/+1, cancellation, race isolation, checksum completion, and exact existing fixture output for ZIP and EPUB.

## Required Gates

```bash
gofmt -w converters/zip/*.go converters/epub/*.go
go test ./converters/zip ./converters/epub -count=20
go test -race ./converters/zip ./converters/epub -count=5
go test ./...
go vet ./...
git diff --check
```

Handoff includes deterministic red bombs (not giant fixtures), nested ledger evidence, warning semantics, fidelity comparison, and exact scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
