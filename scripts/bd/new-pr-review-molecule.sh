#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# new-pr-review-molecule.sh — instantiate the PR review molecule for one PR.
#
# Usage:
#   scripts/bd/new-pr-review-molecule.sh 482            # create it
#   scripts/bd/new-pr-review-molecule.sh 482 --dry-run  # show what would be created
#
# Substitutes {{PR}} in .beads/plans/pr-review-molecule.json and feeds the
# result to `bd create --graph`. bd does not read the plan from stdin, so the
# resolved plan is written to a temp file first.
#
# POSIX sh — no bashisms.

set -eu

PR="${1:-}"
if [ -z "$PR" ]; then
  echo "usage: $0 <pr-number> [--dry-run]" >&2
  exit 2
fi
shift

case "$PR" in
  *[!0-9]* | '')
    echo "$0: PR number must be numeric, got '$PR'" >&2
    exit 2
    ;;
esac

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
PLAN="$ROOT/.beads/plans/pr-review-molecule.json"

if [ ! -f "$PLAN" ]; then
  echo "$0: plan not found: $PLAN" >&2
  exit 1
fi

RESOLVED="$(mktemp -t pr-review-molecule.XXXXXX.json)"
trap 'rm -f "$RESOLVED"' EXIT INT TERM

# Substitute {{PR}}, and drop the "_*comment" lines: JSON has no comment syntax,
# so notes live in those keys and bd would warn about them on every run.
sed -e "s/{{PR}}/$PR/g" -e '/^  "_[a-z_]*comment":/d' "$PLAN" >"$RESOLVED"

bd -C "$ROOT" create --graph "$RESOLVED" "$@"
