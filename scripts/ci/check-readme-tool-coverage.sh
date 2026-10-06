#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-readme-tool-coverage.sh — every registered tool and resource template
# must have a row in the README.
#
# AGENTS.md asks for a README table row whenever a tool is added, and nothing
# enforced it: 22 of 155 tools and 4 of 17 resource templates had drifted out
# of the tables by #601, one of them (delete_org) destructive. Credit without
# documentation is worse than neither — the contributors table had been
# thanking someone for list_repo_contents and get_repo_tree since PR #116/#117
# while the tools themselves were never listed.
#
# What is compared:
#
#   tools      — every `<Name>ToolName = "<tool_name>"` constant under
#                operation/ (excluding _test.go) against every `| `<name>` `
#                row in README.md.
#   resources  — every registered `forgejo://…{…}` URI template (those with a
#                `{` placeholder, so concrete test URIs are ignored) against
#                the README resource table.
#
# The check is one-directional on purpose: a README row with no matching
# constant is NOT an error. Some tools are registered without a ToolName
# constant, and flagging those would make the check lie. Missing documentation
# is the failure mode worth catching.
#
# Exit 0 when every registered name is documented. Exit 1 listing what is not.
#
# POSIX sh — no bashisms — so it runs identically in local shells, the
# pre-commit hook, and the minimal CI image.

set -eu

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# --- tools -----------------------------------------------------------------
# The constant form is the contract: operation/**/*.go declares
#   SomeToolName = "some_tool"
grep -rhE '^[[:space:]]*[A-Za-z]+ToolName[[:space:]]*=[[:space:]]*"[a-z_]+"' \
  operation/ --include='*.go' 2>/dev/null \
  | grep -v '_test\.go' \
  | sed -n 's/.*"\([a-z_]*\)".*/\1/p' \
  | sort -u >"$tmp/tools_code"

# A documented tool is a table row whose first cell is a backticked name.
sed -n 's/^| `\([a-z_][a-z_]*\)`.*/\1/p' README.md | sort -u >"$tmp/tools_readme"

comm -23 "$tmp/tools_code" "$tmp/tools_readme" >"$tmp/tools_missing"

# --- resource templates ----------------------------------------------------
# Only templates: a URI carrying a `{` placeholder. Concrete forgejo:// URIs in
# tests and docs are not registrations.
grep -rhoE '"forgejo://[^"]*\{[^"]*"' operation/ --include='*.go' 2>/dev/null \
  | grep -v '_test\.go' \
  | tr -d '"' \
  | sort -u >"$tmp/res_code"

sed -n 's/^| `\(forgejo:\/\/[^`]*\)`.*/\1/p' README.md | sort -u >"$tmp/res_readme"

comm -23 "$tmp/res_code" "$tmp/res_readme" >"$tmp/res_missing"

# --- report ----------------------------------------------------------------
tools_total="$(wc -l <"$tmp/tools_code" | tr -d ' ')"
res_total="$(wc -l <"$tmp/res_code" | tr -d ' ')"
tools_bad="$(wc -l <"$tmp/tools_missing" | tr -d ' ')"
res_bad="$(wc -l <"$tmp/res_missing" | tr -d ' ')"

if [ "$tools_bad" -gt 0 ]; then
  echo "Registered tools with no README table row:" >&2
  sed 's/^/  - /' "$tmp/tools_missing" >&2
fi

if [ "$res_bad" -gt 0 ]; then
  echo "Registered resource templates with no README table row:" >&2
  sed 's/^/  - /' "$tmp/res_missing" >&2
fi

errors=$((tools_bad + res_bad))

echo "check-readme-coverage: ${tools_total} tool(s), ${res_total} resource template(s) checked, ${errors} undocumented."

if [ "$errors" -gt 0 ]; then
  echo "" >&2
  echo "Add a row to the matching table in README.md. See AGENTS.md, 'Adding a New Tool'." >&2
  exit 1
fi
exit 0
