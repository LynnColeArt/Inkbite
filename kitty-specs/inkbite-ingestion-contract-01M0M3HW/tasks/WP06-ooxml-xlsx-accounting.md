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
agent: "codex:gpt-5.6-sol:reviewer-renata:reviewer"
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
- 2026-08-22T13:12:57Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review: red ec2f9ee/4aa4d26/fa7bde6 plus causal encoded-collision and relationship-shape reds; final 8049aff. Go 1.26.6 focused count20/race5, full normal/race, vet, build, mod verify, gofmt, diff pass; govuln 0 reachable; target-package coverage 81.8%; staticcheck only inherited unchanged converters/pdf/pdf_test.go:181 U1000. Deleting XLSX preflight made traversal succeed with nil error, restored guard rejects integrity. Legacy Markdown SHA-256 docx 2e1443dabe953decefb63bfeb2566c95444bca56659458363ff7ac3089409156, pptx 14435c6edf4499d47ffd8e959ca74656ed4984b57ecce6e765ebd70c7d67e68a, xlsx ab59ca068d4fc9b0af5d62a091551c75eb54cea2457e6c8392333b9ac95e498f. Scope exact eight owned files plus authorized ingestion.go/internal/ingestion/context.go shared seams 6a5ced6 and dfad7d3; no production archive-member io.ReadAll or raw XLSX ZIP bypass; clean tree; ruff N/A no Python.
- 2026-08-22T13:13:47Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T13:22:39Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T13:24:10Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T13:32:44Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Cycle 2 blocker fixed red-first: e7287e9 proves public Engine.Ingest silently omitted malformed referenced PPTX notes while locking legacy Markdown hash; 32bb35b emits one deterministic payload-free optional_extraction_failed warning at the canonical notes location and preserves legacy projection. Behavioral mutation suppressing warning propagation turned the public regression red. Gates green: gofmt owned OOXML formats; package tests count=20; race package tests count=5; go test ./...; go vet ./...; git diff --check; PPTX coverage 82.0%; diff-scoped ruff N/A with no changed Python files. Cycle-2 scope is converters/pptx/pptx.go and converters/pptx/container_security_test.go only; no WP07 or reviewer metadata edits.
- 2026-08-22T13:33:48Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T13:38:27Z – user – shell_pid=2667806 – Cycle-2 independent approval supersedes retained cycle-1 rejection without rewriting it: red e7287e9 and green 32bb35b reproduced; the sole blocker is closed. Public Ingest emits deterministic safe optional_extraction_failed notes warnings; legacy hash/output unchanged; valid/empty/absent notes do not warn; multi-slide order and raw XML/backend/model sentinel redaction pass over 100 runs. Warning-propagation and XLSX-preflight mutations fail causally then restore green. Exact two-file correction and ten-file aggregate product scope; all accounting, fidelity, cancellation, concurrent isolation, count20/race5/full/vet/build/module/format/diff/vulnerability/coverage/API/frozen gates pass; staticcheck only inherited PDF-test U1000. Anti-pattern checklist PASS.
