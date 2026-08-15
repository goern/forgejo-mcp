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
# Two kinds of context come back, and they mean different things:
#
#   "op1st Pipelines as Code"              PaC's own gating slot: absent when no
#                                          approval is needed, the placeholder
#                                          above when it is, "Success" once
#                                          unblocked, "Failed" when PaC could
#                                          not process the event at all.
#   "op1st Pipelines as Code / <run>"      one actual PipelineRun.
#
# Only the second kind is a pipeline result, so only the second kind decides
# this script's exit code. The gating slot is reported, never counted — when
# PaC fails before creating any PipelineRun it writes a top-level failure that
# no later run ever overwrites, so a retest that goes fully green leaves the
# forge reporting aggregate=failure until the head SHA changes. Counting that
# status would make this guard call a green PR red forever. Ignoring it
# silently would be worse, so a top-level failure alongside real runs is
# printed as a warning: the runs that exist passed, but a manifest that failed
# to parse has no run to report and its absence cannot be seen from here.
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

# PaC's gating context. Real PipelineRuns report under "<this> / <run-name>".
PAC_CONTEXT='op1st Pipelines as Code'

# The placeholder PaC leaves in the gating context when it declines to start a
# run. Matched on the description, because the state alone does not say why.
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

# Real pipeline reports: every context except PaC's own gating slot. Any other
# status provider still counts, so a third party reporting failure is not
# quietly dropped.
runs="$(printf '%s' "$status_json" |
  jq --arg c "$PAC_CONTEXT" '[.statuses[] | select(.context != $c)] | length')"

if [ "$runs" -eq 0 ]; then
  awaiting="$(printf '%s' "$status_json" |
    jq --arg c "$PAC_CONTEXT" --arg re "$AWAITING_RE" '[.statuses[]
       | select(.context == $c)
       | select((.description // "") | contains($re))] | length')"

  if [ "$awaiting" -ne 0 ]; then
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

  cat >&2 <<EOF

check-pr-ci-ran: NO PipelineRun ever reported on ${SHA}.

The only status is Pipelines-as-Code's own gating context. PaC processed the
event and created nothing — typically a .tekton manifest that failed to parse,
which it reports at the top level and then stops.

The PR is unverified. Fix the manifest and push, or retest: note that a retest
does NOT clear the top-level failure above, only a new head SHA does.
EOF
  exit 1
fi

failed="$(printf '%s' "$status_json" |
  jq --arg c "$PAC_CONTEXT" '[.statuses[]
     | select(.context != $c)
     | select(.status != "success")] | length')"

if [ "$failed" -ne 0 ]; then
  echo >&2
  echo "check-pr-ci-ran: ${failed} pipeline context(s) not successful on ${SHA}." >&2
  exit 1
fi

# Every real run passed. If PaC's gating context is nevertheless red, say so
# loudly rather than swallowing it: it is usually stale, but it can also mean a
# manifest never produced a run at all, and nothing here can tell the two apart.
gating_failed="$(printf '%s' "$status_json" |
  jq --arg c "$PAC_CONTEXT" '[.statuses[]
     | select(.context == $c)
     | select(.status == "failure" or .status == "error")] | length')"

if [ "$gating_failed" -ne 0 ]; then
  stale="$(printf '%s' "$status_json" | jq --arg c "$PAC_CONTEXT" -r '
    ([.statuses[] | select(.context == $c and (.status == "failure" or .status == "error")) | .created_at] | max) as $gating
    | ([.statuses[] | select(.context != $c) | .created_at] | max) as $newest_run
    | if $gating == null or $newest_run == null then "unknown"
      elif $gating < $newest_run then "yes"
      else "no"
      end')"

  echo >&2
  echo "check-pr-ci-ran: WARNING — '${PAC_CONTEXT}' is red on ${SHA}, but no" >&2
  echo "PipelineRun owns it." >&2
  if [ "$stale" = "yes" ]; then
    echo "It predates the newest run context, so it is most likely stale from an" >&2
    echo "earlier attempt; only a new head SHA clears it, and the forge will keep" >&2
    echo "reporting this commit as failed until then." >&2
  else
    echo "It is NOT older than the newest run context, so treat it as live." >&2
  fi
  echo "It can also mean a .tekton manifest failed to parse: the runs below are" >&2
  echo "green, which is not proof that every intended pipeline ran." >&2
fi

echo "check-pr-ci-ran: OK — ${runs} pipeline context(s) succeeded on ${SHA}."
exit 0
