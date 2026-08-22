# Tasks: Versioned Retained Ingestion Contract

## Delivery Strategy

Nine bounded packages establish the value contract before effects: public envelope and pure verification; bounded primitives; source/remote admission; one engine pipeline; generic ZIP/EPUB accounting; OOXML/XLSX accounting; PDF derivatives; compatibility/documentation/civic adoption; and terminal retained-ingestion acceptance plus release qualification.

All planning artifacts were generated on `feat/inkbite-ingestion-contract`. Spec Kitty owns lane allocation and dependency merges. Completed packages merge only to that mission branch until mission acceptance. `main` changes only through the later reviewed pull request.

`result.go`, `cmd/inkbite/main.go`, `builtins/defaults.go`, and unrelated converters are frozen. Any indispensable edit to a frozen or out-of-map file requires an explicit pre-edit exception and amended ownership.

## Branch and Review Rules

- Use only the lane/worktree returned by Spec Kitty; never pick a base manually.
- Production ownership is mutually exclusive. Aggregate packages inspect prior surfaces but edit only declared tests/docs/gates.
- Every code WP starts with a deterministic red test and receives independent review on final bytes.
- Preserve first failures; do not rerun a governed stability gate until a structural correction is committed.
- No WP may merge, push, publish, tag, or mutate remotes.

## Test Strategy

- Exact `limit` and `limit+1` tests for acquisition, outputs, artifacts, and containers.
- One-byte mutation/deletion/substitution tests for every byte-bearing object and relationship.
- Hostile path, duplicate, checksum, expansion, redirect, address-class, secret-redaction, and cancellation matrices.
- Deterministic fixtures/clocks and 100-run canonical/reentrant/concurrent tests.
- Full normal/race, vet, staticcheck, vulnerability, module, license/adoption, API compatibility, fixed-base changed coverage >=80.0% unrounded, cross-platform, binary fixture, and package reproducibility gates.

## Global Definition of Done

- [ ] `inkbite.ingestion/v1` is additive, deterministic, independently verifiable, and owns its bytes.
- [ ] Legacy `Result`, `Converter`, Engine methods, priorities, and default Markdown CLI remain source/behavior compatible.
- [ ] Every source is bounded before dispatch; remote authority is explicit and fail closed.
- [ ] ZIP, EPUB, OOXML, and XLSX share request-local actual-byte accounting.
- [ ] PDF images are first-class detailed artifacts without changing legacy inline/default behavior.
- [ ] Degradation is visible; no authoritative success silently omits selected work.
- [ ] Normal conversion performs no hidden inference, download, subprocess, or persistence action.
- [ ] A host can verify, persist, discard state, reload, and reverify exact values.
- [ ] All FR/NFR/constraints/success criteria and release gates pass on frozen final bytes.

## Requirement Coverage

| Requirement | Work packages |
|---|---|
| FR-001 | WP01, WP04, WP09 |
| FR-002 | WP01, WP02, WP03, WP04, WP09 |
| FR-003 | WP01, WP04, WP07, WP09 |
| FR-004 | WP01, WP04, WP07, WP09 |
| FR-005 | WP01, WP03, WP04, WP09 |
| FR-006 | WP01, WP04, WP05, WP06, WP07, WP09 |
| FR-007 | WP02, WP03, WP04, WP09 |
| FR-008 | WP02, WP05, WP06, WP09 |
| FR-009 | WP03, WP09 |
| FR-010 | WP01, WP04, WP07, WP08, WP09 |
| FR-011 | WP01, WP02, WP03, WP04, WP05, WP06, WP09 |
| FR-012 | WP01, WP04, WP08, WP09 |
| FR-013 | WP08, WP09 |
| FR-014 | WP01, WP04, WP08, WP09 |
| FR-015 | WP01, WP02, WP04, WP05, WP06, WP07, WP09 |
| FR-016 | WP01, WP02, WP04, WP05, WP06, WP07, WP09 |
| FR-017 | WP01, WP04, WP09 |

## Implementation Concern Mapping

| Plan concern | Work packages |
|---|---|
| IC-01 — Additive envelope and verification | WP01, WP04, WP09 |
| IC-02 — Uniform bounded acquisition and remote authority | WP02, WP03, WP04, WP09 |
| IC-03 — Shared container accounting | WP02, WP05, WP06, WP09 |
| IC-04 — First-class PDF derivatives | WP07, WP09 |
| IC-05 — Compatibility, documentation, and civic adoption | WP08, WP09 |
| IC-06 — Aggregate retained-ingestion acceptance | WP09 |

