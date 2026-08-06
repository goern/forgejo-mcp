#!/usr/bin/env bash
# Install this skill into a project, for `npx skills` and Claude Code both.
#
# WHY a script and not `npx skills add`: that command works fine on a tarball
# of this directory (15 KB, verified) — but nothing publishes one yet, and it
# cannot be pointed at the repo instead. For a non-GitHub host it downloads the
# URL rather than cloning, and this repo's only archive is the whole-repo
# tarball: 12.9 MB across 2888 entries, because .agents/skills/ (other people's
# skills) is committed here. Both exceed its caps of 10 MB and 1000 files.
#
# So: symlink the directory. That is also what makes the skill work airgapped
# and what keeps it current — the link follows the checkout, with no copy to
# drift. It is the same layout `npx skills` produces:
#
#   .agents/skills/<name>        canonical, what `npx skills` reads
#   .claude/skills/<name> -> ../../.agents/skills/<name>
#
# Usage:  ./install.sh [target-project]      (default: the cwd)

set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAME="$(basename "$SKILL_DIR")"
TARGET="$(cd "${1:-$PWD}" && pwd)"

if [ "$TARGET" = "$(cd "$SKILL_DIR/../.." && pwd)" ]; then
    echo "✗ $TARGET is forge-agents itself — the skill is already here." >&2
    exit 1
fi

mkdir -p "$TARGET/.agents/skills" "$TARGET/.claude/skills"
ln -sfn "$SKILL_DIR" "$TARGET/.agents/skills/$NAME"
ln -sfn "../../.agents/skills/$NAME" "$TARGET/.claude/skills/$NAME"

# Absolute link into the checkout, so it breaks loudly if the checkout moves
# rather than silently serving a stale copy. Prove it resolves before claiming
# success — a dangling symlink installs just as quietly as a good one.
test -f "$TARGET/.claude/skills/$NAME/SKILL.md" \
    || { echo "✗ $TARGET/.claude/skills/$NAME does not resolve" >&2; exit 1; }
test -f "$TARGET/.claude/skills/$NAME/job-semantic-release.yaml" \
    || { echo "✗ the Job manifest did not come with it" >&2; exit 1; }
for t in justfile-block.just agents-md-block.md; do
    test -f "$TARGET/.claude/skills/$NAME/$t" \
        || { echo "✗ the $t template did not come with it" >&2; exit 1; }
done

echo "✔ $NAME installed into $TARGET (linked to $SKILL_DIR)"
echo "  next: $TARGET/.claude/skills/$NAME/enable-semantic-release.py \\"
echo "          \"\$(git -C $TARGET remote get-url origin)\" --dry-run"
