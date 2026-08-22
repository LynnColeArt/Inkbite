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
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T033
- T034
- T035
- T036
- T037
agent: "codex:gpt-5.6-sol:reviewer-renata:reviewer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: curator-carla
authoritative_surface: test/contract/
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
- 2026-08-22T14:09:21Z – codex:gpt-5.6-sol:curator-carla:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T14:30:38Z – codex:gpt-5.6-sol:curator-carla:implementer – shell_pid=2667806 – Ready for review: red d71a63d established external legacy compatibility, immutable schema/Go roundtrip, CLI snapshots, documentation vocabulary, and link checks; green a6e960c published evidence-bound API/security/durability/component/adoption docs; 26e961b pinned exact upstream license notices. External consumer compilation proves positional/equality/map-key Result, custom legacy Converter, registration/options, all Convert entry points, additive Ingest, and pure mutation-detecting VerifyEnvelope. CLI snapshots cover exact Markdown success, unsupported, malformed, cancellation, path/output/hints/formats, disabled remote, and no-transport fail-closed behavior without changing frozen main.go/Result/defaults. Gates pass: gofmt; go test contract+CLI; go test ./...; go test -race ./...; go vet ./...; staticcheck ./...; go build ./...; go mod verify; GOTOOLCHAIN=go1.26.6 govulncheck reports 0 reachable vulnerabilities; git diff --check; ruff N/A no Python. Relative links resolve; exact upstream/license URLs returned HTTP 200. Scope exact eight owned files. ADOPTED_COMPONENTS distinguishes inspiration/no copied inventory/direct dependencies and flags xlsReader GPL-3.0-only binary-distribution obligations rather than mislabeling Inkbite MIT-only. Force only acknowledges pre-existing shared status.events ancestry intentionally integrated before WP08; no planning artifact was authored or reverted by WP08.
- 2026-08-22T14:32:29Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T14:38:22Z – user – shell_pid=2667806 – Review passed: red d71a63d reproduced; green a6e960c and license pin 26e961b satisfy external compatibility, exact CLI behavior, immutable schema/API conformance, evidence-bound security and durability documentation, and accurate civic adoption obligations. Integration merge 74aaa8b was audited separately; WP08 scope is exactly the eight authorized files. Force only acknowledges pre-existing same-mission status.events ancestry inherited before WP08; WP08 authored no planning artifact. Full, race, vet, staticcheck, govulncheck, build, module, diff, frozen-surface, cross-platform, coverage, and link gates pass.
