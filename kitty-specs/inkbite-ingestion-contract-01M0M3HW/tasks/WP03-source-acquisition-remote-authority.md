---
work_package_id: WP03
title: Source Acquisition and Remote Authority
dependencies:
- WP01
- WP02
requirement_refs:
- FR-002
- FR-007
- FR-009
- FR-011
tracker_refs: []
planning_base_branch: feat/inkbite-ingestion-contract
merge_target_branch: feat/inkbite-ingestion-contract
branch_strategy: Planning artifacts for this mission were generated on feat/inkbite-ingestion-contract. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/inkbite-ingestion-contract unless the human explicitly redirects the landing branch.
subtasks:
- T010
- T011
- T012
- T013
- T014
agent: "codex:gpt-5.6-sol:reviewer-renata:reviewer"
shell_pid: "2667806"
history:
- at: '2026-08-22T00:00:00Z'
  actor: codex
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: source.go
create_intent:
- internal/ingestion/remote.go
- internal/ingestion/remote_test.go
execution_mode: code_change
model: ''
owned_files:
- source.go
- source_test.go
- options.go
- internal/ingestion/remote.go
- internal/ingestion/remote_test.go
role: implementer
tags:
- source
- http
- ssrf
task_type: implement
---

# Work Package Prompt: WP03 – Source Acquisition and Remote Authority

## ⚡ Do This First: Load Agent Profile

Load `implementer-ivan`. Read WP01/WP02 final contracts. Do not edit Engine or converter code.

## Objective

Apply the same bounded acquisition to bytes, readers, paths, file URIs, data URIs, and explicit HTTP(S), returning exact owned source bytes and safe fact origins before any converter dispatch.

## Trust Rules

- HTTP is off by default and must make zero resolver/dial/transport calls.
- Default explicit remote mode accepts HTTP(S), rejects userinfo, ambient proxy use, transparent decompression, special destinations, and unsafe redirects.
- Every redirect is re-admitted; approved resolved IP is the connection target while original hostname remains TLS/SNI authority.
- Reject IANA non-global classes including loopback/private/link-local/unspecified/multicast/documentation and IPv4-mapped bypasses; do not rely solely on `IsPrivate`/`IsGlobalUnicast`.
- Custom clients are explicit trusted capabilities and appear in provenance.

## Subtasks

### T010 — Bound local source forms

Write zero-dispatch at-limit/+1 tests for bytes, reader/seeker, path, file URI, and data URI. Ensure cooperative blocking readers cancel and join within one second with no partial source. Keep an arbitrary caller-owned non-cooperative `io.Reader` or `io.ReadSeeker` synchronously joined until its in-flight `Read` or `Seek` returns, then observe cancellation at the next checkpoint, return typed cancellation with no partial source, and never abandon an Inkbite-owned worker.

### T011 — Return exact owned source facts

Copy acquired bytes, compute identity/length, sanitize source kind/display, and preserve caller/source/sniff fact origins without storing data payloads or full local paths.

### T012 — Prove remote-disabled zero authority

Counting resolver/dial/redirect/transport fakes must remain zero on HTTP(S) success-like, malformed, fallback, and error paths when authority is absent.

### T013 — Implement admitted redirects and pinned dialing

Test schemes, userinfo, IPv4/IPv6/mapped addresses, mixed DNS answers, rebinding, redirect chains/caps, proxy environment, TLS hostname, and header stripping. Dial only an admitted address.

### T014 — Bound bodies and sanitize failures

Treat Content-Length as early hint; stream limit+1. Disable implicit content decoding where exact received representation bytes are promised. Redact credentials/query tokens/authorization/body/backend text and join cancellation.

## Required Gates

```bash
gofmt -w source.go source_test.go options.go internal/ingestion/remote*.go
go test ./internal/ingestion . -run 'Source|Remote|HTTP|URI' -count=20
go test -race ./internal/ingestion . -run 'Source|Remote|HTTP|URI' -count=5
go test ./...
go vet ./...
git diff --check
```

Handoff includes zero-call evidence, denied-address matrix, redirect/DNS pin proof, cancellation/redaction sentinels, and exact scope.

## Activity Log

- 2026-08-22 — Prompt generated from the approved mission artifacts.
- 2026-08-22T10:36:09Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Assigned agent via action command
- 2026-08-22T10:58:46Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Ready for review: red e62f77c; green dc61faf/1122927/f5c8a49; exact five-file scope; Go 1.26.6 focused 20x/race 5x and full normal+race/vet/build/mod/vuln pass; coverage root 86.3% internal/ingestion 89.5%; disabled zero-call, denied-address, redirect/DNS pin, exact representation, cancellation/redaction, concurrency, and guard-mutation evidence green.
- 2026-08-22T10:59:51Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T11:11:09Z – user – shell_pid=2667806 – Moved to planned
- 2026-08-22T11:13:09Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Started implementation via action command
- 2026-08-22T11:36:10Z – codex:gpt-5.6-sol:implementer-ivan:implementer – shell_pid=2667806 – Cycle 2 ready under ruling 57bc26c and amendment 9929836: amendment preserved through non-rewriting merge 5ef225f; IANA red/green d9d48d9/9d1fe38; cancellation red/green 4866c4b/108130f plus deterministic/no-detached guards 23af83f/0d8ce9f; product scope source.go, source_test.go, internal/ingestion/remote.go, internal/ingestion/remote_test.go; all required gates green. Force is limited to the required planning-amendment ancestry already present on this dedicated lane.
- 2026-08-22T11:37:04Z – codex:gpt-5.6-sol:reviewer-renata:reviewer – shell_pid=2667806 – Started review via action command
- 2026-08-22T11:43:41Z – user – shell_pid=2667806 – Cycle 2 independent approval supersedes retained cycle-1 rejection under binding ruling 57bc26c. Narrow force is solely for required planning amendment 9929836 preserved through non-rewriting merge 5ef225f on dedicated lane-c; no out-of-scope product file is authorized. IANA current-registry longest-prefix mirror, both sentinels, synchronized joined cancellation, no-sleep/no-detached-worker, count100/race10/full/vet/build/module/vulnerability/coverage/API/frozen/scope/mutation gates passed; staticcheck only inherited PDF-test U1000. Anti-patterns: dead code PASS; synthetic fixtures PASS; silent empty return PASS; FR coverage PASS; frozen surface PASS; locked decisions PASS; shared ownership PASS; production fragility N/A.
