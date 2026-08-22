# SK-RETRO-SUMMARY-001: Canonical retrospective is not discovered

## Summary

Spec Kitty 3.2.6 writes the canonical mission retrospective to
`kitty-specs/<mission-slug>/retrospective.yaml`, but `spec-kitty retrospect
summary` does not discover it in this repository. The summary instead scans
eight directories under the local mission-template installation and reports
each one as a missing retrospective.

This is a Spec Kitty reporting/discovery defect. It does not affect Inkbite
product behavior, the accepted mission state, or the canonical retrospective
bytes.

## Reproduction

From `/home/lynn/projects/inkbite` after mission merge:

```bash
test -f kitty-specs/inkbite-ingestion-contract-01M0M3HW/retrospective.yaml
spec-kitty retrospect summary --json
```

Observed on 2026-08-22:

- `mission_count: 8`
- `completed_count: 0`
- `terminus_no_retro_count: 8`
- mission IDs are `__pycache__`, `built_in_step_contracts`, `documentation`,
  `mission-steps`, `mission_types`, `plan`, `research`, and `software-dev`
- `inkbite-ingestion-contract-01M0M3HW` is absent

The dry-run synthesis command completes but produces no planned applications:

```bash
spec-kitty agent retrospect synthesize \
  --mission inkbite-ingestion-contract-01M0M3HW
```

## Expected behavior

The summary should enumerate canonical project mission directories under
`kitty-specs/`, parse their `retrospective.yaml` and status event streams, and
include `inkbite-ingestion-contract-01M0M3HW` with
`findings_status: has_findings`. It should not treat installed mission-template
directories as project mission instances.

## Impact

- Cross-mission learning dashboards falsely report zero completed missions.
- Valid retrospective findings and proposal statistics disappear from the
  summary surface.
- Operators may regenerate or duplicate a retrospective that already exists.

The direct artifact remains the closeout authority until discovery is fixed.

## Proposed regression

Create a temporary project with both `.kittify/missions/` templates and one
canonical `kitty-specs/<slug>/retrospective.yaml`. Assert that summary returns
only the project mission, preserves its `findings_status`, and ignores template,
cache, documentation, and built-in step-contract directories. Run the same
fixture with an explicit `--project` path and from the project working
directory.
