---
name: work-on
description: Start focused work on a bead — rename the tab, blue accent, worktree, and load the issue
argument-hint: <bead-id>
license: GPL-3.0-or-later
compatibility: Requires the `bd`, `wt` and `herdr` CLIs, plus the `wt-switch-create` skill
---

## Model

Run this skill's steps on Sonnet. Skills can't pin a model via frontmatter
(that's a Command-only field) — this is a best-effort instruction to the
invoking agent, not an enforced constraint.

## Argument

Bead id: `$ARGUMENTS`

## Step 0 — no argument means stop

If `$ARGUMENTS` is empty, print exactly the help text below and **do nothing
else**. No tools, no worktree, no renames.

```
/work-on <bead-id>

Sets up a focused work session for one bead:
  1. bd show <bead-id>              — validate the id and load it into context
  2. herdr tab rename               — tab label becomes the bead id
  3. herdr accent = blue            — GLOBAL herdr accent, affects every tab
  4. /wt-switch-create <bead-id>    — worktree + branch named after the bead,
                                      session cwd moves into it

Example: /work-on myrepo-5jwr
Find work with: bd ready
```

## Step 1 — load the bead (this is also the validity check)

```bash
bd show $ARGUMENTS
```

If the bead does not exist, stop here and report it — do not create a worktree
or rename anything for a bogus id.

## Step 2 — rename the herdr tab to the bead id

```bash
herdr tab rename "$(herdr api snapshot | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["snapshot"]["focused_tab_id"])')" "$ARGUMENTS"
```

If herdr is not running, the socket call fails — say so and carry on with the
remaining steps.

## Step 3 — set the herdr accent to blue

`accent` lives under `[ui]` in `~/.config/herdr/config.toml`. There is no CLI
setter, so Read the file and Edit it:

- if an `accent = ...` line exists under `[ui]`, change its value to `"blue"`
- otherwise insert `accent = "blue"` as a new line inside the `[ui]` table

Then apply it:

```bash
herdr config check && herdr server reload-config
```

⚠️ herdr has no per-tab or per-session colour. This accent is **instance-wide** —
every tab in every workspace turns blue and stays blue until changed back.
`/done-with` sets it back to `green` ("free") when the bead is closed out —
blue vs. green is the busy/free signal across the whole herdr instance.

## Step 4 — create the worktree and enter it

Invoke the `wt-switch-create` skill with the bead id as the branch name:

```
Skill(skill: "wt-switch-create", args: "$ARGUMENTS")
```

Follow that skill exactly — it owns worktree creation, the
already-exists/denied/unreachable recovery paths, and the cleanup contract. Do
not hand-roll `wt switch` or `git worktree add` here.

No task text is passed after `--`: the bead body from Step 1 is the task, and
the user drives from there.

## Step 5 — report

One line: bead id, title, worktree path, branch. Then state what the bead asks
for and wait for the user.
