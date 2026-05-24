#!/usr/bin/env bash
# Pre-compact hook for the OpenSpec journal.
#
# Emits a `context.compacted` event into the journal of the most recently
# active OpenSpec change, if one exists. Stays silent (exit 0) when there
# is nothing to log so it does not block compaction.
#
# Claude Code wires this as a `PreCompact` hook. Other agent clients can
# call the same script from an equivalent lifecycle hook; set
# OPEN_SPEC_JOURNAL_AGENT to label the emitted event, for example:
#
#   OPEN_SPEC_JOURNAL_AGENT=Codex /abs/path/to/scripts/openspec-journal-precompact-hook.sh
#
# Wire via `~/.claude/settings.json` or `.claude/settings.json`:
#
#   {
#     "hooks": {
#       "PreCompact": [
#         {"hooks": [{"type": "command",
#           "command": "/abs/path/to/scripts/openspec-journal-precompact-hook.sh"
#         }]}
#       ]
#     }
#   }
#
# The script is intentionally tolerant: any failure (no repo, no active
# change, journal helper missing) is silently ignored so compaction is
# never blocked by journaling.

set -u

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
[ -n "${repo_root}" ] || exit 0

helper="${repo_root}/scripts/openspec-journal.py"
changes_dir="${repo_root}/openspec/changes"
agent="${OPEN_SPEC_JOURNAL_AGENT:-Claude Code}"
[ -x "${helper}" ] && [ -d "${changes_dir}" ] || exit 0

# Pick the most recently modified non-archive journal.
journal=$(find "${changes_dir}" -mindepth 2 -maxdepth 3 -name journal.jsonl \
    -not -path "*/archive/*" -print 2>/dev/null \
    | xargs -I {} stat -f '%m {}' {} 2>/dev/null \
    | sort -rn | head -n 1 | awk '{ $1=""; print substr($0,2) }')

[ -n "${journal}" ] || exit 0

# Only journal compactions for changes touched in the last 6 hours; older
# trails belong to changes the user has likely moved on from.
journal_mtime=$(stat -f '%m' "${journal}" 2>/dev/null || stat -c '%Y' "${journal}" 2>/dev/null || true)
now=$(date +%s 2>/dev/null || true)
case "${journal_mtime}" in ''|*[!0-9]*) exit 0 ;; esac
case "${now}" in ''|*[!0-9]*) exit 0 ;; esac
if [ "$((now - journal_mtime))" -gt 21600 ]; then
    exit 0
fi

change=$(basename "$(dirname "${journal}")")

"${helper}" "${change}" context.compacted \
    input="${agent} PreCompact hook fired during ${change}." \
    output="Context compaction observed; chat continuation may follow." \
    >/dev/null 2>&1 || true

exit 0
