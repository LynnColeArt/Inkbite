#!/usr/bin/env bash
set -euo pipefail

readonly IMMUTABLE_BASE="ee5542edd1ac64b5f66dcb9d0056dd4815739342"

calculate_coverage() {
  local ranges_file="$1"
  local profile_file="$2"
  awk '
    NR == FNR {
      range_file[++range_count] = $1
      range_start[range_count] = $2
      range_end[range_count] = $3
      next
    }
    FNR == 1 { next }
    {
      location = $1
      profile_statements[location] = $(NF - 1)
      if ($NF > profile_executions[location]) {
        profile_executions[location] = $NF
      }
    }
    END {
      for (location in profile_statements) {
      statements = profile_statements[location]
      executions = profile_executions[location]
      file = location
      sub(/:[0-9]+[.][0-9]+,[0-9]+[.][0-9]+$/, "", file)
      span = location
      sub(/^.*:/, "", span)
      split(span, endpoints, ",")
      split(endpoints[1], start_parts, ".")
      split(endpoints[2], end_parts, ".")
      for (range_index = 1; range_index <= range_count; range_index++) {
        suffix = "/" range_file[range_index]
        file_matches = file == range_file[range_index] ||
          (length(file) >= length(suffix) && substr(file, length(file) - length(suffix) + 1) == suffix)
        overlaps = start_parts[1] <= range_end[range_index] && end_parts[1] >= range_start[range_index]
        if (file_matches && overlaps) {
          total += statements
          if (executions > 0) {
            covered += statements
          }
          break
        }
      }
      }
      printf "%d %d\n", covered, total
    }
  ' "$ranges_file" "$profile_file"
}

self_test() {
  local temp_dir
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "$temp_dir"' RETURN
  printf 'sample.go 10 12\n' >"$temp_dir/ranges"
  printf 'mode: atomic\nexample/sample.go:10.1,10.2 1 1\nexample/sample.go:11.1,11.2 4 0\n' >"$temp_dir/profile-red"
  printf 'mode: atomic\nexample/sample.go:10.1,10.2 1 1\nexample/sample.go:11.1,11.2 4 1\n' >"$temp_dir/profile-green"
  [[ "$(calculate_coverage "$temp_dir/ranges" "$temp_dir/profile-red")" == "1 5" ]]
  [[ "$(calculate_coverage "$temp_dir/ranges" "$temp_dir/profile-green")" == "5 5" ]]
  echo "changed coverage mutation self-test: red 1/5, green 5/5"
}

if [[ "${1:-}" == "--self-test" ]]; then
  self_test
  exit 0
fi

base_ref="${COVERAGE_BASE_REF:-$IMMUTABLE_BASE}"
resolved_base="$(git rev-parse "${base_ref}^{commit}")"
if [[ "$resolved_base" != "$IMMUTABLE_BASE" ]]; then
  echo "coverage base must resolve to immutable commit $IMMUTABLE_BASE, got $resolved_base" >&2
  exit 1
fi
git merge-base --is-ancestor "$resolved_base" HEAD

temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT
ranges_file="$temp_dir/changed-ranges"
profile_file="$temp_dir/coverage.out"

while IFS= read -r file; do
  [[ -n "$file" ]] || continue
  git diff --unified=0 --no-color "$resolved_base..HEAD" -- "$file" |
    awk -v file="$file" '
      /^@@ / {
        added = $0
        sub(/^.*[+]/, "", added)
        sub(/ .*/, "", added)
        split(added, parts, ",")
        start = parts[1] + 0
        count = (parts[2] == "" ? 1 : parts[2] + 0)
        if (count > 0) {
          print file, start, start + count - 1
        }
      }
    ' >>"$ranges_file"
done < <(
  git diff --name-only --diff-filter=AMR "$resolved_base..HEAD" -- '*.go' |
    grep -Ev '(^|/)(test|tests)/|_test[.]go$|^cmd/' || true
)

if [[ ! -s "$ranges_file" ]]; then
  echo "changed production coverage gate is vacuous: no changed Go ranges" >&2
  exit 1
fi

go test -covermode=atomic -coverpkg=./... -coverprofile="$profile_file" ./...
read -r covered total < <(calculate_coverage "$ranges_file" "$profile_file")
if (( total == 0 )); then
  echo "changed production coverage gate is vacuous: zero instrumented statements" >&2
  exit 1
fi
percentage="$(awk -v covered="$covered" -v total="$total" 'BEGIN { printf "%.6f", covered * 100 / total }')"
printf 'changed production coverage: %d/%d = %s%% (base %s)\n' "$covered" "$total" "$percentage" "$resolved_base"
if (( covered * 1000 < total * 800 )); then
  echo "changed production coverage is below the unrounded 80.0% floor" >&2
  exit 1
fi
