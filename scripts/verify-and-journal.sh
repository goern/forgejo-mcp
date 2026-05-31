#!/usr/bin/env bash
# verify-and-journal.sh — run a verifier command and journal its verdict.
#
# Wraps the dev-loop verifier so every run appends a `verifier.result`
# event to the change's OpenSpec journal — deterministically, without
# relying on an agent remembering to log. This is what lets the Spec
# Quality Index (docs/kpi/spec-quality-index.md) observe Implementability:
# each FAIL->fix round becomes a journaled line, so `sqi.py scan-journal`
# can count rework (S5). Without it the apply run is unjournaled and SQI
# can only report Fidelity at partial confidence.
#
# Usage:
#   scripts/verify-and-journal.sh <change> <task-ref> -- <verifier cmd...>
#
# Example (the add-dashboard-auto-refresh verifier):
#   scripts/verify-and-journal.sh add-dashboard-auto-refresh 7 -- \
#     'npm run lint && npx svelte-check --tsconfig ./tsconfig.json && \
#      DATABASE_URL=mysql://root@localhost:3307/beads_tv node scripts/smoke-watch.ts && \
#      npm run build'
#
# Notes:
# - The verifier command is passed as one or more args after `--` and run
#   via `bash -c` so an `&&`-chain yields a single combined exit code.
# - On exit 0 -> note=pass, else note=fail. The wrapper always exits with
#   the verifier's real exit code, so it is drop-in for a CI/loop check.
# - Journaling failures (no change dir, helper missing) are swallowed so a
#   green verifier is never turned red by a logging hiccup.
# - `decision`, `task.blocked`, and escalation (S6) are NOT derivable from
#   an exit code; the actor must log those (see AGENTS.md / the dev-loop
#   spawn prompts). This wrapper covers the S5 rework signal only.

set -u

if [ "$#" -lt 3 ]; then
	echo "usage: $0 <change> <task-ref> -- <verifier cmd...>" >&2
	exit 1
fi

change="$1"
ref="$2"
shift 2
[ "${1:-}" = "--" ] && shift

if [ "$#" -eq 0 ]; then
	echo "error: no verifier command given after --" >&2
	exit 1
fi

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
helper="${repo_root}/scripts/openspec-journal.py"

# Run the verifier as one unit so an &&-chain collapses to one exit code.
bash -c "$*"
rc=$?

note=$([ "$rc" -eq 0 ] && echo pass || echo fail)

if [ -x "$helper" ] || [ -f "$helper" ]; then
	python3 "$helper" "$change" verifier.result \
		ref="$ref" note="$note" \
		input="canonical verifier run" \
		output="exit=$rc" \
		>/dev/null 2>&1 || true
fi

exit "$rc"
