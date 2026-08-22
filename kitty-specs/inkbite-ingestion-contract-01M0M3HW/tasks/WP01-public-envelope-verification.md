---
work_package_id: WP01
title: Public Envelope and Pure Verification
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-010
- FR-011
- FR-012
- FR-014
- FR-015
- FR-016
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Spec Kitty allocates the lane from the planning branch; merge completed work only to feat/inkbite-ingestion-contract before mission acceptance.
subtasks:
- T001
- T002
- T003
- T004
- T005
agent: codex
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: public ingestion values and verification
create_intent:
- ingestion_model.go
- ingestion_policy.go
- ingestion_verify.go
- ingestion_model_test.go
- ingestion_policy_test.go
- ingestion_verify_test.go
execution_mode: code_change
model: ''
owned_files:
- converter.go
- errors.go
- ingestion_model.go
- ingestion_policy.go
- ingestion_verify.go
- ingestion_model_test.go
- ingestion_policy_test.go
- ingestion_verify_test.go
role: implementer
tags:
- contract
- integrity
- compatibility
task_type: implement
---

# Work Package Prompt: WP01 – Public Envelope and Pure Verification

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan` with `/ad-hoc-profile-load`. Read the full spec, plan, data model, research, quickstart, and both contract files. Freeze `result.go`; do not add methods to legacy `Converter`.

## Objective

Create additive `inkbite.ingestion/v1` owned values, strict policy defaults, stable typed outcomes, an optional detailed-converter capability, and a pure public verifier. Preserve legacy comparability and source compatibility.

## Invariants

- Identity is `sha256:<64 lowercase hex>` over exact bytes.
- Canonical provenance excludes time, absolute paths, addresses, and map iteration.
- Returned bytes never alias caller or scratch buffers.
- Verification performs zero I/O, conversion, network, component, model, subprocess, clock, or persistence work.
- Digest equality proves integrity, not origin or authority.

## Subtasks

### T001 — Model the v1 contract

Red-first tests define source, primary Markdown, derivatives, stable IDs/relations, metadata facts/origins, warnings, converter/backend/component provenance, and effective policy. Use ordered slices where canonical order matters.

### T002 — Add an optional detailed capability

Define a narrow `DetailedConverter`/detailed result interface without modifying the existing interface. Old converters must compile untouched and retain priority/reset semantics.

### T003 — Add typed public failures

Distinguish unsupported, malformed, limit, policy, integrity, cancellation, and converter failure with `errors.Is`/`errors.As`. Public formatting must never expose source bytes, credentials, grants, backend stack traces, or data-URI payloads.

### T004 — Implement pure verification

Validate contract version, policy, identities, lengths, ordering, uniqueness, references, relationships, warnings, provenance, and byte ownership. Reject one-byte mutation, missing/duplicate objects, invalid references, cross-envelope substitution, and noncanonical identities.

### T005 — Prove compatibility and determinism

Test legacy unkeyed/equality/map-key `Result` use indirectly by ensuring it remains untouched; prove old custom converters compile; run 100 canonical serializations; mutate caller/result buffers; execute concurrent construction/verification under race.

## Required Evidence

- Red commit demonstrating missing contract/verifier.
- Golden schema/JSON round trip and independent SHA-256 recomputation.
- Zero-effect verifier with counting fakes.
- Exact exported API diff and owned-file audit.

## Required Gates

```bash
gofmt -w converter.go errors.go ingestion_*.go
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Do not merge/push or alter files outside ownership. Transition through supported Spec Kitty commands only after final bytes pass.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
