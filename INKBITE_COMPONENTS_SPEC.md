# Inkbite Managed Components Specification

## Shipped boundary

This document records component-management behavior present in the repository.
It is not a roadmap for OCR conversion, GPU backends, inference providers, or
model orchestration.

The CLI can install and inspect an OCR helper foundation. The document
conversion pipeline does not use that helper: normal conversion never installs or invokes OCR,
even when component config exists. `ConvertOptions has no OCR field`, and the
conversion CLI has no `--ocr` flag.

## Commands

The shipped management commands are:

```text
inkbite components list
inkbite install ocr [--provider builtin|paddleocr] [--backend auto|cpu|cuda|rocm|metal] [--dir path]
inkbite doctor
inkbite config show
```

These commands are separate from `inkbite convert` and the default conversion
path. There is no shipped `uninstall` command.

`auto` and `cpu` both resolve to the CPU backend. `cuda`, `rocm`, and `metal`
are rejected as unavailable. Provider names in this document describe the
explicit installer only; they do not describe conversion-time OCR capability.

## Explicit installation effects

### `builtin`

The default `inkbite install ocr` command:

1. copies the current Inkbite executable into a versioned component directory;
2. records its SHA-256 in `manifest.json`;
3. runs the helper's CPU self-test; and
4. records the installed selection in `config/config.json`.

The builtin installation path does not fetch a model and does not add OCR
extraction to document conversion.

### `paddleocr`

`inkbite install ocr --provider paddleocr` is an explicit network- and
subprocess-authorizing administration command. On supported non-Windows hosts
it finds `python3` or `python`, creates a virtual environment, upgrades `pip`,
and installs these pinned packages:

| Package/source | Pin |
| --- | --- |
| PaddlePaddle CPU wheel index | `https://www.paddlepaddle.org.cn/packages/stable/cpu/` |
| `paddlepaddle` | `3.2.0` |
| `paddleocr` | `3.4.0` |
| `chardet` | `5.2.0` |

It then writes a local wrapper and helper script, records their SHA-256 values,
runs a self-test, and writes config. Windows rejects this provider. The command
may cause package or model downloads under the explicitly created component
environment. None of these effects occurs during ordinary conversion.

Installing or self-testing a helper does not prove OCR quality and does not
make OCR output part of `Result` or `IngestionEnvelope`.

## Storage locations

An explicit `--dir` wins. Otherwise `INKBITE_HOME` wins. Without either, the
base directory is:

| Platform | Base directory |
| --- | --- |
| Linux and other Unix | `$XDG_DATA_HOME/inkbite`, or `$HOME/.local/share/inkbite` |
| macOS | `$HOME/Library/Application Support/inkbite` |
| Windows | `%LocalAppData%\inkbite`, or `%USERPROFILE%\AppData\Local\inkbite` |

The managed layout is:

```text
<base>/
  config/config.json
  components/ocr/versions/<version>/
    manifest.json
    bin/inkbite-ocr-helper[.exe]
    models/
```

The PaddleOCR installer additionally creates `venv/`, `libexec/`, and a
component-local home/cache directory.

## Config and manifest

Missing config is a valid empty state. `config show` prints the JSON config;
`components list` reports the enabled configured component. The OCR config
fields are:

- `enabled`
- `provider`
- `backend`
- `component`
- `version`
- `install_dir`
- optional `last_doctor`

The bundle manifest records component, provider, bundle ID, Inkbite version,
OS, architecture, backend, minimum VRAM metadata, and relative file paths with
SHA-256 values.

`doctor` checks that the configured manifest and helper exist, runs the helper
self-test when those files are present, reports issues, and updates
`last_doctor` after a healthy result. It does not run document OCR.

## Conversion authority

The detailed-ingestion `IngestionPolicy.Component` field is an explicit
selection label verified against converter output. It is not connected to the
managed OCR config in this repository. Merely installing a component grants no
conversion, network, subprocess, inference, or persistence authority.

Remote source authority is separately controlled by
`IngestionPolicy.Remote.Enabled` or legacy `ConvertOptions.EnableHTTP` plus a
caller-supplied HTTP transport. Component configuration cannot enable remote
source acquisition.

## Tests and non-claims

Shipped installer, listing, config, doctor, and self-test behavior is covered by
[`cmd/inkbite/main_test.go`](cmd/inkbite/main_test.go) and the tests under
`internal/components` and `internal/ocr`.

This specification makes no claim that:

- OCR is invoked by conversion;
- installed providers produce document text;
- GPU backends are available;
- package installation is offline or hermetic;
- managed files are durable host records; or
- future OCR, inference, or provider behavior is committed.

The public conversion and retention contract is documented in
[INKBITE_SPEC.md](INKBITE_SPEC.md).
