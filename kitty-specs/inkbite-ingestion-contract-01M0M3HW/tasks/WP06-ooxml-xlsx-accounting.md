---
work_package_id: WP06
title: OOXML and XLSX Accounting
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
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T025
- T026
- T027
- T028
agent: "codex:gpt-5.6-sol:implementer-ivan:implementer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/ooxml/
create_intent:
- internal/ooxml/package_test.go
- converters/docx/container_security_test.go
- converters/pptx/container_security_test.go
- converters/xlsx/container_security_test.go
execution_mode: code_change
model: ''
owned_files:
- internal/ooxml/package.go
- internal/ooxml/package_test.go
- converters/docx/docx.go
- converters/docx/docx_test.go
- converters/docx/container_security_test.go
- converters/pptx/pptx.go
- converters/pptx/pptx_test.go
- converters/pptx/container_security_test.go
- converters/xlsx/xlsx.go
- converters/xlsx/xlsx_test.go
- converters/xlsx/container_security_test.go
role: implementer
tags:
- ooxml
- xlsx
- archive
task_type: implement
---

# Work Package Prompt: WP06 – OOXML and XLSX Accounting

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan`. Use WP02's shared request ledger and WP04 pipeline. No raw ZIP bypass or private reset budget is permitted.

## Objective

Make OOXML package loading and XLSX preflight enforce the same actual-byte/container policy before downstream parsing, while preserving DOCX/PPTX/XLSX semantics.

## Subtasks

### T025 — Harden OOXML package loading

Replace unbounded package-member reads with shared ledger reads. Validate count/bytes/ratio/path/duplicates/types/checksum/cancellation and relationship targets. Read accepted members through EOF.

### T026 — Adopt in DOCX and PPTX

Thread request policy/ledger through both converters without altering text/relationship/notes/slide semantics. Add container-security tests for traversal, collisions, forged sizes, CRC, and limit boundaries.

### T027 — Preflight XLSX

Validate the bounded original archive before passing the same bytes to `excelize`. The preflight cannot be skipped, weakened by header declarations, or use a separate ledger.

### T028 — Cross-format fidelity and boundaries

Run identical hostile cases across DOCX/PPTX/XLSX plus normal goldens, cancellation, and race isolation. At-limit passes, +1 fails before uncontrolled third-party expansion.

## Review Gates

- [ ] No unbounded `io.ReadAll` remains on owned archive members.
- [ ] No raw ZIP opening bypasses preflight.
- [ ] Existing format tests and normalized Markdown remain stable.
- [ ] Detailed degradation is visible and ordered.

## Required Gates

```bash
gofmt -w internal/ooxml/*.go converters/docx/*.go converters/pptx/*.go converters/xlsx/*.go
go test ./internal/ooxml ./converters/docx ./converters/pptx ./converters/xlsx -count=20
go test -race ./internal/ooxml ./converters/docx ./converters/pptx ./converters/xlsx -count=5
go test ./...
go vet ./...
git diff --check
```

Handoff includes preflight proof, hostile matrix, legacy fidelity hashes, cancellation/race, coverage, and scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
- 2026-08-22T12:45:22Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Assigned agent via action command