## Subtask Index

| ID | Description | WP | Parallel |
|---|---|---|---|
| T001 | Specify v1 envelope, artifacts, relations, provenance, warnings, policy values | WP01 | No |
| T002 | Add optional detailed-converter capability without changing Converter | WP01 | Yes |
| T003 | Add stable typed public failures and safe formatting | WP01 | Yes |
| T004 | Implement pure structural/content verification | WP01 | No |
| T005 | Prove canonical order, owned bytes, mutation denial, legacy compatibility | WP01 | No |
| T006 | Build bounded read, clone, digest, and limit-plus-one primitives | WP02 | No |
| T007 | Build request-local source/output/artifact/container budgets | WP02 | No |
| T008 | Build safe name/location/metadata canonicalization | WP02 | Yes |
| T009 | Build cancellation checkpoints and central-guard mutation tests | WP02 | Yes |
| T010 | Bound bytes, reader, path, file URI, and data URI acquisition | WP03 | No |
| T011 | Return exact owned source facts and origin metadata | WP03 | No |
| T012 | Prove disabled remote authority makes zero calls | WP03 | Yes |
| T013 | Implement redirect-safe destination admission and pinned dial | WP03 | No |
| T014 | Bound remote bodies and redact remote diagnostics | WP03 | Yes |
| T015 | Compose one detailed and legacy engine pipeline | WP04 | No |
| T016 | Preserve metadata precedence and ordered converter attempts | WP04 | Yes |
| T017 | Seal/self-verify envelopes before success | WP04 | No |
| T018 | Project legacy Result from the same pipeline | WP04 | No |
| T019 | Prove cancellation, fallback, determinism, aliasing, concurrency | WP04 | No |
| T020 | Route generic ZIP through the shared request ledger | WP05 | No |
| T021 | Preserve one ledger through nested ZIP conversion | WP05 | No |
| T022 | Make unsupported/failed ZIP members visible | WP05 | Yes |
| T023 | Route EPUB package/member reads through the shared ledger | WP05 | Yes |
| T024 | Prove hostile container boundaries and legacy fidelity | WP05 | No |
| T025 | Make OOXML package loading policy/path/checksum safe | WP06 | No |
| T026 | Adopt bounded package loading in DOCX and PPTX | WP06 | Yes |
| T027 | Preflight XLSX before third-party expansion | WP06 | No |
| T028 | Prove cross-format boundaries, cancellation, and fidelity | WP06 | No |
| T029 | Return ordered PDF image artifacts with safe facts | WP07 | No |
| T030 | Emit deterministic artifact references/occurrences | WP07 | No |
| T031 | Enforce artifact/output limits and visible degradation | WP07 | Yes |
| T032 | Preserve legacy PDF and prove mutation/concurrency behavior | WP07 | No |
| T033 | Add external legacy API/custom-converter compatibility fixtures | WP08 | No |
| T034 | Lock default Markdown CLI behavior | WP08 | No |
| T035 | Document detailed API, security, and durability handoff | WP08 | Yes |
| T036 | Validate shipped behavior against approved contracts and align public specs/skill | WP08 | Yes |
| T037 | Publish civic adopted-components records | WP08 | No |
| T038 | Build public verify-persist-discard-reload acceptance | WP09 | No |
| T039 | Run reproducibility, mutation, policy, zero-effect, concurrency matrices | WP09 | No |
| T040 | Install fixed-base quality/API/license/security gates | WP09 | No |
| T041 | Qualify Linux/macOS/Windows/race/vulnerability/package CI | WP09 | Yes |
| T042 | Run and record the terminal frozen-tree gate matrix | WP09 | No |

## Work Packages

## WP01 — Public Envelope and Pure Verification

**Prompt**: `tasks/WP01-public-envelope-verification.md`
**Priority**: P0
**Dependencies**: none
**Independent test**: External-style legacy code compiles while deterministic v1 values and a zero-effect verifier reject every mutation/reference defect.

- [x] T001 Specify v1 envelope, artifacts, relations, provenance, warnings, policy values (WP01)
- [x] T002 Add optional detailed-converter capability without changing Converter (WP01)
- [x] T003 Add stable typed public failures and safe formatting (WP01)
- [x] T004 Implement pure structural/content verification (WP01)
- [x] T005 Prove canonical order, owned bytes, mutation denial, legacy compatibility (WP01)

## WP02 — Bounded Ingestion Primitives

**Prompt**: `tasks/WP02-bounded-ingestion-primitives.md`
**Priority**: P0
**Dependencies**: WP01

