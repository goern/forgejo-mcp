#!/usr/bin/env bash
#
# check-spec-demo-anchors.sh — validate anchored Showboat demos against specs.
#
# For every spec.md under openspec/ that opts in with `<!-- demos-anchored: true -->`
# (before its first H2), this checker enforces the anchored-demo contract from
# the `showboat` skill:
#
#   1. The spec has a sibling demo file (demo.md or *.demo.md in the same dir).
#   2. Every `#### Scenario:` heading in the spec has a matching machine anchor
#      `<!-- spec-scenario: <capability>#<slug> -->` in the demo.
#   3. Each machine anchor has a matching human anchor link (`#scenario-<slug>`).
#   4. Each proof block carries at least one evidence block (a fenced code block
#      or an `<!-- evidence-kind: ... -->` marker) before the next anchor.
#
# <capability> is the spec's directory name. <slug> is the GitHub-style
# slugification of the scenario heading text (lowercase; non-alphanumeric runs
# collapsed to a single '-'; leading/trailing '-' trimmed).
#
# Exit 0 when all anchored specs pass (including the trivial "no anchored specs
# yet" case). Exit 1 on any violation.

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

errors=0
specs_checked=0
scenarios_checked=0

err() {
  printf 'FAIL: %s\n' "$1" >&2
  errors=$((errors + 1))
}

# GitHub-style slug: lowercase, non-alnum runs -> '-', trim leading/trailing '-'.
slugify() {
  printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//'
}

# Does the spec opt in? Marker must appear before the first H2 ('## ').
is_anchored() {
  awk '
    /^## / { exit }
    /<!-- demos-anchored: true -->/ { found = 1; exit }
    END { exit (found ? 0 : 1) }
  ' "$1"
}

# Find the sibling demo for a spec.md in the same directory.
find_demo() {
  local dir="$1" d
  if [[ -f "$dir/demo.md" ]]; then
    printf '%s\n' "$dir/demo.md"
    return 0
  fi
  for d in "$dir"/*.demo.md; do
    [[ -e "$d" ]] || continue
    printf '%s\n' "$d"
    return 0
  done
  return 1
}

shopt -s nullglob

# Collect spec.md files under openspec/ (specs + change deltas).
mapfile -t spec_files < <(find openspec -type f -name 'spec.md' 2>/dev/null | sort)

for spec in "${spec_files[@]}"; do
  is_anchored "$spec" || continue
  specs_checked=$((specs_checked + 1))

  dir="$(dirname "$spec")"
  capability="$(basename "$dir")"

  if ! demo="$(find_demo "$dir")"; then
    err "$spec is anchored (demos-anchored: true) but has no sibling demo (demo.md or *.demo.md in $dir)"
    continue
  fi

  # Every '#### Scenario:' heading in the spec.
  while IFS= read -r heading; do
    [[ -n "$heading" ]] || continue
    scenarios_checked=$((scenarios_checked + 1))
    slug="$(slugify "$heading")"

    machine="<!-- spec-scenario: ${capability}#${slug} -->"
    if ! grep -qF "$machine" "$demo"; then
      err "$demo missing machine anchor for scenario \"$heading\" (expected: $machine)"
      continue
    fi

    if ! grep -qF "#scenario-${slug}" "$demo"; then
      err "$demo missing human anchor link '#scenario-${slug}' for scenario \"$heading\""
    fi

    # Evidence: between this machine anchor and the next anchor (or EOF), there
    # must be a fenced code block or an evidence-kind marker.
    if ! awk -v anchor="$machine" '
      index($0, anchor) { inblk = 1; next }
      inblk && /<!-- spec-scenario:/ { exit }
      inblk && (/^```/ || /<!-- evidence-kind:/) { found = 1; exit }
      END { exit (found ? 0 : 1) }
    ' "$demo"; then
      err "$demo proof for scenario \"$heading\" has no evidence block (fenced code or <!-- evidence-kind: ... -->)"
    fi
  done < <(sed -nE 's/^#### Scenario:[[:space:]]*//p' "$spec")
done

echo "check-demos: ${specs_checked} anchored spec(s), ${scenarios_checked} scenario(s) checked, ${errors} error(s)."

if [[ "$errors" -gt 0 ]]; then
  exit 1
fi
exit 0
