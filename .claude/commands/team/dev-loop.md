---
name: "Team: Dev Loop"
description: Spawn an implementer + verifier (+ optional planner) team that iterates on a scoped change until a deterministic check passes. Lead stays light.
category: Agent Teams
tags: [agent-teams, implementation, build-loop]
---

Implementation team with a deterministic verifier in the loop. Use when you
have a scoped change that can be proven done by *running a tool* (tests,
linter, typecheck, schema check, build). Not for review or design work — see
`team:debate` and `team:battle-test` for those.

**Input** (`$ARGUMENTS`): a free-form scope description. The lead reads it,
inspects the relevant files, and decides whether a planner round is needed.

## When this fits

- Refactor with tests as the contract.
- New feature where the slice is small and the check is automated.
- Migration where the verifier is "old and new produce the same output".
- Anything where success collapses to a single shell command exiting 0.

If the verifier is "looks good to me" — wrong skill.

The team can also start *before* the slice is fully defined. Spawn `planner`
first, ask "what's the next sensible slice here?", then derive impl tasks
from the planner's answer. impl and runner sitting idle while planner reads
the codebase is fine — they cost ~nothing while idle.

## Lead does not gate-keep teammates

When the user gives a clear instruction for a specific teammate — "send
this command to the planner", "have impl do X", "ask runner to re-run" —
forward it. Do not second-guess whether the activity "fits the loop
shape". The teammates have their own judgment. If the planner is asked to
do orientation work or run a catchup command, that *is* planner work, not
a violation of the skill.

Refuse only when the instruction would break a hard rule (lead writes
code, runner edits files, scope wall crossed, etc.). Everything else
routes through.

## Pre-flight (lead)

1. Confirm `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1` is set. If not, refuse.
2. Read the files in scope end-to-end. The lead must understand the domain
   well enough to mediate when something goes off the rails.
3. Identify the verifier command. It must exit 0 on success, non-0 on
   failure. Smoke-test it once before spawning the team.
4. Decide: is the slice clear enough to brief `impl` directly, or do you
   want a `planner` round first? Only spawn the planner when *you* can't
   confidently shape the work yourself — opus tokens are the expensive ones.

## Roles

| Name | `subagent_type` | `model` | Tools | Job |
|------|-----------------|---------|-------|-----|
| `planner` (optional) | `Plan` | opus | Read, Grep, Glob, WebFetch | Single-shot: propose the slice, name what to test/build, hand back. |
| `impl` | `implementer` | sonnet | Read, Edit, Write, Bash | Claim impl tasks in order, write code, ping `runner` when ready. |
| `runner` | `verifier` | haiku | Read, Bash | Run the verifier command, report PASS or FAIL excerpt. Never edits. |

Spawn via the `Agent` tool with `team_name`, `name`, `subagent_type`, and
`model`. The subagent types above are pre-installed; check the Agent tool's
own list before assuming a type exists.

## Mechanics — what does the work

Use the team task list (`TaskCreate` / `TaskUpdate` / `TaskList`), not your
prose, to choreograph. The point is that `addBlockedBy` makes the runner
self-trigger when impl finishes, with no polling from the lead.

1. `TeamCreate` with a short `team_name`.
2. Create one task per logical impl step. Last task: "Run verifier and
   report". Use `addBlockedBy` to chain.
3. Spawn `impl` and `runner` (and `planner` if needed). Each spawn prompt
   should reference: their task IDs, the communication protocol below, the
   file boundaries, and "do not touch X".
4. After spawn, the lead **stops** until a teammate sends a real message.
   Idle notifications are not events to react to.

## Communication protocol (bake into spawn prompts)

- impl finishes its tasks, then `SendMessage` runner: `"Phase N ready. Run task #X."`
- runner runs the verifier:
  - PASS → `SendMessage impl "PASS — <one line>. Output tail: ..."`, mark
    runner's task completed, go idle.
  - FAIL → `SendMessage impl` with exit code, failing test names, last ~40
    lines. Keep status `in_progress`. Go idle.
- impl on FAIL: fix, then `SendMessage runner "Fixed: <one-liner>. Re-run task #X."`
- Loop until PASS.