- [x] T006 Build bounded read, clone, digest, and limit-plus-one primitives (WP02)
- [x] T007 Build request-local source/output/artifact/container budgets (WP02)
- [x] T008 Build safe name/location/metadata canonicalization (WP02)
- [x] T009 Build cancellation checkpoints and central-guard mutation tests (WP02)

## WP03 — Source Acquisition and Remote Authority

**Prompt**: `tasks/WP03-source-acquisition-remote-authority.md`
**Priority**: P0
**Dependencies**: WP01, WP02

- [x] T010 Bound bytes, reader, path, file URI, and data URI acquisition (WP03)
- [x] T011 Return exact owned source facts and origin metadata (WP03)
- [x] T012 Prove disabled remote authority makes zero calls (WP03)
- [x] T013 Implement redirect-safe destination admission and pinned dial (WP03)
- [x] T014 Bound remote bodies and redact remote diagnostics (WP03)

## WP04 — Single Engine Pipeline and Legacy Projection

**Prompt**: `tasks/WP04-engine-envelope-legacy-projection.md`
**Priority**: P0
**Dependencies**: WP01, WP02, WP03

- [x] T015 Compose one detailed and legacy engine pipeline (WP04)
- [x] T016 Preserve metadata precedence and ordered converter attempts (WP04)
- [x] T017 Seal/self-verify envelopes before success (WP04)
- [x] T018 Project legacy Result from the same pipeline (WP04)
- [x] T019 Prove cancellation, fallback, determinism, aliasing, concurrency (WP04)

## WP05 — Generic ZIP and EPUB Accounting

**Prompt**: `tasks/WP05-zip-epub-accounting.md`
**Priority**: P0
**Dependencies**: WP02, WP04

- [ ] T020 Route generic ZIP through the shared request ledger (WP05)
- [ ] T021 Preserve one ledger through nested ZIP conversion (WP05)
- [ ] T022 Make unsupported/failed ZIP members visible (WP05)
- [ ] T023 Route EPUB package/member reads through the shared ledger (WP05)
- [ ] T024 Prove hostile container boundaries and legacy fidelity (WP05)

## WP06 — OOXML and XLSX Accounting

**Prompt**: `tasks/WP06-ooxml-xlsx-accounting.md`
**Priority**: P0
**Dependencies**: WP02, WP04

- [ ] T025 Make OOXML package loading policy/path/checksum safe (WP06)
- [ ] T026 Adopt bounded package loading in DOCX and PPTX (WP06)
- [ ] T027 Preflight XLSX before third-party expansion (WP06)
- [ ] T028 Prove cross-format boundaries, cancellation, and fidelity (WP06)

## WP07 — First-Class PDF Derivatives

**Prompt**: `tasks/WP07-pdf-derived-artifacts.md`
**Priority**: P1
**Dependencies**: WP01, WP02, WP04

- [ ] T029 Return ordered PDF image artifacts with safe facts (WP07)
- [ ] T030 Emit deterministic artifact references/occurrences (WP07)
- [ ] T031 Enforce artifact/output limits and visible degradation (WP07)
- [ ] T032 Preserve legacy PDF and prove mutation/concurrency behavior (WP07)

## WP08 — Compatibility, Documentation, and Civic Adoption

**Prompt**: `tasks/WP08-compatibility-docs-adoption.md`
**Priority**: P1
**Dependencies**: WP03, WP04, WP05, WP06, WP07

- [ ] T033 Add external legacy API/custom-converter compatibility fixtures (WP08)
- [ ] T034 Lock default Markdown CLI behavior (WP08)
- [ ] T035 Document detailed API, security, and durability handoff (WP08)
- [ ] T036 Validate shipped behavior against approved contracts and align public specs/skill (WP08)
- [ ] T037 Publish civic adopted-components records (WP08)

## WP09 — Retained Acceptance and Release Qualification

**Prompt**: `tasks/WP09-retained-acceptance-release.md`
**Priority**: P0
**Dependencies**: WP01, WP02, WP03, WP04, WP05, WP06, WP07, WP08

- [ ] T038 Build public verify-persist-discard-reload acceptance (WP09)
- [ ] T039 Run reproducibility, mutation, policy, zero-effect, concurrency matrices (WP09)
- [ ] T040 Install fixed-base quality/API/license/security gates (WP09)
- [ ] T041 Qualify Linux/macOS/Windows/race/vulnerability/package CI (WP09)
- [ ] T042 Run and record the terminal frozen-tree gate matrix (WP09)
