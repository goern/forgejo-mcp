#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-castra-release-onboarding.sh — verify castra:release will actually
# fire on this repo, before anyone spends a label-toggle finding out the hard
# way.
#
# The trap this exists for: labeling an issue `castra:release` fails SILENTLY
# — no comment, no error, nothing — for any of three independent reasons, and
# from the forge UI they all look identical. Investigated live on
# agentic-forges/forgejo-mcp#523 (2026-08-21):
#
#   1. The label was added AT ISSUE CREATION. Forgejo's create-with-labels
#      webhook does not reliably produce the `label_updated` action castra's
#      EventListener CEL filter matches — add the label to an EXISTING issue
#      instead (remove + re-add if it's already there).
#   2. OWNERS has no `release-manager:` list. `castra-release-authz` fails
#      CLOSED on a missing file, missing key, or empty list — silently, before
#      the persona ever runs.
#   3. No Forgejo webhook is registered on this repo pointing at
#      https://webhooks.castra.b4mad.industries. Without it nothing is ever
#      delivered, regardless of (1) or (2).
#
# This script checks (2) and (3) — the two that are true/false facts about
# the repo, independent of any specific issue. (1) is a workflow reminder,
# printed unconditionally, not something a script can check ahead of time.
#
# Usage:
#   scripts/ci/check-castra-release-onboarding.sh [owner/repo]
#
# Environment:
#   FORGEJO_URL    forge base URL   (default https://git.b4mad.industries)
#   FORGEJO_TOKEN  required for the webhook check (hook listing is
#                  admin-only); the OWNERS check works without it on a public
#                  repo.
#
# Exit codes:
#   0  release-manager list is non-empty AND a matching active webhook exists
#   1  either check ran and failed
#   0  (with a warning) when curl/jq, or FORGEJO_TOKEN for the webhook check,
#      are unavailable — same convention as check-pr-ci-ran.sh: don't block
#      a contributor who can't run the full check, just say so loudly

set -eu

FORGE="${FORGEJO_URL:-https://git.b4mad.industries}"
REPO="${1:-agentic-forges/forgejo-mcp}"
WEBHOOK_URL="https://webhooks.castra.b4mad.industries"

fail=0

for tool in curl jq; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "check-castra-release-onboarding: $tool not found on PATH — skipping." >&2
    exit 0
  fi
done

api() {
  # $1 = path below /api/v1
  if [ -n "${FORGEJO_TOKEN:-}" ]; then
    curl -sS -H "Authorization: token ${FORGEJO_TOKEN}" "${FORGE}/api/v1/$1"
  else
    curl -sS "${FORGE}/api/v1/$1"
  fi
}

echo "check-castra-release-onboarding: ${REPO} against ${FORGE}"
echo

# ── 1. OWNERS release-manager list ──────────────────────────────────────
owners_json="$(api "repos/${REPO}/contents/OWNERS?ref=main" || true)"
owners_b64="$(printf '%s' "$owners_json" | jq -r '.content // empty' 2>/dev/null || true)"

if [ -z "$owners_b64" ]; then
  echo "✗ OWNERS: could not fetch OWNERS from ${REPO}@main (missing file, or private repo without FORGEJO_TOKEN)."
  fail=1
else
  managers="$(printf '%s' "$owners_b64" | base64 -d 2>/dev/null |
    awk '/^release-manager:/{f=1;next} /^[a-zA-Z]/{f=0} f && /^[[:space:]]*-/{print}')"
  if [ -z "$managers" ]; then
    echo "✗ OWNERS: no non-empty 'release-manager:' list. castra-release-authz fails"
    echo "  closed on this — add one, e.g.:"
    echo "      release-manager:"
    echo "        - <your-forge-username>"
    fail=1
  else
    echo "✓ OWNERS: release-manager list present:"
    printf '%s\n' "$managers" | sed 's/^/    /'
  fi
fi

echo

# ── 2. Forgejo webhook to castra's EventListener ────────────────────────
if [ -z "${FORGEJO_TOKEN:-}" ]; then
  echo "⚠ webhook: FORGEJO_TOKEN not set — cannot list repo hooks (admin-only"
  echo "  endpoint). Skipping this check; verify manually with:"
  echo "    forgejo-mcp / mcp__*__list_repo_hooks owner=<owner> repo=<repo>"
else
  hooks_json="$(api "repos/${REPO}/hooks" || true)"
  match="$(printf '%s' "$hooks_json" | jq --arg u "$WEBHOOK_URL" \
    '[.[]? | select(.type=="forgejo" and .active==true and (.config.url // .url // "")==$u)] | length' 2>/dev/null || echo 0)"

  if [ "${match:-0}" -ge 1 ] 2>/dev/null; then
    echo "✓ webhook: active forgejo webhook -> ${WEBHOOK_URL} is registered."
  else
    echo "✗ webhook: no active forgejo webhook -> ${WEBHOOK_URL} found on ${REPO}."
    echo "  Without it, no issue-label event ever reaches castra — silently."
    echo "  Create one (Settings -> Webhooks -> Forgejo, or via MCP"
    echo "  create_repo_hook): POST, application/json, events=issues."
    fail=1
  fi
fi

cat <<'EOF'

Reminder (cannot be checked ahead of time — depends on how you label):
  Add castra:release to an EXISTING issue. Never create the issue with the
  label already attached — that emits `action: opened`, which the Forgejo
  EventListener filter (action == 'label_updated') ignores. If the label is
  already on the issue and nothing happened, remove it and re-add it.
EOF

exit "$fail"
