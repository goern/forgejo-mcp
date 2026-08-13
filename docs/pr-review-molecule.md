<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# PR review molecule — operational manual

A repeatable, dependency-ordered checklist for reviewing an external
contributor PR. One command turns the template into eleven beads; you then work
them like any other beads.

- Template: `.beads/plans/pr-review-molecule.json`
- Instantiator: `scripts/bd/new-pr-review-molecule.sh`

## 1. Pour the molecule

Look before you leap — `--dry-run` touches nothing:

```bash
scripts/bd/new-pr-review-molecule.sh 482 --dry-run   # preview
scripts/bd/new-pr-review-molecule.sh 482             # create 11 beads
```

**From a plain shell:** run it from anywhere inside the checkout. The script
resolves the repo root itself.

**From inside Claude Code:** two ways, and the difference matters.

| You want | Do this |
| --- | --- |
| To run it yourself, output lands in the session | Type `! scripts/bd/new-pr-review-molecule.sh 482` |
| Claude to run it and carry on with the review | Ask: *"pour the PR review molecule for 482"* |

The `!` prefix executes in your session, so both you and Claude see the result.
Prefer it when you want to stay in control of what gets created.

## 2. See what you got

```bash
bd show <epic-id>            # the epic
bd children <epic-id>        # the eleven steps
bd graph <epic-id>           # dependency DAG — what unblocks what
```

The chain is: triage → {assign, reproduce}; reproduce → diagnose →
{comment_fix, harden}; comment_fix → demo → state; harden → commit; everything
→ closeout. `followups` is independent and can run any time.

`assign` runs in parallel with the technical work on purpose: ownership and
review requests should land within minutes of triage, not after the
investigation. It is also where you confirm CI actually ran — for an author who
is not in `OWNERS`, Pipelines-as-Code may never have started, and "no red
checks" then means "no checks".

## 3. Work the beads

`bd ready --mol` is the whole loop. It only ever shows steps whose blockers are
closed, so you cannot accidentally comment before you have diagnosed:

```bash
bd ready --mol <epic-id>              # what can I do right now?
bd update <step-id> --claim           # take it (assignee + in_progress, atomic)
bd show <step-id>                     # the step's description IS the instructions
# ... do the work ...
bd close <step-id>
bd ready --mol <epic-id>              # next
```

Each bead's description names the skill, agent, or `castra` command that does
that step. Read it — it is not a title-only checklist.

**Driving it from Claude Code:** ask *"work the next ready step of the PR
review molecule"*, or use the `work-on` skill against a specific step id. Claude
claims, executes what the description specifies, and closes it. Ask for one
step at a time if you want a checkpoint between them.

Notes as you go, so the next session inherits your reasoning:

```bash
bd note <step-id> "pipelinerun logs already pruned; recovered cmd from taskSpec"
```

## 4. Close out

The `closeout` step spells this out, but it is the part people skip:

```bash
git pull --rebase
bd dolt push        # beads are NOT in git — without this they stay on your machine
git push
git status          # must show up to date
```

Also: release the `castra:bot-active` mutex the triage step acquired, and
remove the throwaway worktree (`git worktree list`).

The `done-with` skill runs this protocol for you.

## 5. Improve the template

The molecule is a file, not a ritual. When a review teaches you something the
template did not know, edit `.beads/plans/pr-review-molecule.json` and commit
it — the next PR inherits the lesson. Editing a poured bead only helps that one
PR.

Validate after editing:

```bash
python3 -c "import json;json.load(open('.beads/plans/pr-review-molecule.json'))"
scripts/bd/new-pr-review-molecule.sh 999 --dry-run
```
