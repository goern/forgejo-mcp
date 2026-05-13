# Design

## Context

The `/code-review` skill (shipped by `forgejo-code-review-skill`, archived 2026-05-12) performs multi-agent PR review interactively in Claude Code. This change wraps that skill in a Forgejo Actions workflow so reviews run automatically on every PR event. The reference architecture is Anthropic's GitHub action (`anthropics/claude-code-action`), adapted to Forgejo's trigger semantics and runner ecosystem.

The forgejo-mcp project already ships pre-built binaries via `.forgejo/workflows/release.yml` (goreleaser, attached to Codeberg releases). The `/code-review` skill orchestrates `get_pull_request_by_index`, `get_pull_request_diff`, `get_file_content`, `list_pull_reviews`, and `create_pull_review` — all reachable through the MCP server from a runner with a `FORGEJO_TOKEN`.

The action does **not** add new Go code, MCP tools, or skill changes. It is workflow plumbing only: installing Claude Code, registering forgejo-mcp as an MCP server, invoking the skill in `-p` mode, and reporting results back to the PR.

## Goals / Non-Goals

**Goals:**

- Single reusable workflow file at `.forgejo/workflows/claude-code-review.yml`, callable from any Forgejo repository
- Automatic review on PR `opened`, `synchronize`, `reopened`, `ready_for_review`
- Safe by construction for fork PRs (no untrusted code execution on the runner)
- Cost visibility: step summary reports model, turns, USD cost, findings count
- Hard cost cap via `--max-turns`
- Duplicate-review suppression on identical PR head SHA
- Opt-in transcript artifact for debugging

**Non-Goals:**

- No changes to the `/code-review` skill prompt or its agent pipeline
- No new MCP tools or Go code
- No support for review on `issue_comment @claude` (manual trigger)
- No incremental "review only the diff since last bot review" — duplicate suppression only
- No multi-tenant cost accounting beyond per-run `cost_usd` reporting
- No automatic key rotation or secret provisioning

## Decisions

### D1. Trigger: `pull_request_target` on four event types

**Choice:** `on.pull_request_target.types: [opened, synchronize, reopened, ready_for_review]`

`pull_request_target` runs the workflow file from the base branch with full secret access, including for fork PRs. This is the only trigger that gives both same-repo and fork PRs identical behavior. Draft PRs are skipped at the workflow level with an early `if: !github.event.pull_request.draft` guard so the runner never starts billable steps for them.

**Alternatives considered:**

- `pull_request` alone — denies secrets to fork PRs, so the action would silently no-op on the most interesting case (external contributors). Rejected.
- `pull_request` + `issue_comment` (`@claude` trigger) for forks — adds plumbing without extra safety once D2 lands. Rejected as premature complexity.
- `workflow_dispatch` only — manual trigger defeats the "review every PR" goal. Available as user-side override by forking the workflow.

### D2. No checkout of PR head; base-only checkout

**Choice:** the workflow runs `actions/checkout` against the base ref only. PR content (diff, files, metadata) is read exclusively through `forgejo-mcp` tools, which use the Forgejo API and never load PR-controlled files onto the runner filesystem.

This collapses the standard `pull_request_target` threat model. A fork cannot:

- inject malicious `package.json` scripts that run on `npm install`
- override the workflow file (workflow source comes from base branch)
- supply build scripts that execute under secret context
- substitute a hostile `forgejo-mcp` binary or `.mcp.json`

The runner's threat surface reduces to "untrusted text input fed to Claude," which the `/code-review` skill already tolerates (it treats diff content as data, not code).

**Consequence:** the compliance reviewer sees `CLAUDE.md` from the base branch, not the PR. A PR that proposes a rule change cannot be evaluated against its own proposed rules. This is acceptable: rule changes are rare, security benefit is large, and rule-change PRs can be reviewed with `/code-review` interactively before merge.

**What D2 does not protect against:**