The lead is not in this loop. If the loop stops moving (both idle, no
PASS), then the lead investigates.

### Mandatory checkpoint rule for impl (bake into impl spawn prompt)

Long phases hit per-turn budgets. When that happens, impl finishes its
turn gracefully — which from the harness's view is "idle" — but it has
not handed off to runner. The watchdog can't tell that apart from a dead
agent. Defend against this in the impl prompt:

> After every 2 sub-tasks completed: `SendMessage` lead with one line
> "P<n> progress: <ids> done, on <next>". Never end a turn idle without
> first sending a message. If you have nothing else to say, send "Still
> working on X.Y, will checkpoint after next sub-task." Silence is the
> only failure signal the lead has — silent ≈ replaced.

Also bake routing into the impl prompt explicitly:

> When you write "Run task #N" or "Phase N ready", the recipient is
> `runner`, NOT `team-lead`. Lead is not the verifier — runner is.

Without these two clauses, expect repeat failures: silent stalls and
work-product messages mis-routed to lead.

## Worker discipline (bake into spawn prompts)

Three rules every spawn prompt must include verbatim. Workers ignore vague
guidance, so the exact wording matters.

**1. Canonical task IDs only.** Never call `TaskCreate`. The lead creates
tasks once and hands you the IDs in the spawn brief. You only call
`TaskUpdate(taskId=<id>, status=in_progress|completed)`. Creating a new
task with the same name as an existing one breaks the dependency chain
(the verifier `blockedBy` the original ID never unblocks) and forces lead
cleanup. Past-run symptom: impl creates `#21` "P2.1" while `#14` "P2.1" is
already assigned to it; `#7 verifier blockedBy [1-6]` then sits forever
because impl ticked `#21` not `#14`.

**2. Stale inbox rule.** When you wake, your inbox may contain old
messages from before the team's current state. Read once, identify the
MOST RECENT message from team-lead, act ONLY on that. Discard older lead
instructions, idle notifications, peer-DM summaries. Replying to an
obsolete FAIL or "begin Phase N" after the team has already moved on
burns a turn, sends contradictory state into the loop, and triggers
unnecessary lead intervention. Symptom: impl reporting "fixed!" for a
problem the team solved 10 minutes ago.

**3. (Runner only) Exact trigger phrase.** You run the verifier ONLY when
the active impl sends a message whose body contains the literal phrase
`"Phase N ready. Run task #<id>"`. None of the following are triggers:
idle notifications, peer-DM summaries that mention "ready" or "PASS",
lead messages that route or inform, your own task becoming
`blockedBy:[]`. The `blockedBy` field is the contract: if your task is
blocked, do not run, period. Premature runs produce vacuous-PASSes the
lead must revert; in past sessions, three premature runs = replacement.

## Verifier filter pitfall

`vitest -t <word>` matches by the test-suite name impl chose, not by the
spec vocabulary the lead picked. Past run: lead picked `-t fingerprint`
from tasks.md text; impl named the suite "fetchAndFingerprint" — the
filter matched zero tests but vitest exit-coded 0 (no failures). Lead
treated it as PASS; impl's actual unit tests had never run.

Mitigations, in order of preference:

- **File-path filter** when the new test file path is known: `vitest run
  packages/foo/__tests__/new_file.test.ts`. Most precise.
- **Broad `-t` alternation** when test names span variants: `-t
  "fingerprint|drift|fetch"`. Cast the net wide.
- **Always pair with the vacuous-pass guard** in the runner prompt:

  > If vitest reports `Tests 0 passed` or `no test files matched the
  > filter`, that is a FAIL, not a pass. The phase introduces new test
  > files; if vitest finds zero matches, the test files were not created
  > or the filter is wrong. Report FAIL with "Verifier matched zero
  > tests — test files missing or filter mismatched."

The vacuous-pass guard is non-negotiable for any phase that adds new
test files. Without it, the verifier loop has no signal that the new
tests exist at all.

## Long-running runs: heartbeat + watchdog

For runs that span more than a few phases (overnight, large OpenSpec
changes), the lead needs an external heartbeat. The dev-loop has no
built-in keepalive — if a teammate goes silent mid-phase and the user
isn't watching, nothing kicks the loop forward.

