# Adopted Components and Attribution Record

Audit date: 2026-08-22

This engineering record distinguishes ideas, copied source, and linked Go
dependencies. It is not legal advice. Each record points to an exact upstream
revision and its license; release owners remain responsible for satisfying the
complete license texts and the licenses of transitive dependencies.

Inkbite's repository-authored code is licensed under [MIT](LICENSE). That root
license does not replace third-party licenses.

Default Inkbite binaries link GPL-3.0-only xlsReader, are not MIT-only, and are not qualified for redistribution by this workflow.

The repository's official release workflow therefore publishes only the exact
committed tracked-source manifest. It excludes linked executables, object
files, vendored modules, module-cache material, and third-party dependency
source. Local and CI binary builds are verification-only.

## Classification rules

### Classification: inspiration

An upstream project influenced product vocabulary, workflow, or architecture,
but no upstream source file or fixture is identified as copied. Inspiration is
not a build dependency. Record the exact reference revision used for the audit
and avoid claiming code adoption.

### Classification: copied code

Source, documentation, fixtures, or generated material is copied or adapted
into this repository. A record must identify every local and upstream file,
the exact upstream revision, license and notice, local modifications, adoption
date, and redistribution obligations.

No copied upstream files or fixtures were identified by this audit. The initial
spec suggested that upstream fixtures could be reused, but git history does not
show such a reuse. This statement must be replaced by concrete records if
copied material is later introduced.

### Classification: dependency

The repository imports an external module through `go.mod`; source is not
copied into this repository. Record the selected module version and resolved
tag commit, imported purpose, license, modification status, and binary/source
distribution obligations. Transitive modules remain part of the release bill
of materials even when this file records only directly imported modules.

## Record template

- Classification: `inspiration`, `copied code`, or `dependency`
- Upstream project and URL:
- Exact revision: immutable commit plus human-readable tag when available
- Adopted files/design: exact upstream and local paths, packages, or idea
- SPDX license expression:
- Notice location: upstream license and local distribution notice location
- Local modifications:
- Adoption date:
- Distribution obligations:
- Evidence/rationale:

Do not merge a record with an unknown classification, mutable branch name, or
missing license review.

## Inspiration records

### Microsoft MarkItDown

