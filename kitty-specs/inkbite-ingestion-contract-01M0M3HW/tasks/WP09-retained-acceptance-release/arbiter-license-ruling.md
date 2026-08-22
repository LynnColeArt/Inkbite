---
affected_files:
  - kitty-specs/inkbite-ingestion-contract-01M0M3HW/spec.md
  - kitty-specs/inkbite-ingestion-contract-01M0M3HW/tasks/WP09-retained-acceptance-release.md
mission_slug: inkbite-ingestion-contract-01M0M3HW
review_kind: arbiter_release_license_ruling
reviewed_at: '2026-08-22T15:48:00Z'
verdict: source_only_correction_required
wp_id: WP09
---

# WP09 Release-License Arbitration

## Ruling

The cycle-2 rejection is valid. The live default executable graph includes `github.com/shakinm/xlsReader` (`GPL-3.0-only`), while the current release archives convey that linked executable without a completed GPL/transitive-license closure. A documentation-only warning cannot make those binary archives qualified.

The smallest honest correction is to make every repository-owned publication path source-only. This preserves runtime behavior, XLS compatibility, and repository-authored MIT licensing while declining to distribute the unqualified combined binary. A binary-release strategy is deferred to a separate specification and independent license review.

## Binding correction

- Canonical packaging emits deterministic `*_source.tar.gz` and `*_source.zip` archives plus `checksums.txt`.
- Archive inspection proves the exact allowed tracked-source manifest and required project/license/adoption/module files.
- Archives contain no executable, object, vendored dependency tree, module-cache material, or third-party dependency source.
- `scripts/dist.sh` delegates to the canonical packaging authority.
- CI and tag-release workflows upload only qualified source-archive patterns and the checksum manifest; local binary builds remain verification-only.
- Public documentation states that default binaries link GPL-3.0-only `xlsReader`, are not MIT-only, and are not qualified for redistribution by this workflow.

## Required red-first evidence

1. The current canonical packager fails because it emits five platform-specific linked-binary archives.
2. The legacy distribution script fails because it independently builds binary archives.
3. CI/tag publication fails because broad artifact globs admit linked binaries.
4. Mutations reintroducing an executable, `vendor/`, missing required files, extra entries, nondeterministic metadata, legacy divergence, broad upload globs, or removal of the source-only/GPL warning must fail.

After the focused release matrix is green, freeze one committed tree and run WP09's nine mandatory commands exactly once in order. Any failure remains failed evidence.

## Frozen surfaces

No correction may change runtime conversion behavior, the XLS converter, `go.mod`, `go.sum`, `Makefile`, the CLI, built-in registration, or other work-package product files.
