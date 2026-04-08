# Changelog

All notable changes to this project will be documented in this file.

The format is intentionally lightweight at the current stage of the project.

## Unreleased

### Added

- managed optional-component foundation with `components list`, `doctor`,
  `config show`, and `install ocr`
- experimental `install ocr --provider paddleocr` path for managed CPU OCR
  runtime setup
- paddleocr installer polish with streamed progress, pinned `chardet`
  compatibility, and a faster quieter self-test path
- distributable Codex skill for guiding Inkbite CLI and library usage
- basic legacy XLS extraction with formatted numeric and date rendering
- reduced-scope PPTX extraction with support for slide order, slide titles,
  body text, notes, simple tables, and hyperlinks
- fixture-backed regression coverage for PDF, DOCX, EPUB, PPTX, and ZIP flows
- malformed-input regression tests for PDF, DOCX, EPUB, and PPTX
- ZIP archive guardrails for entry count, entry size, total uncompressed size,
  and recursion depth
- build automation through `Makefile`
- continuous integration workflow for test, vet, and CLI build verification
- release workflow for tagged builds and generated release notes
- cross-platform CI coverage for Linux, macOS, and Windows with race detection
  and automated `govulncheck` scanning
- packaged release archives for Linux, macOS, and Windows with generated
  checksum manifests

### Changed

- PDF extraction is fully self-contained and no longer depends on external
  executables
- legacy XLS extraction now uses a self-contained reader path with improved
  formatted output for common date and numeric cells
- README now documents the project in a formal, research-oriented tone
- remote HTTP fetches now enforce a bounded response size limit by default
- the module now targets Go `1.25.9` to pick up current patched standard
  library fixes used by CI and release builds
