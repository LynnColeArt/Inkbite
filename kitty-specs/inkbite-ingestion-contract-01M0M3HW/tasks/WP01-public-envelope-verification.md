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
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
- T005
agent: "codex:gpt-5.6-sol:reviewer-renata:reviewer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: ingestion_model.go
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
- 2026-08-22T07:36:46Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Assigned agent via action command
- 2026-08-22T07:55:53Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review: additive v1 envelope/policy, optional DetailedConverter, typed redacted failures, and pure verifier. Red c27b1d7; implementation 658bd0b; hardening f9910d5; legacy reset proof 4de2d89. Go 1.26.6: focused count=100, full normal, focused race count=10, full race, vet, govulncheck (0 reachable), go mod verify, build, root staticcheck, diff/API/result freeze, and owned-file audit pass; coverage 86.29% (258/299). Digest-guard deletion mutation failed source/primary/derivative tests, then restored green. Full staticcheck only reports unrelated pre-existing converters/pdf/pdf_test.go:181 U1000. Ruff diff-scoped: no changed Python files, N/A exit 0. Charter section selector commands returned section-not-found; direct charter rules applied.
- 2026-08-22T07:58:27Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T08:09:21Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T08:11:04Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T08:21:00Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review cycle 2: closed all four canonical blockers. Red 51caec1; green 73dd7e4. Public metadata query/fragment and credential/data-URI/authorization/control sentinels reject with redacted findings; all non-empty storage overlaps reject while adjacent and independent equal bytes pass; v1 accepts 256 artifacts and rejects 257 independent of request policy; ordered schema mirrors cover all four closed enums. Go 1.26.6 focused count=100 and race count=10, full normal/race, vet, build, root staticcheck, govulncheck 0 reachable, go mod verify, gofmt/diff, fixed-base coverage 261/291=89.690722%, three-guard deletion mutation, apidiff no incompatible change, frozen legacy and exact owned-file audits all pass. No changed Python files, so diff-scoped ruff is N/A exit 0. Charter section selector remained unavailable; direct charter quality/review rules applied.
- 2026-08-22T08:21:45Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T08:33:12Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T08:34:19Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T08:51:48Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review cycle 3: red 03ec931; green ce659cf. Closed all five cycle-2 blockers with portable absolute-path rejection across public metadata, absolute v1 32 MiB source/primary/derivative ceilings, canonical UTF-8 primary validation, valid non-self relationship requirements, and recursive percent-decoding traversal/separator rejection with benign controls. Go 1.26.6 exact suite count=100 and race count=10, full normal/race, vet, build, module verify, root staticcheck, govulncheck 0 reachable, gofmt/diff, fixed-base coverage 322/357=90.196078%, five deletion mutations, apidiff, external legacy fixture, schema/JSON/enums mirror, exact eight-file scope, frozen surfaces, and no dependency/license delta all pass. Full staticcheck only reports unchanged pre-existing converters/pdf/pdf_test.go:181 U1000. Ruff diff-scoped N/A: no changed Python files. Charter section selectors unavailable; direct charter quality rules applied.
- 2026-08-22T08:52:32Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T09:05:03Z – user – shell_pid=2667806 – Moved to planned
