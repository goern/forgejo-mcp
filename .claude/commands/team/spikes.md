---
name: "Team: Spikes"
description: Spawn N parallel `spike-runner` teammates to execute autonomous feasibility spikes against a single research spec
category: Agent Teams
tags: [agent-teams, sandbox, spikes]
---

Spawn parallel autonomous spike-runner teammates against a research spike spec.

**Input** (`$ARGUMENTS`): one of
- A spec file path (e.g. `docs/research/predicate-language-spike.md`) — spawn one teammate per spike defined in the spec
- A spec path plus a comma-separated list of spike-name=type pairs (e.g. `docs/research/predicate-language-spike.md | cel-wasm=crate, views-lockpick=module`)
- Empty — ask which spec

## Pre-flight

1. **Verify agent teams are enabled** (env flag). If not, refuse + instructions.

2. **Verify the spec file exists** under `docs/research/` and contains pass criteria + a reporting template. Refuse otherwise.

3. **Verify each spike's `sandbox/sk-sndbx-<name>/` does NOT already exist.** Conflict = refuse for that one. Other spikes can still proceed.

4. **Confirm the user wants autonomous execution.** Spike-runner has Bash. It will start a local `spacetime start` node, publish modules, run reducers. If you (the lead) suspect the user wanted to drive these manually, ask first using AskUserQuestion. Skip this confirmation if the spec explicitly authorizes autonomous runs.

5. **Local-node prerequisite check.** Run `spacetime --version` via Bash to confirm the CLI is on PATH. If not, refuse — sandbox doctrine says spike-runner brings up its own local node, which requires the binary.

## Team spawn

Create a team with one `spike-runner` teammate per spike. Each gets `model: sonnet` (from the agent file). For 2+ spikes, this is the full team; for a single spike, prefer single-shot delegation (`Use the spike-runner agent to ...`) instead — teams are overkill for N=1.

For each spike, spawn with this prompt:

```
Execute spike `<spike-name>` (type=<crate|module>) per spec
`<spec-path>`. Run autonomously to verdict per the spike-runner
agent's loop. Produce all required teaching artifacts.

Hard reminders:
- DB name = sk-sndbx-<spike-name>
- Local node only (sandbox default) — start your own background `spacetime start`
- Append your `## Results — <YYYY-MM-DD>` section to the spec file
- Write `sandbox/sk-sndbx-<spike-name>/README.md` with one-line verdict + anchor link
- Stop at 2× the spec's time-box

When done, post a one-line verdict + path to your README to the team
mailbox addressed to `lead`.
```

## Mediation loop (lead's job — i.e. you)

1. Let the teammates run. **Do not implement anything yourself** — wait.
2. As verdicts arrive, collect them. Don't synthesize until all teammates have reported (or hit timeout).
3. Once all teammates have reported, append a **joint synthesis** section to the spec file:

   ```
   ## Joint Verdict — <YYYY-MM-DD>
   - Spikes run: <list>
   - Per-spike verdicts: <table>
   - Joint recommendation: <PROCEED | FALLBACK | NO-GO> with reasoning
   - Open questions for the user: <bullets>
   ```

4. **Do not delete spike directories.** That's the user's call after they've read the reports.

5. List any cleanup state (running `spacetime start` PIDs, undeleted local DBs, dirty working trees) for the user.

## Hard rules

- **Spike-runners are autonomous.** Do not micromanage them. If one is stuck for >10 min, message it once asking for status; don't replace it unless dead.
- **Lead does not edit spike directories.** Each spike dir is owned by its teammate.
- **Lead does not run `cargo` or `spacetime` for the spikes.** That's the teammates' job. The lead may run `git status`, `git diff` for the synthesis section.
- **No `git commit`, no `git push`** from the lead during the run. The user reviews and commits.
- **Cleanup of the team itself**: after synthesis is written, ask the user to confirm before running `clean up the team`.

## Argument: $ARGUMENTS
