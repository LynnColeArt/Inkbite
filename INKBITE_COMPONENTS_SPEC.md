# Inkbite Optional Components and OCR Spec

## Purpose

Define the post-MVP architecture for optional installed components, beginning
with OCR. The goal is to preserve the current pure-Go core while making heavier
capabilities available through an explicit install and validation workflow.

This document extends [INKBITE_SPEC.md](/home/lynn/projects/inkbite/INKBITE_SPEC.md).
It does not replace the MVP goals of deterministic conversion and simple core
deployment.

## Goals

1. Keep the default `inkbite` experience usable without OCR or external model
   assets.
2. Add an explicit installation path for optional components such as OCR.
3. Support CPU-first OCR, with GPU acceleration treated as an optional backend.
4. Avoid surprise downloads during normal conversion.
5. Validate installed components through a built-in `doctor` flow.
6. Allow future optional components to reuse the same install, manifest, and
   config machinery.

## Non-Goals

- baking heavyweight OCR runtimes into the default core binary
- automatic installation of kernel drivers or vendor GPU toolchains
- silently invoking OCR on every conversion
- promising cross-platform GPU parity in the first release
- treating OCR as a required part of supported PDF, DOCX, or PPTX extraction

## Product Shape

### Core vs Optional Components

The product should be split into:

- `inkbite` core: pure-Go conversion engine, CLI, and built-in converters
- optional components: versioned installed artifacts managed by `inkbite`

The first optional component is OCR. Future examples could include:

- alternate OCR runtimes
- image captioning or vision helpers
- format-specific helper runtimes that are too heavy for the core binary

### Installation Principle

Normal conversion must never auto-download models or helper runtimes.

If OCR is requested and the required component is missing, `inkbite` should
return a clear actionable error that points to `inkbite install ocr`.

## Command Surface

Replace any future `--install` flag idea with explicit subcommands.

### Planned Commands

```text
inkbite convert [flags] [source]
inkbite components list
inkbite install ocr [flags]
inkbite install all [flags]
inkbite uninstall ocr
inkbite doctor
inkbite config show
```

### OCR-Related Flags

The convert path should accept:

```text
--ocr off|auto|images|force
```

Semantics:

- `off`: disable OCR entirely
- `auto`: run normal extraction first, then invoke OCR only on near-empty or
  likely image-only content
- `images`: preserve normal extraction and OCR embedded images where supported
- `force`: invoke OCR even if ordinary extraction produced text

Installer flags should include:

```text
--backend auto|cpu|cuda|rocm|metal
--dir <path>
--yes
--force
```

## User Experience

### Happy Path

```bash
inkbite install ocr
inkbite doctor
inkbite ./scan.pdf --ocr auto
```

### Missing Component Path

If the user runs:

```bash
inkbite ./scan.pdf --ocr auto
```

without OCR installed, return a message like:

```text
ocr component is not installed
run: inkbite install ocr
```

### GPU Recommendation Path

`inkbite install ocr` should:

1. detect the current OS and architecture
2. probe CPU and available GPU backends
3. recommend a backend
4. allow user override
5. install the chosen bundle
6. run a self-test

Example output:

```text
Detected backend candidates:
- cpu: available
- cuda: available (RTX 3060, 12288 MB)

Recommended backend: cuda
Installing OCR component: ocr-paddle-cuda-linux-amd64
Running self-test: ok
```

## Storage Layout

Install managed components under a per-user data directory.

### Default Base Directories

- Linux: `${XDG_DATA_HOME:-$HOME/.local/share}/inkbite`
- macOS: `~/Library/Application Support/inkbite`
- Windows: `%LocalAppData%\\inkbite`

### Directory Layout

```text
components/
  ocr/
    current/
    versions/
      0.1.0/
        manifest.json
        bin/
        models/
        lib/
config/
  config.json
cache/
logs/
```

`current/` may be a symlink or a small pointer file, depending on platform
support and implementation simplicity.

## Config Model

The core CLI should persist a lightweight config file that records the selected
backend and installed component metadata.

### Example Config

```json
{
  "ocr": {
    "enabled": true,
    "backend": "auto",
    "component": "ocr-paddle-cpu-linux-amd64",
    "version": "0.1.0",
    "install_dir": "/home/user/.local/share/inkbite/components/ocr/current",
    "last_doctor": "2026-03-22T12:00:00Z"
  }
}
```

### Config Rules

- config should be human-readable
- missing config should be treated as a valid default state
- explicit CLI flags should override config
- config should never imply background downloads

## ConvertOptions Extension

The Go API should grow carefully to support OCR without breaking the core
contract.

### Proposed Additions

```go
type OCRMode string

const (
    OCRModeOff    OCRMode = "off"
    OCRModeAuto   OCRMode = "auto"
    OCRModeImages OCRMode = "images"
    OCRModeForce  OCRMode = "force"
)

type ConvertOptions struct {
    KeepDataURIs bool
    EnableHTTP   bool
    PDFBackend   string
    OCRMode      OCRMode
}
```

