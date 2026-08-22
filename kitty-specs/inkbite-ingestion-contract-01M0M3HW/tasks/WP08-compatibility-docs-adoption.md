---
work_package_id: WP08
title: Compatibility Documentation and Civic Adoption
dependencies:
- WP03
- WP04
- WP05
- WP06
- WP07
requirement_refs:
- FR-012
- FR-013
- FR-014
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Spec Kitty allocates the lane; merge completed work only to feat/inkbite-ingestion-contract.
subtasks:
- T033
- T034
- T035
- T036
- T037
agent: codex
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: curator-carla
authoritative_surface: external compatibility and public documentation
create_intent:
- test/contract/ingestion_contract_test.go
- test/contract/legacy_compatibility_test.go
- ADOPTED_COMPONENTS.md
execution_mode: code_change
model: ''
owned_files:
- test/contract/ingestion_contract_test.go
- test/contract/legacy_compatibility_test.go
- cmd/inkbite/main_test.go
- README.md
- INKBITE_SPEC.md
- INKBITE_COMPONENTS_SPEC.md
- skills/inkbite/SKILL.md
- ADOPTED_COMPONENTS.md
role: implementer
tags:
- compatibility
- docs
- licensing
task_type: implement
---

# Work Package Prompt: WP08 – Compatibility, Documentation, and Civic Adoption

## ⚡ Do This First: Load Agent Profile

Load `curator-carla`. Documentation is evidence-bound: every example must compile or derive from a test, and no aspirational provider/OCR/inference behavior may be presented as shipped.

## Objective

Prove external source compatibility and default Markdown CLI behavior; align the public schema/guides with final code; document security/durability boundaries and a respectful adopted-components policy.

## Subtasks

### T033 — External compatibility fixtures

Use an external test package to compile unkeyed/equality/map-key legacy `Result`, a custom legacy `Converter`, registration/options, every legacy Engine entry point, plus additive detailed ingestion and pure verification.

### T034 — Lock CLI behavior

Test default stdout/stderr/exit for success, unsupported, malformed, cancellation, path, and disabled remote. Default output remains Markdown only; no binary/envelope metadata leaks. `cmd/inkbite/main.go` remains frozen.

### T035 — Document API/security/durability

Update README with contract semantics, tested examples, policy defaults, remote/optional-component authority, untrusted-content warning, digest non-authority caveat, and host-owned ingest→verify→persist→discard→reload→verify sequence.

### T036 — Validate contracts and align public specs/skill

Treat the approved JSON schema, public API contract, and quickstart under `kitty-specs/` as immutable test inputs. Add black-box conformance checks against them, then make `INKBITE_SPEC`, the component spec, and the skill match the shipped names/defaults/warnings. Managed components never auto-download in normal conversion. If implementation cannot satisfy an approved contract, reject the owning WP rather than rewriting planning evidence.

### T037 — Civic adoption record

Create `ADOPTED_COMPONENTS.md` template/records with upstream URL, exact revision, adopted files/design, SPDX expression, notice location, local modifications, date, and distribution obligations. Distinguish inspiration, copied code, and dependencies.

## Required Gates

```bash
gofmt -w test/contract/*.go cmd/inkbite/main_test.go
go test ./test/contract ./cmd/inkbite
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Handoff includes compile evidence, CLI snapshots, schema/Go conformance, checked documentation links/examples, adoption rationale, and exact scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
