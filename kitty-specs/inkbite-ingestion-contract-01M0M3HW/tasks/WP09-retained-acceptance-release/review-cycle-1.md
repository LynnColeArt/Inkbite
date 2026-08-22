---
affected_files:
  - scripts/verify-ingestion-contract.sh
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: GOTOOLCHAIN=go1.26.6 make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP09
---

# WP09 Review Cycle 1 — Changes Required

## Blocking issue — the mandatory quality gate rejects the canonical review workspace's pre-existing lock as its own mutation

The independently executed terminal sequence stopped at mandatory command 5, exactly as the WP requires. Commands 1–4 passed once, in order. The sole invocation of:

```bash
GOTOOLCHAIN=go1.26.6 make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342
```

completed all substantive checks and then exited 2 at `scripts/verify-ingestion-contract.sh`'s final absolute-cleanliness check:

```text
quality gate changed the frozen worktree:
?? .spec-kitty/
make: *** [Makefile:28: quality] Error 1
```

The only reported path was `.spec-kitty/review-lock.json`. It existed before mandatory command 1 and is created by the supported `spec-kitty agent action review` workflow to retain the active WP09 review claim. The quality command did not create or modify it. Nevertheless, `run_quality` checks whether final `git status --porcelain` is non-empty rather than whether the quality run changed the pre-existing state, making the mandatory gate fail inside its canonical independent-review environment.

Per T042 and the explicit reviewer instruction, command 5 was not retried or suppressed, no file was edited, and mandatory commands 6–9 were not run. A required terminal gate failure blocks approval even though its earlier subchecks were green.

Required remediation:

1. Make frozen-worktree mutation detection distinguish pre-existing, explicitly recognized runtime coordination state from changes caused by the quality command. A robust approach is to capture and validate the pre-run state, require no tracked/index dirt, and compare the complete post-run state against that baseline so newly generated residue still fails.
2. Add a shell/acceptance regression proving an existing `.spec-kitty/review-lock.json` survives unchanged and does not fail quality, while a new or modified tracked/untracked artifact created during quality still fails.
3. Re-freeze the corrected tree and run mandatory commands 1–9 once, in order, with no retry. Preserve this failed transcript as historical evidence.

## Mandatory sequence transcript

1. PASS — `go test ./test/acceptance -count=10` (`0.899s`).
2. PASS — `go test -race ./test/acceptance -count=3` (`2.397s`).
3. PASS — `go test ./...`.
4. PASS — `go test -race ./...`.
5. FAIL — `make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342`; final status check rejected the pre-existing review lock.
6. NOT RUN — stopped after the first failure.
7. NOT RUN — stopped after the first failure.
8. NOT RUN — stopped after the first failure.
9. NOT RUN — stopped after the first failure.

Command 5's completed subchecks reported Go 1.26.6, staticcheck 2026.1/v0.7.0, govulncheck v1.6.0, git 2.43.0, zero reachable vulnerabilities, module/API/license/CLI compatibility, mutation self-test red `1/5` and green `5/5`, changed-production coverage `1815/2053 = 88.407209%` unrounded, and autocrlf fixture `587` bytes with SHA-256 `0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b`. The two package builds compared equal within the invocation and printed manifest SHA-256 `3f67da142a412498217064d5806fafbc1a41aea6b1347527ea615ab07856c566`; this differs from the implementation handoff's `1a861c2` hash because the active review tip contains later coordination-only ancestry.

## Independently verified retained evidence

- Final product commit `1a861c2ca88f9560b36384c810191357aa8e6735` is an ancestor of the review tip. The intervening tree changes are only WP09 status/event/task coordination files; product and release files remain frozen.
- WP09's product scope is exactly the seven owned files: three acceptance files, two scripts, `Makefile`, and `.github/workflows/ci.yml`. Root-owned `a7365de` is separately confined to seven planning/review Markdown whitespace fixes and contains no product change.
- The retained host journey uses exported APIs, writes a byte-free JSON manifest plus source/primary/derivative object files, then drops source bytes, envelope, engine, and disposable source storage before fresh disk reads. Reloaded text and PDF envelopes reverify all lengths, identities, output digests, and relations.
- The 100-run text/PDF/office/nested-ZIP reproducibility matrix, 100 concurrent isolation matrix, all retained-byte mutations, missing/duplicate/cross-envelope cases, all source/output/container at-limit/+1 boundaries, remote/address/redirect rules, zero hidden network/model/component/download effects, cancellation, and diagnostic redaction use live public paths.
- Preserved failures remain causal evidence: the inherited whitespace freeze was repaired separately by root-owned `a7365de`; `3e042b9` captures the escaped `RETURN` trap; `1a861c2` structurally confines cleanup to a subshell/`EXIT` trap, and the retained acceptance test exercises that corrected path.
- All eight prerequisite WPs are approved in authoritative status, including retained arbiter evidence where ordinary cycles required it.

## WP anti-pattern checklist

1. Dead code — PASS. Make/script entry points are invoked by CI and local targets; acceptance tests exercise exported production APIs.
2. Synthetic fixture — PASS. Retained, authority, cancellation, concurrency, and mutation scenarios use live engine/converter/verifier paths and real disk I/O.
3. Silent empty return — PASS. Gate failures are explicit and stop the sequence; runtime failure paths return typed zero-envelope outcomes.
4. FR coverage — PASS. The aggregate acceptance matrix has observable assertions spanning FR-001 through FR-017.
5. Frozen surface — PASS. The seven-file WP scope is authorized; root-owned planning whitespace hygiene is audited separately.
6. Locked decision — PASS. No persistence ownership, network/component authority, release publication, retry, or failure suppression was introduced.
7. Shared-file ownership — PASS. The root-owned planning hygiene commit is isolated and explicitly recorded; WP09 product scope has no unapproved crossing.
8. Production fragility — PASS. The escaped cleanup trap is structurally confined and mutation-protected; the remaining defect is the gate's baseline comparison.

## Re-review acceptance

The corrected frozen tree must allow the canonical review lock without masking any new worktree mutation. Independently execute commands 1–9 once in order; all must pass on the unchanged tree, and commands 6–9 must have fresh direct evidence rather than inherited handoff claims.
