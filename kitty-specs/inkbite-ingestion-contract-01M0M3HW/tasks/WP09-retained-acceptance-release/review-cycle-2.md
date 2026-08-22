---
affected_files: []
cycle_number: 2
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command:
reviewed_at: '2026-08-22T15:37:35Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP09
---

---
affected_files:
  - scripts/verify-ingestion-contract.sh
  - .github/workflows/ci.yml
cycle_number: 2
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command: make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342
reviewer_agent: codex:gpt-5.6-sol:reviewer-renata:reviewer
verdict: rejected
wp_id: WP09
---

# WP09 Review Cycle 2 — Changes Required

## Blocking issue — the release package gate certifies an explicitly unresolved license-distribution gap

The cycle-1 review-lock correction is correct and causal. Red `8c82a80`
rejects the unchanged canonical review lock while accepting created and
unrelated dirt. Green `3dc3118` permits only the exact pre-existing regular
`.spec-kitty/review-lock.json` while its porcelain status and SHA-256 remain
unchanged. Independent real-worktree probes rejected creation, deletion,
content change, file or directory symlink replacement, additional
`.spec-kitty` entries, and unrelated dirt. Replacing the final lock fingerprint
with the baseline fingerprint made the modified-lock regression test fail.

The release-license claim remains false, however. `ADOPTED_COMPONENTS.md`
records that the shipped binary links `github.com/shakinm/xlsReader` under
`GPL-3.0-only`, that the full GPL text and Corresponding Source material are
not currently packaged, and that release packaging requires resolution before
distribution. It also records notice/license obligations for the other direct
dependencies and says transitive dependency licenses still require inventory.

Despite that retained warning, `build_packages` copies only `README.md`,
`CHANGELOG.md`, Inkbite's MIT `LICENSE`, and `ADOPTED_COMPONENTS.md` beside each
linked binary. It does not assemble the GPLv3 license and Corresponding Source,
the Apache-2.0 license/NOTICE material, BSD/MIT notices, or a complete
transitive license inventory. `.github/workflows/ci.yml` then names these files
release archives and uploads them after the current gate passes.

`verify_license_inventory` cannot detect this gap. It only greps the root MIT
license, the string `GPL-3.0-only`, and direct module names/versions in the
adoption record. It never inspects archive contents or proves that the recorded
distribution obligations have been satisfied. The gate therefore prints
`MIT/adopted-component direct dependency inventory: pass` while the source of
truth says release packaging is unresolved. This violates WP09's explicit
requirements that known licensing gaps be zero and that release qualification
report no licensing violation.

Required remediation:

1. Choose and document a distribution strategy for the GPL-linked XLS path:
   either exclude/separate that dependency from the distributed binary through
   a technically enforced build boundary, or package the combined work under a
   compliant GPLv3 distribution with the required license, notices,
   Corresponding Source, and installation information where applicable.
2. Produce and package complete direct and transitive dependency license/notice
   material, including Apache, BSD, MIT, and GPL obligations recorded by the
   civic-adoption audit.
3. Make the license/package gate inspect the built archive and fail when any
   selected dependency's required material or the chosen GPL distribution
   boundary is absent. A grep that merely finds an obligation in Markdown is
   not closure evidence.
4. Keep CI artifact upload behind that substantive gate, freeze the corrected
   tree, and rerun the mandatory nine commands once in order without retry.

## Cycle-2 frozen-sequence evidence

Exact committed review HEAD:
`022a4dbaa3e771690aaf1f992c9fded3b310b104`.

The pre-existing lock remained the sole dirty entry and retained SHA-256
`16f7bc6a10ee686d9aaf65a70777c1df259119b08598a4fa9c927aa36c2eb980`.

1. PASS — `go test ./test/acceptance -count=10` (`2.143s`).
2. PASS — `go test -race ./test/acceptance -count=3` (`2.773s`).
3. PASS — `go test ./...`.
4. PASS — `go test -race ./...`.
5. PASS mechanically — `make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342`; this is the gate with the false-positive license assertion described above.
6. PASS — `go build ./...`.
7. PASS — `go mod verify`.
8. PASS — `govulncheck ./...` (zero reachable vulnerabilities).
9. PASS — `git diff --check ee5542edd1ac64b5f66dcb9d0056dd4815739342..HEAD`.

Command 5 reported Go 1.26.6, staticcheck 2026.1/v0.7.0,
govulncheck v1.6.0, Git 2.43.0, changed-production coverage
`1815/2053 = 88.407209%` unrounded, mutation self-test red `1/5` and green
`5/5`, PDF fixture 587 bytes with SHA-256
`0c839d2bbb8c86f4a4ceb48706070efaed8c9880d15dd7a4b815b6de2b63a23b`,
and twice-built package manifest SHA-256
`4a47a3fc09a4264dc4c637dc3415ea5d474bf060cf8d07aa4a5f7858ca476d31`.

## Independently retained evidence

- The exported-API host journey persists source, primary Markdown, PDF
  derivative, and byte-free envelope metadata to separate disk objects; drops
  source bytes, envelope, engine, and disposable source state; reloads through
  fresh reads; and re-verifies the reconstructed envelope.
- Reproducibility, concurrency, mutations, boundaries, remote authority,
  cancellation, secret redaction, and zero model/component/download effects
  execute live production paths.
- All eight prerequisite WPs are approved in authoritative status.
- Cycle-2 correction scope is exactly
  `scripts/verify-ingestion-contract.sh` and
  `test/acceptance/security_boundaries_test.go`. Aggregate WP09 product scope
  remains exactly its seven authorized files. Root-owned `a7365de` is confined
  to seven planning/review Markdown whitespace repairs; post-green ancestry is
  mission status/task metadata only.

## WP anti-pattern checklist

1. Dead code — PASS. Script and Make/CI entry points have live callers.
2. Synthetic fixture — PASS. Acceptance scenarios invoke exported production paths and real disk I/O.
3. Silent empty return — PASS. Gate and runtime failures are explicit.
4. FR coverage — PASS. The retained and aggregate suites exercise FR-001 through FR-017.
5. Frozen surface — PASS. Product scope is exactly the seven authorized files; correction scope is two of them.
6. Locked decision — FAIL. A release archive is mechanically approved despite the adopted-components authority explicitly retaining unresolved distribution obligations.
7. Shared-file ownership — PASS. Planning hygiene and coordination-only ancestry are separately scoped and recorded.
8. Production fragility — PASS. Cleanup and cleanliness failures are fail-loud and mutation-protected.

## Re-review acceptance

Preserve both earlier rejection artifacts. The next cycle must demonstrate a
substantive archive-license gate and a technically coherent distribution
strategy, then run the frozen mandatory sequence once on the corrected exact
HEAD. Do not use the retained-artifact approval override unless every
distribution obligation selected by that strategy has inspectable evidence.