Pattern that works: `ScheduleWakeup` at 270s with a watchdog prompt that:

1. `TaskList` — find lowest-id `in_progress` task. Note id + owner.
2. Compare to last tick (read your own previous user-facing line:
   `Watchdog: stuck on task #N owner=<name>`). Owner change resets the
   stuck count.
3. If team advanced (different task id or owner, or all completed):
   re-arm and emit one-line status.
4. If same task id AND same owner two ticks running:
   - **Tick 1 (1st probe)**: `SendMessage` owner asking for one of
     `working on X.Y` / `blocked: Z` / `done, pinging runner`. Re-arm.
   - **Tick 2 (still silent)**: shutdown owner, `TaskUpdate` task back
     to `pending` with new owner name (`impl<N+1>`), spawn replacement,
     `SendMessage` runner the new active impl name, re-arm.
5. Owner=runner stuck: 3-tick threshold (verifiers can be slow).
6. All tasks completed: do NOT re-arm. Final summary, suggest next.

Why 270s and not 300s: prompt cache TTL is 5 min. Picking 300 burns the
cache miss without amortizing it; 270 keeps cache warm tick-to-tick.

The freshness signal is `TaskList` (status transitions are the only
state every teammate is *required* to update). Do not lean on the
journal — teammates may forget to journal but cannot make the chain
advance without `TaskUpdate`.

False positives are real: a single 4-min silence can be a legitimate
long edit. Two consecutive 4-min silences (~9 min total) on the same
task is a real stall. The two-tick rule trades latency for accuracy.

## Replacing a stuck teammate

When watchdog rule 4 fires, or the user manually orders a replacement:

1. `SendMessage` old teammate `{type:"shutdown_request"}`. **Do not wait**
   for the approval. The shutdown handshake is async and can take many
   minutes — sometimes longer than the rest of the run.
2. `TaskUpdate` the in-progress task: `status=pending`, `owner=impl<N+1>`
   (next free number — check task history; impl, impl2, impl3, ...).
3. `Agent` spawn with `subagent_type=implementer`, `model=sonnet`,
   `name=impl<N+1>`, `team_name` of the live team. The replacement brief
   must say:
   - "You replace impl<N>. Read tasks.md and `git diff` to see partial
     work. Continue task #<id>."
   - The full checkpoint rule and routing rule from the impl prompt.
   - The three worker-discipline rules (canonical IDs, stale-inbox,
     trigger-phrase) verbatim — these are the rules predecessors broke.
   - "Predecessors were replaced for going silent. Your survival depends
     on regular `SendMessage` checkpoints."
4. `SendMessage` new teammate: `"Begin. Task #<id> is yours."` (the
   spawn does not auto-trigger a turn; you have to nudge.)
5. `SendMessage runner`: `"Active impl is now impl<N+1>."`

Async-shutdown caveat: the old teammate may wake up *after* the
replacement has done useful work and try to act on stale messages. Two
mitigations:

- The replacement is `impl<N+1>` with a different name, so messages addressed
  by name don't collide.
- If the old teammate's eventual reply arrives and contradicts the new
  state ("task #X done!" when the replacement just re-did it), prefer
  the replacement's work — it's the live one. The old teammate's claim
  may be a stale message from before its silence began.

Be aware of false-positive replacements: if the old teammate was actually
alive and slow (a single long-running edit can stretch past 9 minutes on
a docs-heavy phase), you'll have spawned a redundant replacement. When
the original surfaces with a "done" message, shutdown the redundant one
and keep the original active. This happens; design for it.

## Lead discipline (how tokens are saved)

- **Write nothing.** No code, no recipes, no docs. If you catch yourself
  reaching for `Edit`, you are doing impl's job.
- **Don't poll.** No periodic `TaskList` calls. The team task list is for
  the teammates; the lead reads it only on suspicion of a hang.
- **Don't echo.** Teammate messages are already rendered to the user. A
  one-line ack is enough.
- **Don't answer idle notifications.** They are not requests for input.
- **Intervene only on:** scope violation, >3 fix rounds on the same fail,
  impl claiming PASS without runner confirmation, or a teammate going
  off-mission.

## Reviews arriving mid-slice route through the team

