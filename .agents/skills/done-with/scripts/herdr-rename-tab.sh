#!/bin/sh
# Rename the herdr tab that hosts THIS agent session — never the focused one.
#
# herdr exports HERDR_PANE_ID into every pane it spawns, so it identifies the
# caller's own pane regardless of what the user has focused. Resolving the tab
# from `herdr api snapshot`'s focused_tab_id instead renames whichever tab
# happens to be in front when the command lands.
#
# Exit codes: 0 renamed, 3 not inside herdr, 4 pane/tab unresolvable,
#             5 rename rejected. All non-zero cases are non-fatal for the
#             caller: report and continue.
set -eu

label="${1:?usage: herdr-rename-tab.sh <label>}"

if [ -z "${HERDR_PANE_ID:-}" ]; then
  echo "herdr: HERDR_PANE_ID unset — this session is not running inside a herdr pane; skipping tab rename" >&2
  exit 3
fi

pane_json=$(herdr pane get "$HERDR_PANE_ID" 2>/dev/null) || {
  echo "herdr: pane $HERDR_PANE_ID not found (server not running, or the pane is gone)" >&2
  exit 4
}

tab_id=$(printf '%s' "$pane_json" | python3 -c '
import json, sys
try:
    print(json.load(sys.stdin)["result"]["pane"]["tab_id"])
except Exception:
    raise SystemExit(1)
') || {
  echo "herdr: could not read tab_id for pane $HERDR_PANE_ID" >&2
  exit 4
}

herdr tab rename "$tab_id" "$label" >/dev/null || {
  echo "herdr: rename of tab $tab_id to '$label' failed" >&2
  exit 5
}

echo "herdr: tab $tab_id (pane $HERDR_PANE_ID) renamed to '$label'"