- Classification: inspiration
- Upstream project and URL: [microsoft/markitdown](https://github.com/microsoft/markitdown)
- Exact revision: `v0.1.5`, commit `4a5340f93b2bf1dc11641f921fbfd6d5f016924b`
- Adopted files/design: design-level ideas only: a stream-oriented dispatcher,
  priority-ordered converters, source hints, Markdown output, and optional data
  URI retention. No upstream file or fixture is identified as copied.
- SPDX license expression: `MIT`
- Notice location: exact-revision upstream
  [`LICENSE`](https://github.com/microsoft/markitdown/blob/4a5340f93b2bf1dc11641f921fbfd6d5f016924b/LICENSE);
  this record documents the inspiration. No MarkItDown notice is required for uncopied ideas, but any
  later copied material requires a copied-code record and retained notice.
- Local modifications: independent Go implementation with a different public
  API, security policy, format scope, and detailed-ingestion envelope.
- Adoption date: 2026-03-21 (initial Inkbite specification/scaffold)
- Distribution obligations: none from inspiration alone; do not use Microsoft
  names to imply endorsement.
- Evidence/rationale: the initial `INKBITE_SPEC.md` explicitly described
  MarkItDown as upstream inspiration and rejected parity. The release tag above
  is the exact audit reference available before the initial scaffold; repository
  history does not prove that any MarkItDown source file was copied.

## Copied-code records

### Current inventory

- Classification: copied code
- Exact revision: not applicable because the audit found no copied upstream
  material.
- Adopted files/design: none identified.
- SPDX license expression: not applicable.
- Notice location: not applicable.
- Local modifications: not applicable.
- Adoption date: not applicable.
- Distribution obligations: add a concrete record before copied material is
  distributed.

## Direct dependency records

### github.com/JohannesKaufmann/html-to-markdown/v2

- Classification: dependency
- Upstream project and URL: [JohannesKaufmann/html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown)
- Exact revision: module `v2.5.0`, tag commit
  `3006818b20a61b0a36eb86321aef57d3d017c27e`
- Adopted files/design: imported package in `converters/html/html.go` for HTML
  parsing/conversion support; no vendored files.
- SPDX license expression: `MIT`
- Notice location: exact-revision upstream
  [`LICENSE`](https://github.com/JohannesKaufmann/html-to-markdown/blob/3006818b20a61b0a36eb86321aef57d3d017c27e/LICENSE);
  dependency and obligation recorded here. Full text is not vendored in this repository.
- Local modifications: none to the dependency; Inkbite wraps its public API.
- Adoption date: 2026-03-21
- Distribution obligations: retain the upstream copyright and MIT permission
  notice in distributions containing substantial portions.

### dslipak/pdf

- Classification: dependency
- Upstream project and URL: [dslipak/pdf](https://github.com/dslipak/pdf)
- Exact revision: module `v0.0.2`, tag commit
  `636e0c026eb4fc360db4e964ac51005acd6286e3`
- Adopted files/design: imported in `converters/pdf/pdf.go` for pure-Go PDF text
  extraction; no vendored files.
- SPDX license expression: `BSD-3-Clause`
- Notice location: exact-revision upstream
  [`LICENSE`](https://github.com/dslipak/pdf/blob/636e0c026eb4fc360db4e964ac51005acd6286e3/LICENSE);
  dependency and obligation recorded here. Full text is not vendored in this repository.
- Local modifications: none to the dependency.
- Adoption date: 2026-03-22
- Distribution obligations: source distributions retain the copyright,
  conditions, and disclaimer; binary distributions reproduce them in supplied
  documentation/materials; names may not imply endorsement.

### pdfcpu/pdfcpu

- Classification: dependency
- Upstream project and URL: [pdfcpu/pdfcpu](https://github.com/pdfcpu/pdfcpu)
- Exact revision: module `v0.12.1`, tag commit
  `148d18d48afbe63e1c55741280adba696306e5c2`
- Adopted files/design: imported in `converters/pdf/pdf.go` for PDF image-object
  extraction; no vendored files.
- SPDX license expression: `Apache-2.0`
- Notice location: exact-revision upstream
  [`LICENSE.txt`](https://github.com/pdfcpu/pdfcpu/blob/148d18d48afbe63e1c55741280adba696306e5c2/LICENSE.txt);
  dependency and obligation recorded here. Full text is not vendored in this repository.
- Local modifications: none to the dependency.
- Adoption date: 2026-05-24
- Distribution obligations: provide Apache-2.0 license text; preserve required
  notices and modification notices; provide any upstream `NOTICE` content when
  present; respect the patent and trademark clauses.

### shakinm/xlsReader

- Classification: dependency
- Upstream project and URL: [shakinm/xlsReader](https://github.com/shakinm/xlsReader)
- Exact revision: module `v0.9.12`, tag commit
  `cb2bf4031fc7b9d539e3d07ab15219ff240630d7`
- Adopted files/design: imported in `converters/xls/xls.go` for legacy XLS
  workbook parsing; no vendored files.
- SPDX license expression: `GPL-3.0-only`
- Notice location: exact-revision upstream
  [`LICENSE`](https://github.com/shakinm/xlsReader/blob/cb2bf4031fc7b9d539e3d07ab15219ff240630d7/LICENSE);
  dependency and obligation recorded here. Full text and corresponding-source offer are not
  currently packaged by this repository.
- Local modifications: none to the dependency.
- Adoption date: 2026-03-22
- Distribution obligations: the bundled license requires GPLv3 compliance for
  conveying a covered combined work, including the GPL license, source and
  binary notices, complete corresponding source, installation information when
  applicable, and licensing the covered work under GPLv3. This direct
  dependency means a compiled Inkbite binary must not be described as
  MIT-only. Release packaging requires legal/license review before distribution.

### github.com/xuri/excelize/v2

- Classification: dependency
- Upstream project and URL: [qax-os/excelize](https://github.com/qax-os/excelize)
- Exact revision: module `v2.10.1`, tag commit
  `5ad5ab3af0054c55bdce09f1530085600e9f2e45`
- Adopted files/design: imported in `converters/xlsx/xlsx.go` for workbook
  parsing after Inkbite's OOXML preflight; no vendored files.
- SPDX license expression: `BSD-3-Clause`
- Notice location: exact-revision upstream
  [`LICENSE`](https://github.com/qax-os/excelize/blob/5ad5ab3af0054c55bdce09f1530085600e9f2e45/LICENSE);
  dependency and obligation recorded here. Full text is not vendored in this repository.
- Local modifications: none to the dependency.
- Adoption date: 2026-03-21
- Distribution obligations: source distributions retain the copyright,
  conditions, and disclaimer; binary distributions reproduce them in supplied
  documentation/materials; names may not imply endorsement.

### golang.org/x/net

- Classification: dependency
- Upstream project and URL: [golang/net](https://go.googlesource.com/net)
- Exact revision: module `v0.55.0`, tag commit
  `7770ec48d03fec35e378665337b4faca93c38423`
- Adopted files/design: imported by `textdecode.go` and the HTML converter for
  character-set and HTML support; no vendored files.
- SPDX license expression: `BSD-3-Clause`
- Notice location: exact-revision upstream
  [`LICENSE`](https://go.googlesource.com/net/+/7770ec48d03fec35e378665337b4faca93c38423/LICENSE);
  dependency and obligation recorded here. Full text is not vendored in this repository.
- Local modifications: none to the dependency.
- Adoption date: 2026-03-21; updated to the recorded revision on 2026-08-22.
- Distribution obligations: source distributions retain the copyright,
  conditions, and disclaimer; binary distributions reproduce them in supplied
  documentation/materials; names may not imply endorsement.

## Release checklist

Before publishing source or binaries:

1. compare this record with `go.mod`, `go.sum`, and `go list -m all`;
2. inventory transitive dependency licenses at the selected revisions;
3. reproduce required third-party license and notice texts in release
   materials;
4. resolve the `GPL-3.0-only` distribution implications of `xlsReader`;
5. add copied-code records for any vendored/adapted source or fixtures; and
6. record any local dependency patch with exact files and rationale.

The absence of vendored third-party license texts is recorded evidence, not a
claim that attribution obligations have already been satisfied. This mission's
source-only workflow does not qualify or publish a linked binary; any future
binary release needs a separate specification and independent license review.
