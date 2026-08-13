#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# openspec-validate.sh — run the same openspec gate that CI runs, locally.
#
# Mirrors the `validate` step of .tekton/tasks/openspec-validate.yaml so a
# malformed change delta fails at commit time instead of in the pipeline.
# This deliberately shells out to `openspec` rather than re-implementing any
# of its rules: one source of truth, and every rule is covered, not just the
# one that bit us last.
#
# On failure it appends a hint about the most common — and most confusing —
# violation: openspec only inspects the FIRST physical line of a requirement
# paragraph when looking for SHALL/MUST, so a hard-wrapped statement whose
# first line happens to end before the keyword is rejected even though the
# paragraph is full of SHALLs.
#
# Skips with a warning when openspec is not installed, so contributors
# without the tool are not blocked; CI runs it in an image that has it.
#
# POSIX sh — no bashisms — so it runs identically in local shells, the
# pre-commit hook, and the minimal release-tools CI image.

set -eu

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

if ! command -v openspec >/dev/null 2>&1; then
  echo "openspec-validate: openspec not found on PATH — skipping." >&2
  echo "openspec-validate: install it to catch spec errors before CI does." >&2
  exit 0
fi

if openspec validate --all --strict --no-interactive; then
  exit 0
fi

cat >&2 <<'EOF'

openspec-validate: validation failed.

Hint — if the error reads:

    ADDED "<name>" must contain SHALL or MUST

...check whether that requirement's statement is hard-wrapped. openspec only
looks at the first physical line of the paragraph, so this fails:

    ### Requirement: Attachment upload source

    The `create_issue_attachment`, `create_comment_attachment`, and
    `create_release_attachment` tools SHALL accept ...

Put the whole statement on one unwrapped line instead. See DEVELOPER.md,
"OpenSpec conventions".
EOF

exit 1
