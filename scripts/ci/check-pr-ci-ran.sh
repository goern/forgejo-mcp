#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-pr-ci-ran.sh — assert that CI actually RAN for a pull request.
#
# The trap this exists for: on an external contributor's PR, Pipelines-as-Code
# refuses to start a PipelineRun unless the author is listed in OWNERS. It says
# so in a PR comment and leaves exactly one commit status behind:
#
#     context: "op1st Pipelines as Code"
#     state:   pending
#     description: "Pending approval, waiting for an /ok-to-test"
#
# A reviewer scanning the PR sees no RED checks and concludes "CI is fine".
# It is not fine — there are NO checks. "Not red" and "green" are different
# states, and only this one is visible at a glance.
#
# This script shells out to the forge's own combined-status API rather than
# re-deriving anything: one source of truth, and it reports every context, not
# just the one that bit us.
#
# Usage:
#   scripts/ci/check-pr-ci-ran.sh <pr-number> [owner/repo]
#   scripts/ci/check-pr-ci-ran.sh --sha <commit-sha> [owner/repo]
#
# Environment:
#   FORGEJO_URL    forge base URL   (default https://git.b4mad.industries)
#   FORGEJO_TOKEN  optional; only needed for private repos
#
# Exit codes:
#   0  every reported pipeline context succeeded
#   1  CI never ran, is still awaiting /ok-to-test, is pending, or failed
#   0  (with a warning) when curl or jq is absent — contributors without the
#      tools are not blocked, the same convention openspec-validate.sh uses
#
# POSIX sh — no bashisms — so it behaves identically in a local shell and in
# the minimal release-tools CI image.

set -eu

FORGE="${FORGEJO_URL:-https://git.b4mad.industries}"
DEFAULT_REPO="agentic-forges/forgejo-mcp"

# The placeholder PaC leaves when it declines to start a run. Matching on the
# description rather than the context, because the context is the same string
# PaC uses for real run reporting.
AWAITING_RE='waiting for an /ok-to-test'

usage() {
  echo "usage: $0 <pr-number> [owner/repo]" >&2
  echo "       $0 --sha <commit-sha> [owner/repo]" >&2
  exit 2
}

for tool in curl jq; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "check-pr-ci-ran: $tool not found on PATH — skipping." >&2
    echo "check-pr-ci-ran: install curl and jq to verify CI ran before reviewing." >&2
    exit 0
  fi
done

SHA=""
PR=""
case "${1:-}" in
  "") usage ;;
  --sha)
    SHA="${2:-}"
    [ -n "$SHA" ] || usage
    REPO="${3:-$DEFAULT_REPO}"
    ;;
  -*) usage ;;
  *)
    PR="$1"
    REPO="${2:-$DEFAULT_REPO}"
    ;;
esac

api() {
  # $1 = path below /api/v1
  if [ -n "${FORGEJO_TOKEN:-}" ]; then
    curl -sS -H "Authorization: token ${FORGEJO_TOKEN}" "${FORGE}/api/v1/$1"
  else
    curl -sS "${FORGE}/api/v1/$1"
  fi
}

if [ -z "$SHA" ]; then
  pr_json="$(api "repos/${REPO}/pulls/${PR}")"
  SHA="$(printf '%s' "$pr_json" | jq -r '.head.sha // empty')"
  if [ -z "$SHA" ]; then
    echo "check-pr-ci-ran: could not resolve head SHA for ${REPO}#${PR}." >&2
    printf '%s\n' "$pr_json" | head -c 400 >&2
    echo >&2
    exit 1
  fi
  echo "check-pr-ci-ran: ${REPO}#${PR} head is ${SHA}"
else
  echo "check-pr-ci-ran: ${REPO} at ${SHA}"
fi

status_json="$(api "repos/${REPO}/commits/${SHA}/status")"
total="$(printf '%s' "$status_json" | jq -r '.total_count // 0')"

if [ "$total" -eq 0 ]; then
  cat >&2 <<EOF

check-pr-ci-ran: NO commit statuses at all for ${SHA}.

CI did not run. This is not a passing PR — it is an unverified one. Do not
read "no red checks" as "checks passed".
EOF
  exit 1
fi

printf '%s' "$status_json" |
  jq -r '.statuses[] | "  \(.status)\t\(.context)\t\(.description)"'

# Statuses that are real pipeline reports, i.e. everything except the
# "waiting for an /ok-to-test" placeholder.
real="$(printf '%s' "$status_json" |
  jq --arg re "$AWAITING_RE" '[.statuses[] | select((.description // "") | contains($re) | not)] | length')"

if [ "$real" -eq 0 ]; then
  cat >&2 <<EOF

check-pr-ci-ran: CI is BLOCKED, not passing.

The only status on ${SHA} is the Pipelines-as-Code placeholder: it refused to
start a PipelineRun because the PR author is not in the repo's OWNERS file.

Fix: a maintainer must comment

    /ok-to-test

on the pull request. Then re-run this script — the head SHA does not change,
but real pipeline contexts will appear.
EOF
  exit 1
fi

failed="$(printf '%s' "$status_json" |
  jq --arg re "$AWAITING_RE" '[.statuses[]
     | select((.description // "") | contains($re) | not)
     | select(.status != "success")] | length')"

if [ "$failed" -ne 0 ]; then
  echo >&2
  echo "check-pr-ci-ran: ${failed} pipeline context(s) not successful on ${SHA}." >&2
  exit 1
fi

echo "check-pr-ci-ran: OK — ${real} pipeline context(s) succeeded on ${SHA}."
exit 0