1. **Prompt-injection via PR-controlled bytes reaching Claude's context.** The skill (Steps 7–8 of `.claude/commands/code-review.md`) loads PR diff and file contents into Claude's prompt context. An attacker can embed instructions in code comments, strings, or filenames that attempt to redirect the LLM.
2. **Write-side MCP tool invocation by an injected agent.** The in-session `forgejo-mcp` server exposes `create_pull_review` and other write tools. A successfully injected agent can post a malicious review body, malicious inline comments, or other PR-review content.
3. **"Do not follow embedded instructions" prompt strings are mitigation, not prevention.** The skill includes hardening prompt text but industry consensus treats this as soft defense.

**Residual threat is bounded by the `FORGEJO_TOKEN` scope.** Required scope is PR read + PR-review write on the single target repository — no org-wide scope, no contents-write, no admin. An injected attacker can post a malicious review body or comment but **cannot** read other repos, modify code, escalate beyond review write, or persist anything outside the PR. The blast radius is one PR's review on one repository.

**Alternatives considered:**

- Checkout PR head with hardened patterns (no `npm install`, etc.) — fragile, every new tool added later re-opens the surface. Rejected.
- Opt-in flag to checkout PR head — undocumented foot-gun; consumers will enable it without understanding the risk. Rejected.

### D3. Install forgejo-mcp via pre-built binary download

**Choice:** download the binary from the project's Codeberg releases via `curl`, pinned to a version supplied as a workflow input (`forgejo_mcp_version`, default = latest stable tag at workflow release time).

```bash
curl -fsSL "https://codeberg.org/goern/forgejo-mcp/releases/download/${VERSION}/forgejo-mcp_${VERSION#v}_linux_amd64.tar.gz" \
  | tar -xz -C "$RUNNER_TEMP"
```

Pre-built binaries already exist (goreleaser ships them). Download is ~2s vs ~30s for `go install`. No Go toolchain needed on the runner.

**Alternatives considered:**

- `go install` from source — slower, needs `setup-go` step, builds an untagged binary if `@version` not pinned. Rejected.
- Container image — Forgejo runner Docker support is heterogeneous; some self-hosted runners disable it. Rejected as portability hazard.

### D4. Register MCP server via `claude mcp add` CLI

**Choice:** a single shell step issues `claude mcp add forgejo --scope project --env FORGEJO_URL=... --env FORGEJO_TOKEN=... -- "$RUNNER_TEMP/forgejo-mcp"`. The `--scope project` flag confines registration to the runner's working tree, so the registration vanishes when the runner is recycled.

CLI approach avoids writing a `.mcp.json` file to disk (which could otherwise leak token via misconfigured artifact upload).

**Alternatives considered:**

- Generate `.mcp.json` in workflow — declarative but file-on-disk increases leak surface. Rejected.
- Hybrid template + env interpolation — more moving parts, no extra benefit. Rejected.

### D5. Capture JSON output, write step summary, opt-in transcript artifact

**Choice:** invoke Claude Code with `--output-format json`. Parse the final result object for `cost_usd`, `num_turns`, model, and the skill's findings-posted count (the skill writes a final JSON line summarizing what it posted). Write a markdown block to `$GITHUB_STEP_SUMMARY` (Forgejo Actions exposes the same variable). If workflow input `debug: true`, upload the full transcript as an artifact via `actions/upload-artifact`.

Example step summary:

```
## Claude Code Review
Model: claude-sonnet-4-6
Turns: 12 / 20 (max)
Cost: $0.08
Findings posted: 3
Status: success
```

**Alternatives considered:**

- Plain text logs only — loses parseable cost and turn data, makes opt-in budgeting impossible. Rejected.
- Always upload transcript — needless artifact churn for healthy runs. Rejected; gate on `debug` input.

### D6. Concurrency cancel + action-side dedup via SHA trailer

**Choice:** two layers, bound by a spec contract.

