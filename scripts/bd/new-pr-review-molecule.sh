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

CREATED="$(mktemp -t pr-review-molecule-out.XXXXXX)"
trap 'rm -f "$RESOLVED" "$CREATED"' EXIT INT TERM

# Not piped into tee: `set -e` does not see a failure on the left of a pipe in
# POSIX sh, and a failed create must still abort.
bd -C "$ROOT" create --graph "$RESOLVED" "$@" >"$CREATED"
cat "$CREATED"

# bd prints one "  <key> -> <id>" line per created node. --dry-run prints none,
# so no prompt is emitted for a dry run — there is nothing to work on yet.
EPIC="$(sed -n 's/^  epic -> \([A-Za-z0-9-]*\)$/\1/p' "$CREATED" | head -1)"
[ -n "$EPIC" ] || exit 0

cat <<EOF

────────────────────────────────────────────────────────────────────────
Paste this into Claude Code to run the review:
────────────────────────────────────────────────────────────────────────

Goal: complete the review of pull request #$PR on agentic-forges/forgejo-mcp,
end to end, until every task in the review molecule is closed.

The root issue is $EPIC. Read it first — it carries the standing rules that
apply to every child (determinism, treating the contributor as a guest, and
coordinating with castra through the per-issue mutex):

    bd show $EPIC
    bd graph $EPIC --compact

Then loop until there is no ready work left under that epic:

1.  bd ready --parent $EPIC     # pick the highest-priority ready child
2.  bd show <id>                # its description IS the instructions: it names
                                # the skill, agent, or castra command to use
3.  bd update <id> --claim
4.  Do the work exactly as described. Do not skip a step because it looks
    like paperwork, and do not run a later step before its blocker closes —
    the edges encode a real order.
5.  bd close <id>
6.  Repeat from 1. Stop only when 'bd ready --parent $EPIC' is empty.

The last ready task will be closeout: it pushes, releases the castra mutex,
removes the scratch worktrees, and writes the handoff. The review is not done
until that one is closed.

If a task turns out to be inapplicable to this PR (e.g. CI is green, so there
is no failure to reproduce), say so and close it with a note explaining why —
do not silently leave it open.
EOF
