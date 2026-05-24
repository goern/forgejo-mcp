---
name: "Team: Battle Test"
description: Spawn a proposal battle-test team for an OpenSpec change's proposal.md and design.md before specs/tasks harden
category: Agent Teams
tags: [agent-teams, openspec, review, architecture]
---

Battle-test an OpenSpec change before specs/tasks harden.

**Input** (`$ARGUMENTS`): one of

- A change name (e.g. `spatial-niche-attachments`)
- Empty — ask which OpenSpec change

This command reviews **both**:

- `openspec/changes/<change>/proposal.md`
- `openspec/changes/<change>/design.md`

The output is a change-local review artifact:

- `openspec/changes/<change>/battle-test.md`

## Pre-flight

1. **Verify agent teams are enabled.** `.claude/settings.json` must contain
   `"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"` under `env`. If not, refuse and tell the user to add it and restart Claude Code.

2. **Resolve the change.**
   - If `$ARGUMENTS` includes a change name, use it.
   - If empty, run `openspec list --json` and ask the user which change.
   - Refuse if `openspec/changes/<change>/` does not exist.

3. **Verify target artifacts exist.**
   - Require `proposal.md`.
   - Require `design.md`.
   - If either is missing, refuse and tell the user to run `/opsx:continue` until both artifacts exist.

4. **Verify the change is pre-apply.** Run `openspec status --change <change>
   --json`. If tasks are complete and implementation has started, warn that this command is proposal/design review, not implementation verification.  Continue only if the user explicitly wants late review.

5. **Read the targets end-to-end before spawning teammates.** The lead must
   read:
   - `proposal.md`
   - `design.md`
   - any existing `specs/**/*.md` if present
   - `openspec/specs/CLAUDE.md`
   - every canonical touchpoint listed in `proposal.md`

6. **Write OpenSpec journal entries when a journal exists.**
   - Before the team run:
     `python3 scripts/openspec-journal.py <change> turn.start input="Battle-test proposal and design"`
   - Record team spawn:
     `python3 scripts/openspec-journal.py <change> agent.spawned count=<n> kind="battle-test"`
   - After writing `battle-test.md`:
     `python3 scripts/openspec-journal.py <change> artifact.added ref="battle-test.md"`
   - End:
     `python3 scripts/openspec-journal.py <change> turn.end output="Battle-test complete; see battle-test.md"`

## Team Spawn

Create an agent team with these teammates:

| Name | Subagent type | Model | Role |
|------|---------------|-------|------|
| `adversary` | `devils-advocate` | opus | Break the proposal/design with load-bearing critiques only. |
| `defender` | `proponent` | opus | Defend, concede-patch, concede-future, or stalemate each critique. |

Spawn prompts:

- **adversary**:
  "Adversarially review OpenSpec change `<change>`, focusing on
  `proposal.md` and `design.md`. Read the canonical touchpoints listed in
  `proposal.md`, plus any existing change specs if present. Produce up to 10
  load-bearing critiques only. Each critique must name failure mode, affected
  artifact section, canonical conflict if any, and the cheapest falsifier.
  Post critiques to the team mailbox addressed to `defender`. Do NOT edit
  files."

- **defender**:
  "Defend OpenSpec change `<change>` against critiques posted by `adversary`.
  Read `proposal.md`, `design.md`, canonical touchpoints, and any existing
  change specs. For each critique use exactly one verdict: defend,
  concede-patch, concede-future, stalemate. For concede-patch, name the
  smallest artifact edit. Post responses to the team mailbox addressed to
  `lead`. Do NOT edit files."

## Mediation Loop

Lead's job:

1. Wait for `adversary` critiques.
2. Ensure `defender` responds to every critique.
3. Allow one rebuttal round only for defended critiques.
4. Synthesize `openspec/changes/<change>/battle-test.md`.
5. Apply only trivial typo/clarity patches immediately. For structural edits, list exact recommended patches in `battle-test.md` and ask the user before mutating proposal/design/spec artifacts.

## Battle-Test Artifact Shape

Write:

```markdown
# Battle Test — <change>

**Date:** <YYYY-MM-DD>
**Targets:** proposal.md, design.md
**Team:** adversary, defender

## Verdict

Proceed | Patch first | Rework

<one paragraph>

## Surviving Critiques

### <title>
**Severity:** blocker | major | watch
**Failure mode:** ...
**Hits:** proposal.md#... / design.md#... / canonical doc...
**Falsifier:** ...
**Disposition:** defended | patch | future | stalemate

## Conceded Patches

- <artifact>: <exact edit needed>

## Future Work

- <item>

## Lead Recommendation

<what to do before /opsx:continue or /opsx:apply>
```

## Hard Rules

- **No teammate edits files.** Lead writes only `battle-test.md` unless the user approves structural artifact patches.
- **No Bash for thought teammates.** Use Bash only in the lead pre-flight and journal/status steps.
- **Maximum one rebuttal round.** This is a hardening pass, not an endless debate.
- **Concede early.** If critique identifies a missing falsifier, missing observability surface, hidden migration cost, or canonical contradiction, record it.
- **English only.** Repo rule.
- **Do not run implementation checks.** This command reviews proposal/design. Use verifier/apply workflows for implementation evidence.

## Cleanup

After writing `battle-test.md`, ask the user whether to clean up the team.
Do not archive temp findings until the user has read the synthesis.

## Argument: $ARGUMENTS