```
L1: concurrency:
      group: code-review-${{ github.event.pull_request.number }}
      cancel-in-progress: true
```

`synchronize` events fire repeatedly during a rebase or force-push. L1 ensures only the latest commit's review runs to completion; earlier runs are cancelled.

```
L2: pre-flight step calls list_pull_reviews via forgejo-mcp.
    If a review whose body contains the machine-parseable trailer
    <!-- forgejo-mcp-review: <current_head_sha> --> exists,
    exit 0 with a notice; do not invoke Claude.
```

L2 saves real money on manual re-runs and on `synchronize` events that fire after the head SHA is already reviewed. The dep change's design.md marks dedup as "future work" at the skill level; we implement it at the action level because the action is what spends money.

**Tagging mechanism (spec-bound, not soft-prompt):** the spec's "Dedup contract" requirement binds the workflow to append the literal trailer `<!-- forgejo-mcp-review: <head_sha> -->` to every posted review body. Because the skill's Step 12 body templates do not currently emit this trailer, a tasks.md entry will amend the skill to either (a) append the trailer when invoked with `--head-sha <SHA>` argument, or (b) accept the trailer template via `additional_instructions` injection. Option (a) is the contract; option (b) is the bridge while the skill amendment lands.

**Alternatives considered:**

- L1 only — duplicate reviews on every manual re-run. Rejected.
- L1 only + document as known limitation — matches dep design but ignores cost. Rejected.
- Soft prompt-injection via `additional_instructions` as the sole mechanism — unreliable for a contract the spec promises. Rejected; trailer must be enforced at skill level via amendment.

### D7. Default `GITEA_TOKEN`, allow PAT override

