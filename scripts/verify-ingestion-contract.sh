#!/usr/bin/env bash
set -euo pipefail

readonly IMMUTABLE_BASE="ee5542edd1ac64b5f66dcb9d0056dd4815739342"
readonly PINNED_TOOLCHAIN="go1.26.6"
readonly ARCHIVE_TIMESTAMP="200001010000"
readonly REVIEW_LOCK_PATH=".spec-kitty/review-lock.json"

quality_baseline_status=""
quality_baseline_lock_sha256=""
quality_baseline_lock_present=false

require_tool() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required tool: $1" >&2
    exit 1
  }
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

capture_quality_cleanliness() {
  quality_baseline_status="$(git status --porcelain=v1 --untracked-files=all)"
  case "$quality_baseline_status" in
    ""|"?? $REVIEW_LOCK_PATH") ;;
    *)
      echo "quality gate requires a clean frozen worktree (except an unchanged $REVIEW_LOCK_PATH):" >&2
      git status --short --untracked-files=all >&2
      return 1
      ;;
  esac

  quality_baseline_lock_sha256=""
  quality_baseline_lock_present=false
  if [[ -e .spec-kitty || -L .spec-kitty ]]; then
    local spec_kitty_entries
    if [[ ! -d .spec-kitty || -L .spec-kitty ]]; then
      echo "quality gate found a non-directory .spec-kitty entry" >&2
      return 1
    fi
    spec_kitty_entries="$(find .spec-kitty -mindepth 1 -print | LC_ALL=C sort)"
    if [[ "$spec_kitty_entries" != "$REVIEW_LOCK_PATH" || ! -f "$REVIEW_LOCK_PATH" || -L "$REVIEW_LOCK_PATH" ]]; then
      echo "quality gate permits only the pre-existing regular file $REVIEW_LOCK_PATH under .spec-kitty" >&2
      return 1
    fi
    quality_baseline_lock_sha256="$(sha256_file "$REVIEW_LOCK_PATH")"
    quality_baseline_lock_present=true
  elif [[ "$quality_baseline_status" == "?? $REVIEW_LOCK_PATH" ]]; then
    echo "quality gate could not read the reported $REVIEW_LOCK_PATH" >&2
    return 1
  fi
}

verify_quality_cleanliness() {
  local final_status
  final_status="$(git status --porcelain=v1 --untracked-files=all)"
  if [[ "$final_status" != "$quality_baseline_status" ]]; then
    echo "quality gate changed the frozen worktree:" >&2
    git status --short --untracked-files=all >&2
    return 1
  fi

  if [[ "$quality_baseline_lock_present" == true ]]; then
    local spec_kitty_entries final_lock_sha256
    if [[ ! -d .spec-kitty || -L .spec-kitty ]]; then
      echo "quality gate deleted or replaced the pre-existing .spec-kitty directory" >&2
      return 1
    fi
    spec_kitty_entries="$(find .spec-kitty -mindepth 1 -print | LC_ALL=C sort)"
    if [[ "$spec_kitty_entries" != "$REVIEW_LOCK_PATH" || ! -f "$REVIEW_LOCK_PATH" || -L "$REVIEW_LOCK_PATH" ]]; then
      echo "quality gate changed the permitted $REVIEW_LOCK_PATH entry" >&2
      return 1
    fi
    final_lock_sha256="$(sha256_file "$REVIEW_LOCK_PATH")"
    if [[ "$final_lock_sha256" != "$quality_baseline_lock_sha256" ]]; then
      echo "quality gate modified the pre-existing $REVIEW_LOCK_PATH" >&2
      return 1
    fi
  elif [[ -e .spec-kitty || -L .spec-kitty ]]; then
    echo "quality gate created .spec-kitty state" >&2
    return 1
  fi
}

validate_release_component() {
  local kind="$1"
  local value="$2"
  if [[ ! "$value" =~ ^[A-Za-z0-9._-]+$ ]]; then
    echo "invalid $kind for source archive: $value" >&2
    return 1
  fi
}

source_archive_stem() {
  printf '%s_%s_source' "$1" "$2"
}

verify_source_release_warning() {
  local root="${1:-.}"
  local warning="Default Inkbite binaries link GPL-3.0-only xlsReader, are not MIT-only, and are not qualified for redistribution by this workflow."
  local document
  for document in README.md ADOPTED_COMPONENTS.md CHANGELOG.md; do
    grep -Fqx "$warning" "$root/$document" || {
      echo "missing source-only GPL-linked-binary warning in $document" >&2
      return 1
    }
  done
}

