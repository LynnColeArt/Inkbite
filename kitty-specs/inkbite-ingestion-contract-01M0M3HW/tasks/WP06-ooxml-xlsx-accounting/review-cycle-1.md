---
affected_files: []
cycle_number: 1
mission_slug: inkbite-ingestion-contract-01M0M3HW
reproduction_command:
reviewed_at: '2026-08-22T13:22:39Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP06
---

# WP06 Review — Cycle 1

## Canonical finding

### 1. Detailed PPTX ingestion silently omits a failed optional extraction (blocking)

`converters/pptx/pptx.go:262-280` treats a referenced notes part as optional, but an XML parse failure returns `""` and is indistinguishable from an absent or intentionally empty note. `renderSlide` then succeeds without the notes, while `ConvertDetailed` at lines 71-80 always returns only `DetailedConversion{Result: result}` and therefore cannot report the degradation. A structurally valid PPTX whose referenced notes member exists but contains malformed XML consequently produces an authoritative partial detailed envelope with neither a stable warning nor a typed failure.

This violates FR-016 and WP06's explicit review gate that detailed degradation be visible and ordered. It also fails the review anti-pattern checks for a silent empty return and FR coverage: the WP has no production-path assertion for an allowed optional-extraction failure reaching `Engine.Ingest` as an ordered warning (or as a terminal typed failure).

Remediation: preserve the legacy `Convert` projection and its locked Markdown hash, but make the detailed path retain the notes outcome. Return a stable, safe-location warning in deterministic slide/source order when policy permits omission, or return a typed terminal failure. Add a deterministic public ingestion regression using a valid package with a referenced malformed notes part; assert no silent success and, if warning is selected, assert its category/location/order. The regression must call the real PPTX converter through `Engine.Ingest`, not construct a literal `DetailedConversion`.

## Verified implementation evidence

- The shared `RequestBudget` is the single actual-byte container authority across DOCX, PPTX, and XLSX. Member admission happens once after bounded EOF observation and declared/actual comparison; count, entry bytes, aggregate bytes, depth, ratio, checksum, cancellation, path/type, duplicate/collision, and relationship-target controls are exercised through production paths.
- XLSX calls `ooxml.Open` before `excelize.OpenReader` on every converter path and passes the same bounded `data.Bytes` to excelize. Production search found no raw XLSX ZIP bypass and no archive-member `io.ReadAll`; raw ZIP helpers are confined to tests and `internal/ooxml` itself is the authority.
- The WP product diff is exactly the six owned converter/security files plus `internal/ooxml/package.go`, `internal/ooxml/package_test.go`, and the explicitly coordinated shared seams `ingestion.go` and `internal/ingestion/context.go` (`6a5ced6`, `dfad7d3`). Final product tip `8049aff` is followed only by coordination/status commits.
- Legacy DOCX/PPTX/XLSX output hashes remain locked by production-path tests. No frozen public model, contract, module, dependency, license, or CLI surface changed.

## Gate record (Go 1.26.6)

- PASS: focused packages `-count=20`.
- PASS: focused packages under race `-count=5`.
- PASS: uncached `go test ./... -count=1` and `go test -race ./... -count=1`.
- PASS: `go vet ./...`, `go build ./...`, `go mod verify`, gofmt check, and `git diff --check`.
- PASS: `govulncheck ./...` reports zero reachable vulnerabilities.
- INFO: full `staticcheck ./...` reports only inherited `converters/pdf/pdf_test.go:181:6 makeSimplePDF is unused (U1000)`, outside WP06 scope.
- Coverage evidence is not the blocker: focused package coverage is 74.8% aggregate, while the implementation handoff records 81.8% changed-production coverage. Security deletion evidence for mandatory XLSX preflight is causal; removal allows traversal to reach a nil-error result.

## WP-level anti-pattern checklist

1. Dead code — PASS. New helpers and detailed converter methods have production callers.
2. Synthetic-fixture test — PASS. Security and fidelity cases invoke `ooxml.Open` and the real converter paths.
3. Silent empty return — FAIL. `renderNotes` swallows the optional notes parse failure.
4. FR coverage — FAIL. FR-016 lacks a production-path degradation assertion; FR-008, FR-011, and inherited FR-015 sealing behavior are exercised.
5. Frozen surface — PASS. No frozen contract/public model file changed.
6. Locked decision — PASS. No private budget, raw XLSX preflight bypass, network fetch, or unbounded production member read was introduced.
7. Shared-file ownership — PASS. The two shared ingestion seams and their exact commits are explicitly coordinated in the handoff.
8. Production fragility — N/A. No exception-style bare raise was added; security/integrity failures fail loud through typed Go errors.

## Re-review acceptance

The malformed-referenced-notes case must fail deterministically before the fix, then pass through the public detailed-ingestion path with either an ordered stable warning or a typed terminal error. All current accounting, fidelity, race, and full gates must remain green.