**Choice:** default the workflow to use the ephemeral `${{ secrets.GITEA_TOKEN }}` (Forgejo's auto-injected token) with an explicit `permissions:` block:

```yaml
permissions:
  pull-requests: write
  contents: read
```

A repository can override by supplying `forgejo_token: ${{ secrets.MY_PAT }}` as a workflow input.

**Verification (C4 spike, 2026-05-13):** confirmed Codeberg's Forgejo (v11.x) honors `pull-requests: write` for `create_pull_review` on **fork PRs under `pull_request_target`**. Evidence:

- spike repo `goern/c4-spike-target` ← fork `tinytalesshop/c4-spike-target`, PR #2
- workflow ran with ephemeral `${{ secrets.GITEA_TOKEN }}` only (no PAT)
- review posted: id `1350819`, bot identity `forgejo-actions`, state `COMMENT`, HTTP 201
- follow-up test: modified fork's workflow body string; **BASE** workflow body posted, not fork's → Forgejo honors `pull_request_target` BASE-workflow-file semantics (display label `pull_request` is cosmetic)
- spike artifacts: `openspec/changes/forgejo-action-code-review/spikes/c4-ephemeral-token-fork-pr/`

The override path (`forgejo_token: ${{ secrets.MY_PAT }}` input) remains documented as the safe fallback for Forgejo versions other than v11.x, and for consumers who want human-attributed reviews instead of `forgejo-actions[bot]`.

**Bot identity trade-off:** ephemeral token attributes reviews to `forgejo-actions[bot]`. PAT path attributes to the chosen user. Document both; let consumers choose.

**Alternatives considered:**

- Require custom PAT always — friction for adopters, manual rotation. Rejected as default.
- Ephemeral only, no override — leaves consumers stranded if their Forgejo version rejects bot reviews on PRs. Rejected.

### D8. Runner: `ubuntu-latest`, explicit `setup-node@v4` at Node 20

**Choice:** the workflow declares `runs-on: ubuntu-latest`. The first step is `actions/setup-node@v4` with `node-version: 20`. Subsequent steps install Claude Code via `npm install -g @anthropic-ai/claude-code`. Other required tools (`curl`, `tar`, `jq`) are assumed present on stock ubuntu-latest.

`runs-on` cannot be parametrized via workflow inputs at the workflow file level. Consumers needing Codeberg's `codeberg-small` (or other labels) must fork the workflow file and edit the line; that pattern is documented in the setup notes.

**Alternatives considered:**

- Default to `codeberg-small` — works on Codeberg, breaks every self-hosted Forgejo. Rejected.
- Multiple workflow variants (one per common runner) — maintenance burden. Rejected.

### D9. Distribution: copy-paste primary, reusable `uses:` secondary (pending C5 spike)

**Choice:** ship the workflow at `.forgejo/workflows/claude-code-review.yml` in the `goern/forgejo-mcp` repository. Two consumer paths, in order of certainty:

**Path A — copy-paste (supported, certain).** Consumers copy `claude-code-review.yml` verbatim into their own repository's `.forgejo/workflows/` directory. Trade-off: drift from upstream; no automatic security patches.

**Path B — cross-repo `uses:` reference (recommended when supported by target Forgejo).** Consumers reference the workflow from their own caller workflow:

```yaml
# consumer .forgejo/workflows/code-review.yml
on:
  pull_request_target:
    types: [opened, synchronize, reopened, ready_for_review]

jobs:
  review:
    uses: goern/forgejo-mcp/.forgejo/workflows/claude-code-review.yml@v0.21.0
    secrets:
      ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
    with:
      model: claude-sonnet-4-6
```

Consumers MUST pin to a tag (`@v0.21.0`), never `@main`. Setup docs make this explicit.

**Stalemate (C5):** Forgejo Actions' cross-repo `workflow_call` semantics are not documented as GitHub-equivalent. Specific concerns: (i) does `secrets: inherit` propagate `ANTHROPIC_API_KEY` correctly across repo boundaries; (ii) does tag-resolution honor immutable tags; (iii) does the called workflow get the caller's `github.event` context for `pull_request_target`. Spike result drives whether Path B is documented as supported or marked experimental.

**Alternatives considered:**

- Copy-paste only — central security patches don't propagate. Acceptable fallback if C5 spike fails.
- Composite action (`action.yml`) — Forgejo Actions composite-action support is behind reusable-workflow support in practice. Rejected; revisit if maintenance costs warrant.

### D10. Failure semantics matrix

Each named failure mode has a known exit path. The workflow surfaces the failure with the quoted message:

| Failure | Step | Exit | Surface |
|---------|------|------|---------|
| `ANTHROPIC_API_KEY missing` | preflight | fail | step log, no Claude invocation |
| `FORGEJO_TOKEN missing` (when override path active) | preflight | fail | step log, no Claude invocation |
| `Claude Code install failed` | setup | fail | step log, step summary records "setup failure" |
| `forgejo-mcp download failed` | setup | fail | step log + summary |
| `MCP registration failed` | setup | fail | step log + summary |
| `duplicate review on HEAD SHA` | dedup pre-flight | **exit 0** | step summary: "skipped, already reviewed" |
| `Claude exited non-zero` | review | fail | transcript uploaded; step summary records exit code |
| `max_turns exhausted` | review | **exit 0** | step summary warning; whatever findings posted remain |
| `inconclusive run (max_turns trip before posting)` | review | **exit 0** | step summary records `max_turns_exhausted` with zero findings |
| `Forgejo API 4xx/5xx during MCP call` | review | fail | transcript uploaded; step summary records last MCP error |

**Rationale for `max_turns` soft-fail:** `--max-turns` trips most often mid-pipeline (skill Steps 8–11) before the post step (Step 12) runs. Partial-review-not-posted is the expected outcome, not a bug. The step summary surfaces `max_turns_exhausted` so consumers raise the limit. **Recommended floor: `max_turns: 25`.** Setting it lower produces zero-finding runs that still cost money.

### D11. Skill invocation prompt shape

**Choice:** the action invokes Claude Code with the literal prompt string `/code-review ${{ github.event.pull_request.number }}`. The PR number comes from the workflow event context. The `--dry-run` flag is **not** passed (the skill posts by default; passing `--dry-run` would suppress posting and break the entire purpose of the action).

The action **MUST NOT** rely on the skill's branch-based auto-detection (`git branch --show-current` in skill Step 3). Under D2's base-only checkout, the current branch is the base branch, so auto-detect either fails or matches the wrong PR (e.g., a different PR landing into the same base).

**Alternatives considered:**

- Pass `--dry-run` and have the action post via a separate MCP call — the skill is the single source of truth for review formatting and confidence filtering; splitting that responsibility duplicates logic. Rejected.
- Let the skill auto-detect from `GITHUB_REF` or `FORGEJO_REF` env vars — skill's current logic uses `git branch --show-current` only, not env vars. Future skill amendment could add env-var detection, but that's a skill change, not an action change. Rejected for now.
- Omit the PR number, fail loudly if auto-detect fails — silent foot-gun; the skill might silently match the wrong PR if a different PR lands into the same base branch. Rejected.

### D12. `--allowedTools` whitelist: MCP-only, no Bash fallback

**Choice:** the action invokes Claude Code with `--allowedTools` set to the minimal MCP surface needed for the review:

```
mcp__codeberg__get_pull_request_by_index,
mcp__codeberg__list_pull_request_files,
mcp__codeberg__get_pull_request_diff,
mcp__codeberg__get_file_content,
mcp__codeberg__list_pull_reviews,
mcp__codeberg__create_pull_review,
Task
```

(`Task` is included because the skill dispatches parallel subagents via the Task tool; the subagents inherit the same `--allowedTools` whitelist.)

**No `Bash(...)` entries. No `Read`. No `AskUserQuestion`.**

**Rationale:**

- The skill's own `allowed-tools` frontmatter includes `Bash(forgejo-mcp --cli:*)` as an MCP fallback path. If the action inherits that fallback, an injected agent (see D2 "What D2 does not protect against") can craft arbitrary `--cli` calls within FORGEJO_TOKEN scope — broader blast radius than the explicitly enumerated MCP tools.
- Dropping the Bash fallback means: if MCP calls error in the action context, the skill's fallback dies silently. The workflow surfaces those errors via D10's "Forgejo API 4xx/5xx during MCP call" row, so failures are observable, not silent.
- `Read` is excluded because base-only checkout means the runner has only base-branch source on disk. The action explicitly does not want Claude reading runner-local files; compliance context (`CLAUDE.md`, `AGENTS.md`) comes from MCP at the base ref.
- `AskUserQuestion` is excluded because the action is non-interactive (`-p` mode).

**Trade-off:** brittleness (no Bash fallback) for tighter blast-radius (no shell escape from prompt injection). FORGEJO_TOKEN scope (D2, D7) further bounds damage.

**Alternatives considered:**

- Inherit the skill's full `allowed-tools` set including `Bash(forgejo-mcp --cli:*)` — opens prompt-injection escape route to shell. Rejected.
- Inherit `Read` for runner-local files — base-only checkout means PR-side `CLAUDE.md` is not on disk anyway; reading base-side is what MCP `get_file_content` already provides at the chosen ref. Rejected.
- Allow `Bash(:*)` with a guard prompt — guard prompts are the same soft defense D2 calls out. Rejected.

Soft-fail (exit 0) cases: duplicate review (intended skip) and max-turns exhaustion (partial review is valuable and the cost cap worked as designed).

## Risks / Trade-offs

**[Fork PR threat model]** D2 is load-bearing. If a future change to the workflow re-introduces PR-head checkout (e.g., to run a linter as additional context), the `pull_request_target` + secrets combination becomes dangerous overnight.
→ Mitigation: a CI check in this repository fails if `.forgejo/workflows/claude-code-review.yml` contains the literal string `pull_request.head.sha` or `ref: ${{ github.event.pull_request.` in a checkout step. Mechanical guard against accidental regression.

**[Concurrency cancel races]** L1 cancels in-flight runs on rapid `synchronize`. If Claude has already posted a partial review before cancellation, that review remains; the next run (against newer SHA) posts a fresh review. Net effect: occasional duplicate-looking reviews when force-pushing fast.
→ Acceptable. L2 dedup catches identical-SHA duplicates; rapid force-push to different SHAs is genuinely different content and warrants re-review.

**[Forgejo runner heterogeneity]** Self-hosted Forgejo installs may have stripped-down runners without Node, `jq`, or recent `curl`. Stock `ubuntu-latest` on Codeberg is well-known but self-hosted varies.
→ Document required tools explicitly. Surface clear errors on first failing step. Do not silently install missing tools (root-level apt-get on unknown runner = bad behavior).

**[Claude Code CLI breaking changes]** `npm install -g @anthropic-ai/claude-code` installs latest by default. A breaking change to `--output-format` JSON shape or `claude mcp add` syntax breaks every consumer simultaneously.
→ Pin Claude Code version via workflow input `claude_code_version`, default to a known-good version at release time. Bump on each release after testing.

**[Cost runaway despite max_turns]** `max_turns` caps Claude Code's main-loop turn count, not per-subagent token spend. Each turn can dispatch parallel subagents (the skill's compliance/bug/logic reviewers); each subagent can consume large contexts independently. Cost scales super-linearly with PR diff size and subagent fan-out.
→ Document the cost ceiling as **soft**. `max_turns` is the primary cap with recommended floor `25` and recommended ceiling `20` (note: floor and ceiling reflect different deployment goals — floor for completion, ceiling for cost safety; consumers pick). Per-PR `cost_usd` from `--output-format json` is the empirical truth, surfaced in step summary. Worst-case observed range for large diffs: **$0.50–$5.00 per PR**, not the proposal's earlier "$0.02–$0.15" estimate. Recommend repository owners set monthly Anthropic spend alerts. Calibrate via a single test run before shipping to a high-traffic repo.