validate_source_manifest() {
  local required path mode
  for required in README.md CHANGELOG.md LICENSE ADOPTED_COMPONENTS.md go.mod go.sum; do
    git cat-file -e "HEAD:$required" || {
      echo "source release missing required tracked file: $required" >&2
      return 1
    }
  done
  while IFS= read -r path; do
    case "$path" in
      vendor/*|*/vendor/*|pkg/mod/*|*/pkg/mod/*|third_party/*|*/third_party/*|*.exe|*.dll|*.so|*.dylib|*.a|*.o|*.obj)
        echo "source release contains forbidden dependency or binary path: $path" >&2
        return 1
        ;;
    esac
  done < <(git ls-files)
  while read -r mode _ _ path; do
    case "$mode" in
      120000|160000)
        echo "source release cannot contain symlink or submodule entry: $path" >&2
        return 1
        ;;
    esac
  done < <(git ls-files -s)
}

write_source_checksums() {
  local output_dir="$1"
  shift
  (
    cd "$output_dir"
    : >checksums.txt
    local artifact
    for artifact in "$@"; do
      printf '%s  %s\n' "$(sha256_file "$artifact")" "$artifact" >>checksums.txt
    done
  )
}

build_source_packages_raw() (
  local output_dir="$1"
  local version="$2"
  local binary="$3"
  local stage_dir archive_name package_dir tar_name zip_name
  validate_release_component version "$version"
  validate_release_component binary "$binary"
  validate_source_manifest
  verify_source_release_warning .
  if [[ "$output_dir" != /* ]]; then
    output_dir="$(pwd)/$output_dir"
  fi
  archive_name="$(source_archive_stem "$binary" "$version")"
  tar_name="$archive_name.tar.gz"
  zip_name="$archive_name.zip"
  stage_dir="$(mktemp -d)"
  trap 'rm -rf "$stage_dir"' EXIT
  rm -rf "$output_dir"
  mkdir -p "$output_dir"
  package_dir="$stage_dir/$archive_name"
  mkdir -p "$package_dir"
  git archive --format=tar HEAD | tar -x -C "$package_dir"
  find "$package_dir" -type d -exec chmod 0755 {} +
  find "$package_dir" -type f -exec chmod 0644 {} +
  find "$package_dir" -exec touch -t "$ARCHIVE_TIMESTAMP" {} +

  GOTOOLCHAIN="$PINNED_TOOLCHAIN" go run ./scripts/package-source.go \
    "$package_dir" "$archive_name" "$output_dir/$tar_name" "$output_dir/$zip_name"
  write_source_checksums "$output_dir" "$tar_name" "$zip_name"
)

inspect_source_tree() {
  local tree="$1"
  local expected="$2"
  local relative header
  if [[ -n "$(find "$tree" -type l -print -quit)" ]]; then
    echo "source archive contains a symlink" >&2
    return 1
  fi
  while IFS= read -r relative; do
    if [[ -x "$relative" ]]; then
      echo "source archive contains an executable file mode" >&2
      return 1
    fi
  done < <(find "$tree" -type f -print)
  while IFS= read -r relative; do
    relative="${relative#./}"
    case "$relative" in
      vendor/*|*/vendor/*|pkg/mod/*|*/pkg/mod/*|third_party/*|*/third_party/*|*.exe|*.dll|*.so|*.dylib|*.a|*.o|*.obj)
        echo "source archive contains forbidden dependency or binary path: $relative" >&2
        return 1
        ;;
    esac
    header="$(od -An -tx1 -N8 "$tree/$relative" | tr -d ' \n')"
    case "$header" in
      7f454c46*|4d5a*|feedface*|cefaedfe*|feedfacf*|cffaedfe*|cafebabe*|213c617263683e0a*)
        echo "source archive contains executable or object bytes: $relative" >&2
        return 1
        ;;
    esac
  done < <(cd "$tree" && find . -type f -print | LC_ALL=C sort)
  diff -qr "$expected" "$tree"
  verify_source_release_warning "$tree"
}

inspect_source_archive() (
  local archive="$1"
  local format="$2"
  local archive_name="$3"
  local temp_dir expected extracted
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "$temp_dir"' EXIT
  expected="$temp_dir/expected"
  extracted="$temp_dir/extracted"
  mkdir -p "$expected" "$extracted"
  git archive --format=tar HEAD | tar -x -C "$expected"
  case "$format" in
    tar) tar -xzf "$archive" -C "$extracted" ;;
    zip) unzip -qq "$archive" -d "$extracted" ;;
  esac
  if [[ "$(cd "$extracted" && find . -mindepth 1 -maxdepth 1 -print | sed 's#^\./##')" != "$archive_name" || ! -d "$extracted/$archive_name" ]]; then
    echo "source archive has an invalid top-level manifest" >&2
    return 1
  fi
  inspect_source_tree "$extracted/$archive_name" "$expected"
)

verify_source_packages() (
  local output_dir="$1"
  local version="$2"
  local binary="$3"
  local archive_name tar_name zip_name actual expected_manifest reference
  archive_name="$(source_archive_stem "$binary" "$version")"
  tar_name="$archive_name.tar.gz"
  zip_name="$archive_name.zip"
  actual="$(cd "$output_dir" && find . -mindepth 1 -maxdepth 1 -print | sed 's#^\./##' | LC_ALL=C sort)"
  if [[ "$actual" != "checksums.txt
$tar_name
$zip_name" ]]; then
    echo "source release output contains missing or extra files:" >&2
    printf '%s\n' "$actual" >&2
    return 1
  fi
  expected_manifest="$(mktemp)"
  reference="$(mktemp -d)"
  trap 'rm -f "$expected_manifest"; rm -rf "$reference"' EXIT
  (
    cd "$output_dir"
    printf '%s  %s\n' "$(sha256_file "$tar_name")" "$tar_name"
    printf '%s  %s\n' "$(sha256_file "$zip_name")" "$zip_name"
  ) >"$expected_manifest"
  cmp "$expected_manifest" "$output_dir/checksums.txt"
  inspect_source_archive "$output_dir/$tar_name" tar "$archive_name"
  inspect_source_archive "$output_dir/$zip_name" zip "$archive_name"
  build_source_packages_raw "$reference" "$version" "$binary"
  cmp "$reference/$tar_name" "$output_dir/$tar_name"
  cmp "$reference/$zip_name" "$output_dir/$zip_name"
  cmp "$reference/checksums.txt" "$output_dir/checksums.txt"
  echo "source archive manifest/license boundary: pass"
)

build_packages() {
  build_source_packages_raw "$@"
  verify_source_packages "$@"
}

verify_package_reproducibility() (
  local first second
  first="$(mktemp -d)"
  second="$(mktemp -d)"
  trap 'rm -rf "$first" "$second"' EXIT
  build_source_packages_raw "$first" "contract" "inkbite"
  build_source_packages_raw "$second" "contract" "inkbite"
  diff -u "$first/checksums.txt" "$second/checksums.txt"
  while IFS= read -r artifact; do
    cmp "$first/$artifact" "$second/$artifact"
  done < <(awk '{print $2}' "$first/checksums.txt")
  printf 'package reproducibility: sha256:%s\n' "$(sha256_file "$first/checksums.txt")"
)

extract_api() {
  local repository="$1"
  local output="$2"
  (
    cd "$repository"
    GOTOOLCHAIN="$PINNED_TOOLCHAIN" go doc -all .
  ) |
    grep -E '^(const |var |type |func |[[:space:]]+[A-Z][A-Za-z0-9_]*([[:space:]]|[(]))' |
    sed -E 's/[[:space:]]+/ /g; s/^ //' |
    LC_ALL=C sort -u >"$output"
}

verify_public_api() {
  local temp_dir base_dir missing
  temp_dir="$(mktemp -d)"
  base_dir="$temp_dir/base"
  mkdir -p "$base_dir"
  git archive "$IMMUTABLE_BASE" | tar -x -C "$base_dir"
  extract_api "$base_dir" "$temp_dir/base.api"
  extract_api "$(pwd)" "$temp_dir/current.api"
  missing="$temp_dir/missing.api"
  comm -23 "$temp_dir/base.api" "$temp_dir/current.api" >"$missing"
  if [[ -s "$missing" ]]; then
    echo "public API declarations removed or changed from immutable base:" >&2
    sed -n '1,80p' "$missing" >&2
    rm -rf "$temp_dir"
    exit 1
  fi
  go test ./test/contract ./cmd/inkbite
  rm -rf "$temp_dir"
  echo "public API/downstream compatibility: pass"
}

verify_license_inventory() {
  grep -Fq "MIT License" LICENSE
  grep -Fq "GPL-3.0-only" ADOPTED_COMPONENTS.md
  verify_source_release_warning .
  local module version
  while read -r module version; do
    [[ -n "$module" ]] || continue
    grep -Fq "$module" ADOPTED_COMPONENTS.md
    grep -Fq "$version" ADOPTED_COMPONENTS.md
  done < <(go list -m -f '{{if and (not .Main) (not .Indirect)}}{{.Path}} {{.Version}}{{end}}' all)
  echo "MIT source/adopted-component dependency inventory: pass"
}

verify_release_surfaces() {
  local root="${1:-.}"
  local legacy="$root/scripts/dist.sh"
  local ci="$root/.github/workflows/ci.yml"
  local release="$root/.github/workflows/release.yml"
  verify_source_release_warning "$root"
  grep -Fq 'exec "$repo_root/scripts/verify-ingestion-contract.sh" package "$version" "$binary" "$dist_dir"' "$legacy"
  if grep -Eq 'go build|(^|[[:space:]])tar([[:space:]]|$)|(^|[[:space:]])zip([[:space:]]|$)' "$legacy"; then
    echo "legacy distribution script diverges from canonical source packaging" >&2
    return 1
  fi
  grep -Fq 'make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342' "$ci"
  grep -Fq 'make dist VERSION=ci' "$ci"
  grep -Fq 'dist/inkbite_ci_source.tar.gz' "$ci"
  grep -Fq 'dist/inkbite_ci_source.zip' "$ci"
  grep -Fq 'dist/checksums.txt' "$ci"
  grep -Fq 'make quality COVERAGE_BASE_REF=ee5542edd1ac64b5f66dcb9d0056dd4815739342' "$release"
  grep -Fq 'make dist VERSION=${GITHUB_REF_NAME}' "$release"
  grep -Fq 'dist/inkbite_${{ github.ref_name }}_source.tar.gz' "$release"
  grep -Fq 'dist/inkbite_${{ github.ref_name }}_source.zip' "$release"
  grep -Fq 'dist/checksums.txt' "$release"
  if grep -Eq 'dist/\*|dist/\*\.(tar\.gz|zip)|dist/\*\.tar\.gz' "$ci" "$release"; then
    echo "publication workflow contains a broad distribution glob" >&2
    return 1
  fi
  echo "source-only publication surfaces: pass"
}

verify_autocrlf_fixture() {
  local temp_dir checkout fixture current_size checkout_size current_hash checkout_hash
  temp_dir="$(mktemp -d)"
  checkout="$temp_dir/checkout"
  fixture="converters/pdf/testdata/simple.pdf"
  git clone --quiet --no-hardlinks --local --no-checkout "$(pwd)" "$checkout"
  git -C "$checkout" config core.autocrlf true
  git -C "$checkout" checkout --quiet --detach HEAD
  current_size="$(wc -c <"$fixture" | tr -d ' ')"
  checkout_size="$(wc -c <"$checkout/$fixture" | tr -d ' ')"
  current_hash="$(sha256_file "$fixture")"
  checkout_hash="$(sha256_file "$checkout/$fixture")"
  rm -rf "$temp_dir"
  if [[ "$current_size" != "$checkout_size" || "$current_hash" != "$checkout_hash" ]]; then
    echo "autocrlf binary fixture mismatch: $current_size/$current_hash vs $checkout_size/$checkout_hash" >&2
    exit 1
  fi
  printf 'autocrlf fixture: %s bytes sha256:%s\n' "$current_size" "$current_hash"
}

run_quality() {
  require_tool go
  require_tool git
  require_tool staticcheck
  require_tool govulncheck
  require_tool tar
  require_tool gzip
  require_tool zip
  require_tool unzip

  capture_quality_cleanliness

  printf 'quality base: %s\n' "$(git rev-parse "${COVERAGE_BASE_REF:-$IMMUTABLE_BASE}^{commit}")"
  go version
  staticcheck -version
  govulncheck -version
  git --version

  local unformatted
  unformatted="$(gofmt -l $(git ls-files '*.go'))"
  if [[ -n "$unformatted" ]]; then
    echo "gofmt required:" >&2
    echo "$unformatted" >&2
    exit 1
  fi
  go vet ./...
  go test ./...
  go test -race ./...
  go build ./...
  staticcheck ./...
  GOTOOLCHAIN="$PINNED_TOOLCHAIN" govulncheck ./...
  go mod verify
  verify_license_inventory
  verify_release_surfaces .
  verify_public_api
  scripts/changed-coverage.sh --self-test
  COVERAGE_BASE_REF="${COVERAGE_BASE_REF:-$IMMUTABLE_BASE}" scripts/changed-coverage.sh
  verify_autocrlf_fixture
  verify_package_reproducibility
  go generate ./...
  verify_quality_cleanliness
  echo "quality gate: pass"
}

case "${1:-quality}" in
  quality)
    run_quality
    ;;
  package)
    require_tool git
    require_tool tar
    require_tool gzip
    require_tool zip
    require_tool unzip
    build_packages "${4:-dist}" "${2:-dev}" "${3:-inkbite}"
    printf 'wrote %s/checksums.txt\n' "${4:-dist}"
    ;;
  verify-source-package)
    require_tool git
    require_tool tar
    require_tool gzip
    require_tool zip
    require_tool unzip
    verify_source_packages "${4:-dist}" "${2:-dev}" "${3:-inkbite}"
    ;;
  release-surfaces)
    verify_release_surfaces "${2:-.}"
    ;;
  package-reproducibility)
    verify_package_reproducibility
    ;;
  api-license)
    verify_license_inventory
    verify_public_api
    ;;
  autocrlf-fixture)
    verify_autocrlf_fixture
    ;;
  *)
    echo "usage: $0 [quality|package VERSION BINARY DIST_DIR|verify-source-package VERSION BINARY DIST_DIR|release-surfaces ROOT|package-reproducibility|api-license|autocrlf-fixture]" >&2
    exit 2
    ;;
esac