When a code review surfaces during a live team — most commonly the user
running `/plannotator-review` after a runner-PASS, but also PR comments,
inline `gh` review threads, or "ich habe folgendes gefunden"-style user
feedback — the lead **does not handle it directly**. The team handles it.

Default flow:

1. **Planner** receives the raw review payload (the markdown blob, the PR
   comment thread, the user's verbatim text — *not* the lead's
   pre-classification). Asked to triage each comment as
   `push-back` (architectural correctness, the reviewer's intuition is
   wrong) / `fix` (small edit) / `refactor` (larger change), and return a
   knappe mapping table per comment with proposed response and target file
   paths.
2. **Implementer** receives the planner's mapping. Edits the files.
3. **Runner** re-verifies. If the diff is purely comments / spec wording /
   type-aliases with no behavioral change, the lead may downgrade the
   verifier scope (e.g. `typecheck + openspec validate` only) — but
   *runner* still runs it, not the lead.
4. **Lead** acks each step in 1–2 lines and writes the user-facing summary
   at the end.

Why: the same lead-discipline rules that protect tokens during the slice
apply during the review-response. A plannotator round usually adds 3–10
small edits; doing them as lead burns opus on sonnet-level work and leaves
three paid-for agents idle. The trigger to delegate is *the team is alive*,
not *the change is large*.

Self-edit only when the team has already been disbanded (`TeamDelete`).

The architectural-pushback case is the trap: it feels like reasoning work,
which feels like lead work. It's not — pushback still ends in either a
small spec/comment edit (impl) or a verbal answer (planner-drafted, lead
relays). If the planner classifies a comment as `push-back`, the planner
also drafts the citation-backed reply; the lead doesn't re-derive it.

## Scope walls

Bake the file boundaries into the impl spawn prompt:

> Stay in: `<list of paths>`. Do not touch: `<list of paths>`. Do not
> modify production source unless a real bug is uncovered during
> verification — surface it, do not silently fix.

