---
work_package_id: WP04
title: Single Engine Pipeline and Legacy Projection
dependencies:
- WP01
- WP02
- WP03
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-010
- FR-011
- FR-012
- FR-014
- FR-015
- FR-016
- FR-017
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T015
- T016
- T017
- T018
- T019
agent: "codex:gpt-5.6-sol:implementer-ivan:implementer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: ingestion.go
create_intent:
- ingestion.go
- ingestion_test.go
- sniff_test.go
- stream_info_test.go
execution_mode: code_change
model: ''
owned_files:
- engine.go
- engine_test.go
- conversion_integration_test.go
- ingestion.go
- ingestion_test.go
- sniff.go
- sniff_test.go
- stream_info.go
- stream_info_test.go
role: implementer
tags:
- engine
- compatibility
- concurrency
task_type: implement
---

# Work Package Prompt: WP04 – Single Engine Pipeline and Legacy Projection

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan`. Treat WP01–WP03 APIs as frozen. Preserve manual registry configuration, stable priority, Reset calls, normalization, and legacy signatures.

## Objective

Add the context-aware detailed `Ingest` entry point and make all legacy methods project from one shared selection/conversion pipeline. Seal and publicly verify a complete envelope before success; no second privileged path.

## Subtasks

### T015 — Compose one pipeline

Start with red tests showing detailed and legacy paths select/reset the same converter in the same order. Dispatch optional detailed capability when present; adapt legacy-only converters into primary-only detailed output.

### T016 — Preserve fact precedence and attempts

Keep caller hint precedence, source facts, and sniff-only-fill behavior while recording origin. Detailed provenance records ordered unsupported/failure attempts and the winner; legacy successful fallback remains compatible.

### T017 — Seal and self-verify

Build exact source/primary/derivatives/relations/warnings/provenance under request budgets, clone all bytes, canonicalize order, call `VerifyEnvelope`, and return success only after verification.

### T018 — Project legacy behavior

`Convert`, `ConvertPath`, `ConvertReader`, and `ConvertURI` delegate the shared path and return the exact historical Markdown/Title. No JSON round trip, detailed metadata stdout, or changed converter priority.

### T019 — Prove failure/cancellation/concurrency semantics

No policy/integrity/cancel/converter failure returns a successful envelope. Run 100 repeated and concurrent mixed requests, including fallback and cancellation, under race; registration remains configuration-time only.

## Review Gates

- [ ] One registry/pipeline only.
- [ ] Existing conversion/integration tests pass unchanged.
- [ ] Zero hidden component/model/download calls unless selected.
- [ ] No request state or byte buffer crosses calls.
- [ ] Canonical results are byte-identical for fixed inputs.

## Required Gates

```bash
gofmt -w engine.go engine_test.go conversion_integration_test.go ingestion*.go sniff*.go stream_info*.go
go test . -count=20
go test -race . -count=5
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Report legacy projections, attempt provenance, self-verification red proof, 100-run stress, coverage, and scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
- 2026-08-22T11:46:07Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Assigned agent via action command
- 2026-08-22T12:05:40Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for independent review: red 1fa62d6; green baa2198/e2bc421 plus hardening 0ac8190/a66f156/227484e. Exact owned product scope conversion_integration_test.go, engine.go, ingestion.go, ingestion_test.go. Go 1.26.6 focused count20/race5 and full normal+race/vet/build/module/gofmt/diff pass; govulncheck 0 reachable; apidiff compatible additions; owned coverage 189/219=86.301370%; 100 sequential+100 concurrent mixed fallback/cancel/alias stress; self-verify and remote-authority deletion mutations red then restored green; staticcheck only inherited PDF-test U1000; frozen/module/license surfaces unchanged. Charter section selector unavailable, direct charter review rules applied. Ruff diff-scoped N/A, no Python files, exit 0.