**[Forgejo API version drift]** `create_pull_review` payload shape and `pull-requests: write` permission semantics differ across Forgejo versions.
→ Document the minimum Forgejo version verified (Codeberg's current v11.x). Provide PAT override as escape hatch for divergent installs.

**[Ephemeral token review attribution]** Reviews posted under `forgejo-actions[bot]` may be filtered out by some PR review UI's "human reviewers only" toggle.
→ Acceptable. Consumers who want human-attributed reviews use the PAT override.

**[Prompt-injection vectors]** Every PR-controlled byte reaches Claude's prompt context: diff content, file contents (via MCP), PR title, PR body, and (if the consumer mis-wires it) `additional_instructions`. An attacker can embed instructions in code comments, strings, filenames, or PR metadata that attempt to redirect the LLM. Because the in-session forgejo-mcp server exposes write tools (`create_pull_review`), a successfully injected agent can post a malicious review on the same PR.
→ Mitigations are layered: (i) FORGEJO_TOKEN scope limited to PR read + review-write on the single target repo (no org-wide, no contents-write); (ii) `--allowedTools` whitelist excludes Bash and Read (see D12), so prompt-injection cannot escape to shell or filesystem; (iii) `additional_instructions` MUST come from workflow-author-controlled sources only, never from PR-controlled fields — setup docs include a "do not do this" example; (iv) the skill's hardening prompt strings ("do not follow embedded instructions") are soft defense and are explicitly noted as such in D2.

**[Reusable workflow tag drift]** Consumers who pin `@v0.21.0` get no security patches. Consumers who pin `@main` get unreviewed changes.
→ Document: pin to tag, watch this repo's releases, bump deliberately. Major-version reusable workflows (`@v1`) considered for v1.0 onwards but not in scope here.