The runner's prompt: `Read-only + Bash. You do not edit files. Always run
the verifier through the canonical command, never piecemeal sub-commands.
Also: the implementer's name may change mid-run if a replacement was
spawned (impl → impl2 → impl3, etc.). Address impl-direction messages
to whichever name the lead most recently named, or to whoever currently
owns the impl tasks per TaskList — never to a name you've held since
spawn-time.`

## Verifier coverage caveats

A passing verifier means "what's tested passed", not "everything that
was supposed to be done was done". Two real failure modes:

- **Missing test files don't fail.** If sub-task 9.4 was "add a test
  for X" and impl forgets, `cargo test` happily passes — there's no
  test to fail. The verifier reports green; the gap is invisible.
- **Unticked checkboxes don't fail.** OpenSpec/`tasks.md` checkbox
  state isn't part of most verifiers. impl can mark a phase "done"
  with sub-tasks unticked.

Defend in impl spawn prompt: "for each numbered sub-task you complete,
produce evidence — a test name, a file path, or a git diff hunk — and
tick the checkbox in tasks.md before moving on. Lead may spot-check at
phase boundaries."

Lead spot-check at phase end is cheap: `grep -c '^- \[x\]'` vs `^- \[ \]`
on tasks.md catches the second class. Reading the new test names against
the task list catches the first.

## Running this loop on Codex

Codex does not currently expose Claude Code agent-team primitives. There is
no `TeamCreate`, shared team task list, teammate mailbox, split-pane teammate
UI, teammate-to-teammate messaging, or `ScheduleWakeup` equivalent in the
Codex tool surface. Do not pretend this command can run unchanged.

What Codex can do is a lead-orchestrated approximation using subagents:

- `spawn_agent` creates bounded `worker` or `explorer` subagents.
- `send_input` redirects or nudges a spawned subagent.
- `wait_agent` waits for one or more spawned subagents to finish.
- `close_agent` shuts down a subagent when it is no longer needed.

The important behavioral difference: Codex subagents report to the lead.
They do not coordinate directly with each other. The lead owns the task list,
handoffs, verifier loop, and final synthesis.

### Codex lead recipe

Use this shape when the user explicitly asks to try the dev loop on Codex:

1. Read the scoped files and identify the deterministic verifier command.
2. Smoke-test the verifier once locally before spawning agents.
3. If the slice is unclear, spawn one `explorer` as `planner` with a
   read-only prompt. Ask it for a small task breakdown, file boundaries,
   and verifier command. Wait for it before implementation.
4. Spawn one `worker` as `impl` for implementation. Give it:
   - Exact owned files or directories.
   - "You are not alone in the codebase; do not revert edits made by
     others."
   - The numbered implementation tasks.
   - The verifier command, but tell it not to treat self-run verifier
     output as final unless the lead asked for that.
   - The evidence rule: for each sub-task, report file paths, test names,
     or diff hunks that prove completion.
5. While `impl` works, do not edit the same files locally. Only inspect
   unrelated context or prepare the verifier.
6. When `impl` returns, inspect its changed paths and run the verifier
   locally as the runner. Codex does not need a separate read-only runner
   unless the verifier is expensive and can run in parallel with lead work.
7. On verifier FAIL, send the failing excerpt back to `impl` with
   `send_input`. Keep the loop tight: exit code, failing test names, and
   the last useful output lines.
8. Repeat until the verifier passes or the same failure class repeats
   three times. At three repeats, stop and investigate as lead.
9. Close the subagent once no more follow-up is needed.

### Optional Codex runner

Spawn a separate `explorer` as `runner` only when it adds real value:

- The verifier is long-running and the lead can do useful non-overlapping
  inspection meanwhile.
- The verification requires independent read-only review of artifacts.
- You want a second agent to check whether the implementation actually
  satisfied each numbered task, not just whether tests pass.

The runner prompt must say: "Read-only. Do not edit files. Run only the
verifier command requested by the lead. Report PASS or FAIL with the exit
code and a short output excerpt." Because Codex subagents do not message each
other, the lead forwards runner failures to `impl`.

### Codex watchdog substitute

Codex has no scheduled wakeup. If a subagent appears stuck, the lead should:

1. `wait_agent` with a bounded timeout only when the result is needed.
2. If it times out, continue non-overlapping work if possible.
3. If progress is required, `send_input` one direct checkpoint request:
   "Report status: working on X, blocked by Y, or ready for verification."
4. If the subagent remains silent and the task is still needed, close it
   and spawn a replacement worker with the instruction: "Read `git diff`
   and continue from the partial work; do not revert predecessor changes."

Replacement is a lead decision. There is no shared ownership field to update,
so record the live owner in the lead's own notes or user-facing status.

## Cleanup

When runner reports PASS and all tasks are completed:

1. `SendMessage` each teammate with `{type:"shutdown_request"}`. Wait for
   approvals.
2. `TeamDelete` once both have shut down.

Confirm the result with the user before cleanup. They may want a follow-up
phase, in which case keep the team alive and queue more tasks.

## A minimal example shape (not a template — illustrative)

Scope: "Replace `axios` with native `fetch` across `src/api/`. Public
function signatures stay the same. Verifier: `npm test && npx tsc --noEmit`."

Tasks:
1. `impl`: rewrite `src/api/client.ts` to use `fetch`; preserve exports.
2. `impl`: update call sites in `src/api/*.ts`; remove `axios` import.
3. `impl`: drop `axios` from `package.json` dependencies.
4. `runner`: `npm test && npx tsc --noEmit` — PASS or FAIL.

Task #4 `addBlockedBy: [1, 2, 3]`. impl pings runner when 1–3 done. On
FAIL the runner posts the failing test name and the tsc error excerpt;
impl fixes; runner re-runs. Done when #4 is green.

A Rust variant looks the same: scope = "migrate `error_chain` to
`thiserror` in `crates/foo`", verifier = `cargo check --all-targets &&
cargo test -p foo`. Same three roles, same protocol.

## Hard rules

- **Verifier is deterministic.** Subjective review belongs in `team:debate`.
- **Lead stays text-light.** Long lead replies are a smell that you are
  doing the team's job for them.
- **No nested teams.** Teammates cannot spawn their own team.
- **Permissions inherit at spawn.** Set the lead's permission mode
  appropriately *before* `TeamCreate`.
- **Cleanup belongs to the lead.** Teammates must not call `TeamDelete`.

## Argument: $ARGUMENTS
