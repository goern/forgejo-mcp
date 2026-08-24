---
name: done-with
description: Close out a bead started with /work-on — verify it's safe, clean up the worktree, free the tab and the session name
argument-hint: "[<bead-id>]"
license: GPL-3.0-or-later
compatibility: Requires the `wt`, `jq` and `herdr` CLIs; the session rename additionally needs the `UserPromptSubmit` hook from the README
---

Counterpart to `/work-on`. Undoes every step it took: removes the worktree
(when safe) and frees the herdr tab and the Claude session name.

## Model

Run this skill's steps on Sonnet. Skills can't pin a model via frontmatter
(that's a Command-only field) — this is a best-effort instruction to the
invoking agent, not an enforced constraint.

## Step 0 — resolve the branch/bead

`$ARGUMENTS` if given; otherwise the current branch:

```bash
git rev-parse --abbrev-ref HEAD
```

If that resolves to the repo's default branch, there is nothing to clean up —
stop and say so.

## Step 1 — check it's safe to remove: merged, or a PR is open

```bash
wt list --format=json --full | jq -e --arg b "$BRANCH" '.[] | select(.branch == $b)'
```

Read the matched entry:

- **Dirty working tree** — any of `working_tree.staged` / `.modified` /
  `.untracked` / `.renamed` / `.deleted` is true → **stop**. Report the
  uncommitted changes and do not touch the worktree or the tab. The user
  commits or stashes first.
- **Merged** — `main_state` is `"integrated"` or `"empty"` → safe, proceed.
- **PR/MR open** — `ci.number` is present (any `ci.status`, draft included —
  "opened" is the bar here, not "approved") → safe, proceed.
- **Neither** — not merged and no PR → **stop**. Report the branch's ahead/behind
  counts and that no PR was found; tell the user to push and open a PR, or
  merge, before cleaning up. Do not touch the worktree or the tab.

If the branch has no worktree entry at all in the JSON, say so and stop —
nothing to clean up.

## Step 2 — remove the worktree (only after Step 1 passed)

Try `ExitWorktree({action: "remove"})` first — this is the common case: the
session that's closing out the bead is the same one `/work-on` created the
worktree in. It fires the `WorktreeRemove` hook wired into the agent's settings
(`.claude/settings.json` in Claude Code), which runs
`wt remove --foreground <path>` — that keeps the branch unless it's fully
merged (`main_state` `integrated`/`empty`), so a branch backing a still-open PR
survives even though its worktree is gone.

If `ExitWorktree` reports no active worktree session (this session didn't
create it, or it's a fresh session resuming cleanup) — fall back to:

```bash
wt remove "$BRANCH" --foreground
```

## Step 3 — rename the herdr tab to "free"

```bash
scripts/herdr-rename-tab.sh free
```

(Run it from the skill's own directory, or by absolute path — it lives next to
this file.)

Same script `/work-on` uses: it renames the tab hosting **this** session,
resolved from the `HERDR_PANE_ID` herdr exports into every pane. Never resolve
the tab from `herdr api snapshot`'s `focused_tab_id` — that renames whichever
tab is focused when the command lands, not the one running the session.

Best effort — every failure is non-fatal, report it and carry on:

| exit | meaning |
|------|---------|
| 0 | renamed |
| 3 | `HERDR_PANE_ID` unset — session is not inside a herdr pane, nothing to rename |
| 4 | pane unknown to the server (herdr not running, or the pane is gone) |
| 5 | herdr rejected the rename |

## Step 3b — free the Claude session name too

```bash
scripts/session-title.sh queue free
```

Same label as the tab, for the same reason `/work-on` sets both: a session
still called `<bead-id>` after cleanup is a lie. The rename is queued and
applied by the `UserPromptSubmit` hook on the user's next message — a session
cannot rename itself mid-turn. Exit 3 means `CLAUDE_CODE_SESSION_ID` is unset
(not a Claude Code session); non-fatal, report and carry on.

Both renames only ever run after Step 1 passed. A refused cleanup leaves the
tab *and* the session name on the bead id, which is exactly the truthful
signal.

## Step 4 — report

One line: branch, why it was safe to remove (merged / PR #N), worktree removed
y/n, branch kept/deleted, tab renamed to "free" and the session rename queued
(it lands on the next message). Mention `bd close <bead-id>`
if the bead itself is still open — this command does not close beads.
