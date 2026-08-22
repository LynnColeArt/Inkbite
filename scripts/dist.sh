#!/usr/bin/env bash
set -euo pipefail

version="${1:-${VERSION:-dev}}"
binary="${2:-${BINARY:-inkbite}}"
dist_dir="${DIST_DIR:-dist}"
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

cd "$repo_root"
exec "$repo_root/scripts/verify-ingestion-contract.sh" package "$version" "$binary" "$dist_dir"
