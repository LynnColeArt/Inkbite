#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-${VERSION:-dev}}"
BINARY="${2:-${BINARY:-inkbite}}"
DIST_DIR="${DIST_DIR:-dist}"
REPO_ROOT="$(pwd)"
LDFLAGS="-X main.version=${VERSION}"
TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

write_checksums() {
  local output_path="$1"
  shift

  if command -v sha256sum >/dev/null 2>&1; then
    (
      cd "$DIST_DIR"
      sha256sum "$@" > "$output_path"
    )
    return
  fi

  if command -v shasum >/dev/null 2>&1; then
    (
      cd "$DIST_DIR"
      shasum -a 256 "$@" > "$output_path"
    )
    return
  fi

  echo "missing checksum tool: expected sha256sum or shasum" >&2
  exit 1
}

require_tool go
require_tool tar
require_tool zip

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

stage_dir="$(mktemp -d)"
trap 'rm -rf "$stage_dir"' EXIT

artifacts=()
for target in "${TARGETS[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  binary_name="$BINARY"
  archive_name="${BINARY}_${VERSION}_${goos}_${goarch}"
  package_dir="${stage_dir}/${archive_name}"

  if [[ "$goos" == "windows" ]]; then
    binary_name="${binary_name}.exe"
  fi

  mkdir -p "$package_dir"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" -o "${package_dir}/${binary_name}" ./cmd/inkbite
  cp README.md CHANGELOG.md "$package_dir/"

  if [[ "$goos" == "windows" ]]; then
    (
      cd "$stage_dir"
      zip -q -r "${REPO_ROOT}/${DIST_DIR}/${archive_name}.zip" "$archive_name"
    )
    artifacts+=("${archive_name}.zip")
    continue
  fi

  tar -C "$stage_dir" -czf "${DIST_DIR}/${archive_name}.tar.gz" "$archive_name"
  artifacts+=("${archive_name}.tar.gz")
done

write_checksums "checksums.txt" "${artifacts[@]}"

printf 'wrote %s\n' "${DIST_DIR}/checksums.txt"
