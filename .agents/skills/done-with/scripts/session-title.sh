#!/bin/sh
# Keep the Claude Code session name in step with the herdr tab label.
#
# A session cannot rename itself mid-turn: `sessionTitle` is only accepted from
# the UserPromptSubmit and SessionStart hooks — /rename is a human-only builtin.
# So the skills *queue* the label here, and a UserPromptSubmit hook applies it
# on the user's next message. One prompt of latency, no guessing.
#
# Modes:
#   session-title.sh queue <label>   — called by /work-on and /done-with
#   session-title.sh hook            — wired into settings.json; hook JSON on stdin
#
# Exit codes (queue): 0 queued, 3 no session id in the environment (not running
# under Claude Code) — non-fatal for the caller, report and continue.
# Hook mode always exits 0: a UserPromptSubmit hook must never block a prompt.
set -eu

queue_dir() { printf '%s/session-title-queue' "${CLAUDE_CONFIG_DIR:-$HOME/.claude}"; }

case "${1:-}" in
queue)
  label="${2:?usage: session-title.sh queue <label>}"
  sid="${CLAUDE_CODE_SESSION_ID:-}"
  if [ -z "$sid" ]; then
    echo "session-title: CLAUDE_CODE_SESSION_ID unset — not inside a Claude Code session; skipping session rename" >&2
    exit 3
  fi
  d=$(queue_dir)
  mkdir -p "$d"
  printf '%s' "$label" >"$d/$sid"
  # Sessions that never send another prompt leave their queue file behind.
  find "$d" -type f -mtime +7 -delete 2>/dev/null || true
  echo "session-title: queued '$label' for session $sid (applies on the next user message)"
  ;;
hook)
  input=$(cat)
  sid=$(printf '%s' "$input" | python3 -c '
import json, sys
try:
    print(json.load(sys.stdin).get("session_id", ""))
except Exception:
    pass
' 2>/dev/null) || exit 0
  [ -n "$sid" ] || exit 0
  f="$(queue_dir)/$sid"
  [ -f "$f" ] || exit 0
  label=$(cat "$f" 2>/dev/null) || exit 0
  rm -f "$f"
  [ -n "$label" ] || exit 0
  python3 -c '
import json, sys
print(json.dumps({"hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit", "sessionTitle": sys.argv[1]}}))
' "$label"
  ;;
*)
  echo "usage: session-title.sh queue <label> | session-title.sh hook" >&2
  exit 2
  ;;
esac