The library should not expose installation concerns through `ConvertOptions`.
Install and component management should remain a CLI and internal package
concern.

## Component Registry

Optional components should be described by manifests rather than hardcoded
download logic.

### Bundle Identity

Bundle IDs should encode component, backend, OS, and architecture.

Examples:

- `ocr-paddle-cpu-linux-amd64`
- `ocr-paddle-cuda-linux-amd64`
- `ocr-paddle-metal-darwin-arm64`

### Manifest Shape

```json
{
  "component": "ocr",
  "bundle_id": "ocr-paddle-cpu-linux-amd64",
  "version": "0.1.0",
  "os": "linux",
  "arch": "amd64",
  "backend": "cpu",
  "requirements": {
    "min_vram_mb": 0
  },
  "files": [
    {
      "path": "bin/inkbite-ocr-helper",
      "sha256": "..."
    },
    {
      "path": "models/det.onnx",
      "sha256": "..."
    },
    {
      "path": "models/rec.onnx",
      "sha256": "..."
    }
  ]
}
```

### Registry Rules

- every downloaded file must be checksum-verified
- manifests must be versioned
- installs should be atomic from the user's perspective
- a failed install must not corrupt an existing working version

## OCR Runtime Boundary

Keep OCR outside the core library binary through a helper runtime boundary.

### Recommended Shape

- core `inkbite` binary remains pure Go
- OCR lives in an installed helper such as `inkbite-ocr-helper`
- the helper is launched only when OCR is requested

This allows the OCR stack to carry native libraries, ONNX runtimes, or model
files without forcing them into the default build.

## Backend Detection

Backend detection should be advisory and validated, not guessed from hardware
names alone.

### Detection Stages

1. detect OS and architecture
2. detect candidate runtime families:
   - CPU
   - CUDA
   - ROCm
   - Metal
3. collect device metadata where available
4. compare against bundle requirements
5. run a lightweight self-test for the selected backend

### Rules

- CPU is always the fallback recommendation
- GPU availability without a working runtime must not be treated as usable
- successful self-test matters more than nominal hardware capability

## Doctor Command

`inkbite doctor` should validate both the core environment and installed
optional components.

### Doctor Output Should Cover

- core version
- config path
- installed optional components
- configured OCR backend
- detected backends
- helper binary presence
- model file presence
- self-test status
- recommended action if anything is missing or broken

## OCR Scope By Format

OCR should be introduced incrementally.

### Phase 1

- standalone image OCR
- embedded-image OCR for DOCX
- embedded-image OCR for PPTX

These are the best starting targets because images can be extracted directly
from the input container.

### Phase 2

- scanned PDF OCR

This phase requires page rasterization in addition to OCR. It should not be
attempted until the component and helper model are already working for simpler
image inputs.

### Output Strategy

OCR text should be appended as labeled supplemental sections rather than mixed
silently into ordinary extraction.

Example:

```md
## OCR Supplement

### Embedded Image 1
Recognized text here
```

## Package Layout

Add internal packages for component management.

### Proposed Packages

```text
cmd/inkbite/
    main.go

internal/components/
    bundle.go
    config.go
    doctor.go
    install.go
    manifest.go
    paths.go
    probe.go

internal/ocr/
    mode.go
    client.go
    merge.go
```

The CLI should remain the orchestration layer. Converters should not download or
install anything on their own.

## Milestones

### Milestone 1: Component Foundation

- add config path and data directory helpers
- add component manifest types
- add install state tracking
- add `components list`, `config show`, and `doctor`

### Milestone 2: CPU OCR Installation

- add `install ocr`
- support CPU bundle download and verification
- support helper self-test
- persist selected backend and installed version

### Milestone 3: OCR Conversion Surface

- add `OCRMode` to `ConvertOptions`
- add `--ocr` CLI flag
- wire OCR invocation behind installed helper checks

### Milestone 4: Embedded Image OCR

- add OCR for standalone images
- add embedded-image OCR for DOCX and PPTX
- append OCR results as labeled supplements

### Milestone 5: Scanned PDF OCR

- choose a page rasterization strategy
- add OCR fallback for scanned PDFs
- gate it behind `auto` or `force`

### Milestone 6: GPU Backends

- add backend-specific OCR bundles
- add runtime probing and bundle recommendation
- validate GPU self-test paths

## Acceptance Criteria

This extension is successful when:

1. `inkbite install ocr` can install a CPU OCR bundle end to end.
2. `inkbite doctor` can report installed state and detect broken installs.
3. `inkbite --ocr auto` fails clearly when OCR is missing.
4. OCR-enabled conversion does not change default non-OCR behavior.
5. OCR can be added to supported formats without requiring auto-downloads or
   external shell scripts in the core conversion path.
