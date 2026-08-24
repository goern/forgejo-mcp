---
name: work-on
description: Start focused work on a bead — rename the tab and the Claude session, create the worktree, and load the issue
argument-hint: <bead-id>
license: GPL-3.0-or-later
compatibility: Requires the `bd`, `wt` and `herdr` CLIs, plus the `wt-switch-create` skill; the session rename additionally needs the `UserPromptSubmit` hook from the README
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
  2. herdr tab rename               — THIS session's tab label becomes the bead id
  2b. session title queued          — the Claude session takes the same name
  3. /wt-switch-create <bead-id>    — worktree + branch named after the bead,
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
scripts/herdr-rename-tab.sh "$ARGUMENTS"
```

(Run it from the skill's own directory, or by absolute path — it lives next to
this file.)

The script renames the tab hosting **this** session, resolved from the
`HERDR_PANE_ID` that herdr exports into every pane it spawns. Never resolve the
tab from `herdr api snapshot`'s `focused_tab_id`: that is whichever tab is in
front at the moment the command lands, so switching tabs while the skill runs
renames a bystander.

Best effort — every failure is non-fatal, report it and carry on with the
remaining steps:

| exit | meaning |
|------|---------|
| 0 | renamed |
| 3 | `HERDR_PANE_ID` unset — session is not inside a herdr pane, nothing to rename |
| 4 | pane unknown to the server (herdr not running, or the pane is gone) |
| 5 | herdr rejected the rename |

## Step 2b — give the Claude session the same name

```bash
scripts/session-title.sh queue "$ARGUMENTS"
```

The tab label and the session name should never disagree, so this runs right
after Step 2 with the same argument.

A session cannot rename itself mid-turn — Claude Code accepts `sessionTitle`
only from the `UserPromptSubmit` and `SessionStart` hooks, and `/rename` is a
human-only builtin. The script therefore *queues* the label; the
`UserPromptSubmit` hook (see the README) applies it when the user sends their
next message. Tell the user that in the Step 4 report: the tab flips now, the
session name one message later.

Best effort, same as the tab rename:

| exit | meaning |
|------|---------|
| 0 | queued |
| 3 | `CLAUDE_CODE_SESSION_ID` unset — not a Claude Code session, nothing to queue |

If the hook is not installed the label is queued and never consumed, which is
harmless: the file is dropped after seven days. Do not try to work around a
missing hook by editing `~/.claude/sessions/*.json` — that registry is owned by
the running session and your write will be overwritten.

## Step 3 — create the worktree and enter it

Invoke the `wt-switch-create` skill with the bead id as the branch name:

```
Skill(skill: "wt-switch-create", args: "$ARGUMENTS")
```

Follow that skill exactly — it owns worktree creation, the
already-exists/denied/unreachable recovery paths, and the cleanup contract. Do
not hand-roll `wt switch` or `git worktree add` here.

No task text is passed after `--`: the bead body from Step 1 is the task, and
the user drives from there.

## Step 4 — report

One line: bead id, title, worktree path, branch, plus the tab/session rename
outcome (and, when the session name was only queued, that it lands on the next
message). Then state what the bead asks for and wait for the user.
