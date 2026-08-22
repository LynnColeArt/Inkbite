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
agent: "codex:gpt-5.6-sol:reviewer-renata:reviewer"
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
- 2026-08-22T13:36:55Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Cycle 2 ready: red 2c00adb; green 0d560dc; reference hardening 840b419. Engine is the sole canonical artifact-ID authority and rewrites converter-local detailed ordinals after final relationship-aware ordering; mixed JPEG/PNG and repeated-identical-byte production fixtures resolve visible MIME/page/object/dimensions/occurrences exactly, while legacy Convert remains literal. Go 1.26.6 PDF count20/race5, full normal/race, vet, build, module tidy-diff/verify, staticcheck v0.8.1, govuln 0 reachable, CLI count20, Windows/Darwin compile, diff/API/frozen/scope, 81.5% PDF raw and 1391/1530=90.915033% fixed-base coverage pass; raw-index mutation fails and restores. Cycle-2 scope exactly converters/pdf/{pdf.go,detailed_test.go,artifact_limits_test.go} plus previously authorized ingestion.go; clean lane, no residue.
- 2026-08-22T13:39:16Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T13:45:28Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T13:46:49Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T13:50:14Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Cycle 3 ready: red e688ac0; green 822a590. Explicit exact-token grammar accepts end-of-input and generated/legitimate Markdown delimiters, while public Ingest rejects path, query, fragment, percent-encoded, colon, backslash, authority-like, punctuation, alphanumeric, and Unicode continuations with ErrIntegrityFailure and zero envelopes; legacy literal Markdown and cycle1/2 canonical mapping remain exact. Default-accept mutation makes all continuation cases red and restores clean. Go 1.26.6 PDF count20/race5, focused count100, full normal/race, vet/build, mod verify/tidy-diff, staticcheck v0.8.1, govuln 0 reachable, CLI count20, Windows/Darwin compile, diff/API/frozen pass. PDF raw 81.5%; immutable-base 1393/1532=90.926893%; cycle3 6/6=100%. Exact cycle3 scope ingestion.go plus owned converters/pdf/artifact_limits_test.go; fixture SHA-256 0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b; lane clean.
- 2026-08-22T13:50:50Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T13:54:57Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T13:57:03Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T14:02:47Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – terminal arbiter correction — exact artifact token boundaries. Red ccaef26; green 50b5f24. Public Ingest with real mixed JPEG/PNG artifacts proves embedded identifiers, longer URI/path substrings, and adjacent prose remain byte-exact; generated parenthesized destinations, EOF, all bound openers/closers, and multiple references resolve through engine-only canonical ID assignment; malformed references in valid contexts fail ErrIntegrityFailure with zero envelopes; legacy direct/engine/CLI behavior remains exact. Start-default-accept and end-default-accept mutations fail independently and restore. Go 1.26.6 focused count100/race10, PDF count20/race5, full normal/race, vet/build, mod verify/tidy-diff, staticcheck v0.8.1, govuln 0 reachable, CLI count20, Windows/Darwin compile, formatting/diff/API/frozen/no-authority pass. PDF raw 81.5%; fixed-base 1402/1541=90.979883%; terminal 22/22=100%. Exact scope ingestion.go + converters/pdf/artifact_limits_test.go. Final tree 6e7f8ba8e9bafc09013146cb1c52d7a6ec9066a2; fixture 0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b; lane clean.
- 2026-08-22T14:03:33Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
