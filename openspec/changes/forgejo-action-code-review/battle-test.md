# Battle Test — forgejo-action-code-review

**Date:** 2026-05-12
**Targets:** proposal.md, design.md, specs/action-code-review/spec.md
**Team:** adversary (devils-advocate, opus), defender (proponent, opus)

## Verdict

**Patches applied; C4 spike resolved; C5 spike deferred. Proceed to tasks.md.**

Of 9 critiques: 7 patched, 1 resolved by spike (C4), 1 deferred via spec split (C5 — Path A copy-paste is supported; Path B cross-repo `uses:` is recommended-when-supported and tracked in bd for follow-up).

The change was written against the dep change's **design**, not its **shipped artifact**. The `/code-review` skill drifted from its archived design before this action layered on top, and this change inherited the drift. The single most damaging finding (C1): the action invoked the skill with `--comment` to enable posting; the shipped skill posts by **default** and accepts `--dry-run` to suppress. Every action artifact referenced the wrong flag. C2 was similarly load-bearing: the D6 dedup design assumed the skill embeds head-SHA in review bodies — it does not. Both fixed by patches and a follow-up tasks.md entry to amend skill Step 12 body templates.

C4 spike (2026-05-13) validated D2 (BASE workflow file executes under `pull_request_target` on fork PRs; Forgejo's display label `pull_request` is cosmetic, not semantic) and D7 (ephemeral `GITEA_TOKEN` + `permissions: pull-requests: write` permits `create_pull_review` on fork PRs on Codeberg v11.x). Spike artifacts retained under `openspec/changes/forgejo-action-code-review/spikes/c4-ephemeral-token-fork-pr/` for reproducibility.

## Surviving Critiques

### C1. `--comment` flag does not exist; posting is the skill's default

**Severity:** blocker
**Failure mode:** workflow passes `--comment` to a skill that does not recognize it; or omits it and posts anyway. Either way the action ships against a non-existent flag contract. The proposal's "supports `--comment` flag to post findings" is fiction relative to the shipped `.claude/commands/code-review.md`.
**Hits:** proposal.md "What Changes" bullet 2; design.md D6 "Tagging mechanism" paragraph; spec.md "Findings posted as a Forgejo PR review" requirement.
**Canonical conflict:** `.claude/commands/code-review.md:3` declares `argument-hint: [PR#] [--dry-run]`; line 12 reads "By default, findings are posted as a PR review with inline comments. Use `--dry-run` for terminal-only output."
**Falsifier:** `grep -n "dry-run\|--comment" .claude/commands/code-review.md` — done; confirms inversion.
**Disposition:** patch (concede-patch)

### C2. D6 dedup tagging is fabricated

**Severity:** blocker
**Failure mode:** D6 promises action-side dedup via "current head SHA tag in review body footer." Skill's body templates (Step 12 of `.claude/commands/code-review.md`) use fixed strings with no SHA and no placeholder. Without a contract-level mechanism, dedup either no-ops (every run posts a new review) or relies on soft prompt-injection via `additional_instructions` — unreliable for a mechanism the spec promises to honor.
**Hits:** design.md D6 paragraph beginning "Tagging mechanism"; the change's "duplicate-review suppression" goal in design.md "Goals" list.
**Canonical conflict:** skill design.md (archived dep change) marks dedup as future work; shipped skill provides no tagging surface.
**Falsifier:** `grep -n "sha\|SHA\|<!--" .claude/commands/code-review.md` — done; no trailer mechanism present.
**Disposition:** patch (concede-patch — choose Option A drop L2 or Option B lift dedup to spec contract + skill amendment)

### C3. D2 protects filesystem but not LLM prompt context

**Severity:** major
**Failure mode:** D2's "no PR-head checkout" claim collapses the runner-filesystem threat model but leaves the LLM prompt-injection vector wide open. The skill (Steps 7.2 and 8) loads PR-controlled diff and file content into Claude's context, and the in-session forgejo-mcp server has write-side tools (`create_pull_review`). Industry-weak "do not follow embedded instructions" defenses are mitigation, not prevention. D2 reads as if it solved the whole problem; it solved one half.
**Hits:** design.md D2 "This collapses the standard pull_request_target threat model"; design.md Risks "`additional_instructions` injection vector" bullet (too narrow); spec.md "Required secrets" requirement.
**Canonical conflict:** design honestly says "untrusted text input fed to Claude" remains; that line undersells what an injected agent can do with a tool-equipped Claude.
**Falsifier:** code-read of `.claude/commands/code-review.md` Step 7-8 and the in-session MCP tool list — done; PR-controlled bytes do reach Claude's context.
**Disposition:** patch (concede-patch — bound residual threat via FORGEJO_TOKEN scope; rename Risks bullet to "Prompt-injection vectors")

### C4. Fork-PR ephemeral-token `create_pull_review` — RESOLVED

**Severity:** major (resolved)
**Failure mode (originally):** D7 marked "verification owed" for whether Codeberg Forgejo v11.x honors `pull-requests: write` on `${{ secrets.GITEA_TOKEN }}` for fork PRs.
**Spike result (2026-05-13):** ran C4 spike against `goern/c4-spike-target` ← fork `tinytalesshop/c4-spike-target`. Ephemeral `GITEA_TOKEN` + `permissions: pull-requests: write` **PERMITTED** `create_pull_review` on the fork PR. Bot identity: `forgejo-actions` (review id 1350819 on PR #2, HTTP 200). Follow-up test: pushed a modified workflow to the fork's `spike-pr` branch with a distinctive body string; the BASE workflow body was posted, **not** the fork's modified body. Conclusion: Forgejo Actions honors `pull_request_target` semantics (BASE workflow file executes with secrets), even though the displayed event label is `pull_request` (cosmetic UI artifact). Both D2 and D7 validated.
**Disposition:** **defend / verified by spike.** No design changes needed. Spike artifacts retained under `openspec/changes/forgejo-action-code-review/spikes/c4-ephemeral-token-fork-pr/` for reproducibility.

### C5. Cross-repo `workflow_call` semantics on Forgejo — DEFERRED

**Severity:** major
**Failure mode:** D9 mandates `uses: goern/forgejo-mcp/.forgejo/workflows/claude-code-review.yml@<tag>` as distribution. Forgejo Actions' `workflow_call` cross-repo semantics, `secrets: inherit` behavior, and tag resolution are not GitHub-equivalent.
**Hits:** design.md D9; spec.md "Reusable Forgejo Actions workflow definition" Scenarios.
**Canonical conflict:** none directly; Forgejo Actions docs not loaded into repo.
**Falsifier (scaffolded but not executed):** spike scaffolding lives at `openspec/changes/forgejo-action-code-review/spikes/c5-cross-repo-workflow-call/`. Library + consumer repos created on Codeberg (`goern/c5-spike-lib`, `goern/c5-spike-consumer`). Tag, secrets, and trigger commit not yet applied.
**Disposition:** **deferred to follow-up bd issue.** Spec patch already applied (copy-paste vs. `uses:` paths split into two scenarios). D9 documents Path A (copy-paste) as the certain path and Path B (cross-repo `uses:`) as recommended-when-supported. Implementation can proceed on Path A; C5 spike result will determine whether Path B is documented as supported or marked experimental.

### C6. `max_turns` trip kills posting; "submit partial review" unenforceable

**Severity:** major
**Failure mode:** spec promises "the workflow SHALL still attempt to submit the partial review (if any findings exist) before exiting" when `max_turns` is reached. Step 12 (the post step) is the last skill pipeline step. `--max-turns` typically trips mid-pipeline (Steps 8-11). Claude Code has no "post on the way out" hook. Spec contract is unenforceable.
**Hits:** spec.md "Cost cap via max-turns" Scenario "max_turns reached"; design.md D10 "max_turns exhausted" row.
**Canonical conflict:** `.claude/commands/code-review.md` step ordering puts posting last; Claude Code CLI has no exit hook.
**Falsifier:** test run with `--max-turns 5` on a non-trivial PR — done by code-read of skill pipeline; trip occurs before Step 12.
**Disposition:** patch (concede-patch — rewrite scenario to record `max_turns_exhausted` status in step summary, accept zero-finding outcome, recommend `max_turns: 25` floor)

### C7. `--max-turns` is soft cost cap; proposal's "$0.02-0.15" understates

**Severity:** major
**Failure mode:** proposal claims `--max-turns` is "a hard cap" on cost; spec says "hard upper bound on Claude Code execution." `--max-turns` bounds main-loop turn count, not subagent token spend. Three parallel Opus subagents per main-loop turn can each consume large contexts. Worst-case cost on a large PR is closer to $0.50-$5.00 than $0.02-$0.15. Marketing copy must match implementation.
**Hits:** proposal.md "Cost implications"; spec.md "Cost cap via max-turns"; design.md Risks "Cost runaway despite max_turns" (which already admits soft cap — contradicting spec).
**Canonical conflict:** Claude Code's documented `--max-turns` semantics + dep change design's tiered subagent model.
**Falsifier:** single test run on a 500-line PR with `cost_usd` reported via `--output-format json` — confirms or refutes range.
**Disposition:** patch (concede-patch — rewrite to "soft cap on main-loop turns"; surface `cost_usd` in step summary; recalibrate proposal cost range)

### C8. Action must pass PR# explicitly; base-only checkout breaks auto-detect

**Severity:** major
**Failure mode:** skill Step 3 auto-detects PR via `git branch --show-current` when PR# not passed. With D2's base-only checkout, current branch is the base branch — auto-detect either fails or matches the wrong PR (e.g., a PR landing into base). Spec scenario says "against the PR identified by the workflow event context" without specifying mechanism.
**Hits:** spec.md "Non-interactive Claude Code invocation" Scenario "/code-review skill is the entry point".
**Canonical conflict:** `.claude/commands/code-review.md` Step 3 (auto-detect logic) + D2's base-only checkout decision.
**Falsifier:** code-read of skill Step 3 + workflow checkout strategy — done; confirms mismatch.
**Disposition:** patch (concede-patch — invocation prompt MUST be literal `/code-review ${{ github.event.pull_request.number }}`; add D11 "Skill invocation prompt shape")

### C9. `--allowedTools` whitelist unspecified; Bash fallback inheritance has security implications

**Severity:** major
**Failure mode:** skill's `allowed-tools` frontmatter includes `Bash(forgejo-mcp --cli:*)` as MCP fallback. Action's `--allowedTools` must either inherit (an injected agent can craft arbitrary `--cli` calls within FORGEJO_TOKEN scope) or drop (skill's MCP fallback silently dies). Either choice is load-bearing for security and reliability; design leaves it unspecified.
**Hits:** proposal.md "Capabilities" (mentions `--allowedTools` as cost control with no detail); design.md (no decision present).
**Canonical conflict:** `.claude/commands/code-review.md` frontmatter `allowed-tools` declaration.
**Falsifier:** code-read of skill frontmatter — done; Bash MCP fallback is present.
**Disposition:** patch (concede-patch — add D12 "`--allowedTools` whitelist: MCP-only, no Bash fallback"; add corresponding spec requirement)

## Conceded Patches

Apply these before tasks.md is written and implementation begins. Numbers map to critiques above.

### proposal.md

- **"What Changes" bullet 2** (C1): replace `invokes the /code-review skill in non-interactive (-p) mode` with `invokes /code-review <PR#> in non-interactive (-p) mode (skill posts by default; --dry-run is not passed)`.
- **"Impact" → "Cost implications"** (C7): replace `Each PR review run costs ~$0.02-0.15 depending on diff size and model tier. --max-turns provides a hard cap.` with `Each PR review run costs ~$0.10-$5.00 depending on diff size, model tier, and subagent dispatch fan-out. --max-turns caps main-loop turns (soft cost cap); per-run cost_usd is surfaced in the step summary for monitoring; large PRs scale cost super-linearly with subagent count.`
- **"Impact" → "Capabilities"** (C9): expand `cost controls (--max-turns, --allowedTools)` to `cost controls (--max-turns, --allowedTools restricted to MCP review tools per design D12)`.

### design.md

- **D2** (C3): append sub-section **"What D2 does not protect against"** listing (1) prompt-injection via PR-controlled bytes reaching Claude's context, (2) write-side MCP tool invocation by an injected agent, (3) "do not follow embedded instructions" prompt-string defense as mitigation not prevention. Bound residual threat: "FORGEJO_TOKEN scope (PR read + PR-review write only on the target repo). An injected attacker can post a malicious review body or comment but cannot read other repos, modify code, or escalate beyond review write."
- **D6** (C1 + C2): rewrite "Tagging mechanism" paragraph. Pick **Option A** (drop L2 dedup, document duplicate-review limitation matching dep design, remove "duplicate-review suppression" from Goals) **or Option B** (lift dedup to a spec.md "Dedup contract" requirement that binds the workflow to append `<!-- forgejo-mcp-review: <head_sha> -->` trailer, requires skill amendment via separate tasks.md entry). User must choose.
- **D7** (C4): retain "Verification owed" — but mark explicitly **stalemate pending spike**. After spike, either remove note (ephemeral works) or flip default to PAT-required (ephemeral does not work for fork PRs).
- **D9** (C5): lead with copy-paste as the supported distribution path; describe cross-repo `uses:` as recommended **when supported by the target Forgejo version**. List specific cross-repo concerns: `secrets: inherit` behavior, tag resolution, token grants. Mark `uses:` path as **stalemate pending spike**.
- **D10** (C6): clarify "max_turns exhausted" row rationale — "trips most often mid-pipeline before Step 12, so partial-review-not-posted is the expected outcome. Surface `max_turns_exhausted` in step summary so consumers raise the limit. Recommended floor: 25." Add new failure row: `| inconclusive run (max_turns trip before posting) | review | exit 0 | step summary records max_turns_exhausted with zero findings |`.
- **Risks → "`additional_instructions` injection vector" bullet** (C3): rename to **"Prompt-injection vectors"**. Expand to note every PR-controlled byte (diff, file contents, PR title/body) reaches Claude's context. Mitigation: scope FORGEJO_TOKEN to PR read + review-write on the single target repo, no org-wide scope.
- **Risks → "Cost runaway despite max_turns" bullet** (C7): strengthen — recommend `max_turns: 20` ceiling and document worst-case cost range `$0.50-$5.00 per large-PR review`.
- **New decision D11** (C8): **"Skill invocation prompt shape"** — invocation is the literal string `/code-review ${{ github.event.pull_request.number }}`. PR# from event context. No `--dry-run`. Skill's branch-based auto-detection bypassed because base-only checkout makes it unreliable.
- **New decision D12** (C9): **"`--allowedTools` whitelist: MCP-only, no Bash fallback"** — enumerate exact allowed tools (`mcp__codeberg__get_pull_request_by_index`, `mcp__codeberg__list_pull_request_files`, `mcp__codeberg__get_pull_request_diff`, `mcp__codeberg__get_file_content`, `mcp__codeberg__list_pull_reviews`, `mcp__codeberg__create_pull_review`, `Task`). No `Bash(...)`, no `Read`, no `AskUserQuestion`. Trade-off: brittleness (no fallback) for tighter blast-radius (no shell escape from prompt injection). Bounded further by FORGEJO_TOKEN scope.

### spec.md

- **"Findings posted as a Forgejo PR review"** (C1): add sentence `The workflow MUST invoke the skill without the --dry-run flag.`
- **"Required secrets"** (C3): add scenario `Scenario: FORGEJO_TOKEN broader than PR read + PR-review write` → `WHEN the workflow runs with a FORGEJO_TOKEN whose scope exceeds PR read + PR-review write on the target repository THEN preflight SHALL fail with a clear error.`
- **"Reusable Forgejo Actions workflow definition"** (C5): split the existing scenario into two — `Scenario: PR opened on a repository that copies the workflow file` (the supported path) and `Scenario: PR opened on a repository that references via uses:` (gated on `workflow_call` support, marked uncertain until spike).
- **"Cost cap via max-turns"** (C6 + C7): rewrite first sentence to `The workflow SHALL impose a soft upper bound on Claude Code's main-loop turn count via the --max-turns flag. The cap bounds the number of subagent dispatches, not the token spend within each subagent.` Add requirement `The workflow SHALL surface per-run cost_usd from --output-format json in the step summary.` Rewrite Scenario "max_turns reached" to `WHEN Claude Code consumes its configured max_turns budget without finishing organically THEN Claude Code SHALL terminate AND the workflow step SHALL exit zero AND the step summary SHALL record a max_turns_exhausted status with whatever findings were posted (which may be none).` Drop the unenforceable "SHALL still attempt to submit."
- **"Non-interactive Claude Code invocation" → Scenario "/code-review skill is the entry point"** (C8): replace `against the PR identified by the workflow event context` with `the prompt SHALL be the literal string "/code-review ${{ github.event.pull_request.number }}". The workflow MUST NOT rely on the skill's branch-based auto-detection.`
- **New requirement** (C2 Option B only): "Dedup contract" — `The workflow SHALL append a machine-parseable trailer of the form <!-- forgejo-mcp-review: <head_sha> --> to every posted review body. The workflow's pre-flight step SHALL parse trailers from list_pull_reviews results and skip if the current head SHA matches an existing bot review's trailer.` Plus a tasks.md entry to amend skill Step 12 templates.
- **New requirement** (C9): "Allowed tools" — `The workflow SHALL invoke Claude Code with --allowedTools restricted to the MCP tools enumerated in design D12. The list MUST NOT include any Bash(...) entries.`

## Future Work

- **C5 spike** is deferred to a separate bd issue. Path A (copy-paste) is the certain distribution; Path B (cross-repo `uses:`) is recommended-when-supported and gated on a future spike. Implementation may proceed on Path A without blocking.

## Lead Recommendation

**Status (2026-05-13):** C4 resolved by spike. C5 deferred but documentation already split into Path A + Path B. C2 decision made (Option B: lift dedup to spec + skill amendment). C1, C3, C6, C7, C8, C9 patches applied.

**Resolved before tasks.md:**

1. ✅ **C4 spike** complete. Ephemeral `GITEA_TOKEN` works for `create_pull_review` on fork PRs on Codeberg Forgejo v11.x. D2's BASE-workflow-file guarantee verified (`pull_request_target` semantics honored despite displayed `pull_request` label).
2. **C5 spike** deferred. Repos created (`goern/c5-spike-lib`, `goern/c5-spike-consumer`); tag + secrets + trigger commit not yet applied. Tracked in bd. Implementation proceeds on Path A; Path B documented as conditional.
3. ✅ **C2 decision**: Option B chosen. Spec gained "Dedup contract" requirement; skill amendment for SHA trailer is a tasks.md entry (forward dependency on `forgejo-code-review-skill` follow-up).
4. ✅ **Mechanical patches** applied for C1, C3, C6, C7, C8, C9 to proposal.md, design.md, spec.md.

**Change is now hardened enough for tasks.md** with the following caveats: the C5 spike result may demote D9 Path B from "recommended" to "experimental" if cross-repo `workflow_call` semantics turn out unreliable on Codeberg Forgejo. That demotion is doc-only and does not block implementation.

**Most embarrassing findings (recorded for future battle-tests):** C1 (`--comment` vs `--dry-run` flag inversion) and C2 (fabricated dedup tagging mechanism). Both indicate the change set was written against the dep change's design rather than the shipped skill artifact. Future battle-tests should explicitly include "code-read the shipped artifact" as a pre-flight, not just "read the dep change's design."
