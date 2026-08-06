---
name: done-with
description: Close out a bead started with /work-on — verify it's safe, clean up the worktree, free the tab
argument-hint: "[<bead-id>]"
license: GPL-3.0-or-later
compatibility: Requires the `wt`, `jq` and `herdr` CLIs
---

Counterpart to `/work-on`. Undoes every step it took: removes the worktree
(when safe), frees the herdr tab, resets the accent.

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
  uncommitted changes and do not touch the worktree, tab, or accent. The user
  commits or stashes first.
- **Merged** — `main_state` is `"integrated"` or `"empty"` → safe, proceed.
- **PR/MR open** — `ci.number` is present (any `ci.status`, draft included —
  "opened" is the bar here, not "approved") → safe, proceed.
- **Neither** — not merged and no PR → **stop**. Report the branch's ahead/behind
  counts and that no PR was found; tell the user to push and open a PR, or
  merge, before cleaning up. Do not touch the worktree, tab, or accent.

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
herdr tab rename "$(herdr api snapshot | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["snapshot"]["focused_tab_id"])')" "free"
```

If herdr is not running, say so and carry on with the remaining step.

## Step 4 — reset the herdr accent to green

Read `~/.config/herdr/config.toml`, ensure `accent = "green"` under `[ui]`
(insert if absent), then:

```bash
herdr config check && herdr server reload-config
```

`/work-on` sets this same accent to **blue** when a bead *starts* — blue vs.
green is the busy/free signal, instance-wide across every herdr tab and
workspace.

## Step 5 — report

One line: branch, why it was safe to remove (merged / PR #N), worktree removed
y/n, branch kept/deleted, tab renamed to "free". Mention `bd close <bead-id>`
if the bead itself is still open — this command does not close beads.
