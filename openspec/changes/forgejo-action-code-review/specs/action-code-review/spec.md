## ADDED Requirements

### Requirement: Reusable Forgejo Actions workflow definition

The change SHALL ship a reusable workflow file at `.forgejo/workflows/claude-code-review.yml`. The workflow SHALL be triggered by pull-request events on the consuming repository. The workflow SHALL be opt-in: a repository that does not import or copy the file MUST observe no behavioral change. Two distribution paths SHALL be supported, in order of certainty: (A) copy-paste of the workflow file into the consuming repository, and (B) cross-repo `uses:` reference (gated on the target Forgejo version's `workflow_call` support, pending C5 feasibility spike).

#### Scenario: PR opened on a repository that copies the workflow file

- **WHEN** a contributor opens a pull request against a repository that has copied `.forgejo/workflows/claude-code-review.yml` verbatim into its own `.forgejo/workflows/` directory
- **THEN** Forgejo Actions SHALL schedule the workflow run on the PR event
- **AND** the workflow SHALL execute against the base branch (per the "no PR-head checkout" requirement)

#### Scenario: PR opened on a repository that references the workflow via `uses:`

- **WHEN** a contributor opens a pull request against a repository whose `.forgejo/workflows/` contains a caller workflow with `jobs.<id>.uses: goern/forgejo-mcp/.forgejo/workflows/claude-code-review.yml@<tag>`
- **AND** the target Forgejo version supports cross-repo `workflow_call`
- **THEN** Forgejo Actions SHALL resolve the referenced workflow at the pinned tag, propagate inherited secrets, and execute it
- **NOTE** This scenario is gated on the C5 feasibility spike. If the target Forgejo version does not support cross-repo `workflow_call`, the copy-paste scenario is the only supported path.

#### Scenario: PR opened on a repository without the workflow

- **WHEN** a contributor opens a pull request against a repository that has not installed the workflow
- **THEN** no Claude-driven review SHALL be posted
- **AND** no resource SHALL be consumed by this change

### Requirement: Environment setup for Claude Code with forgejo-mcp

The workflow SHALL install Claude Code on the runner and SHALL register `forgejo-mcp` as an MCP server before invoking the review. The MCP server SHALL be configured with the runner's `FORGEJO_TOKEN` and the target instance URL so that the in-session `forgejo-mcp` tools can reach the target Forgejo instance.

#### Scenario: Claude Code installed at the start of the run

- **WHEN** the workflow begins on a runner without Claude Code pre-installed
- **THEN** a step SHALL install Claude Code (e.g. via `npm install -g @anthropic-ai/claude-code` or equivalent) before any review step runs

#### Scenario: forgejo-mcp registered before review invocation

- **WHEN** the install step completes
- **THEN** a configuration step SHALL register `forgejo-mcp` as an MCP server for Claude Code
- **AND** the registration SHALL pass through `FORGEJO_URL` and `FORGEJO_TOKEN` from the runner's secret context

### Requirement: Non-interactive Claude Code invocation

The workflow SHALL invoke Claude Code in non-interactive mode (`claude -p`, or equivalent flag the CLI provides) with a prompt that triggers the `/code-review` skill against the current pull request. Claude Code MUST NOT prompt the user — the run SHALL complete autonomously and return a final state to the workflow.

#### Scenario: Review runs without interactive prompts

- **WHEN** Claude Code is invoked by the workflow
- **THEN** Claude Code SHALL run in `-p` (non-interactive) mode
- **AND** the run SHALL terminate without waiting for stdin

#### Scenario: /code-review skill is the entry point

- **WHEN** Claude Code starts in non-interactive mode
- **THEN** the prompt passed by the workflow SHALL be the literal string `/code-review ${{ github.event.pull_request.number }}` (the PR number from the workflow event context, passed as the first positional argument)
- **AND** the workflow MUST NOT pass `--dry-run` (the skill posts by default; `--dry-run` would suppress posting)
- **AND** the workflow MUST NOT rely on the skill's branch-based auto-detection, because the base-only checkout makes branch-based detection unreliable

### Requirement: Findings posted as a Forgejo PR review

Review findings SHALL be posted back to the pull request as a Forgejo PR review (not as free-form issue comments). Inline comments SHALL be attached to specific files and lines using `create_pull_review` from `forgejo-mcp`. The review state SHALL be `COMMENT` by default; the workflow MAY allow callers to override this (see configurable-inputs requirement). The workflow MUST invoke the skill without the `--dry-run` flag — the skill posts by default, and the action's purpose is to post.

#### Scenario: Review submitted with inline comments

- **WHEN** Claude Code finishes with at least one finding
- **THEN** the workflow's MCP call SHALL submit a single PR review via `create_pull_review` containing all inline comments for the affected files and lines

#### Scenario: No findings

- **WHEN** Claude Code finishes with zero findings
- **THEN** the workflow SHALL either skip the review submission or submit a `COMMENT` review with a body that explicitly states no findings were produced
- **AND** the workflow SHALL exit with a success status

### Requirement: Fork-PR support via pull_request_target

The workflow SHALL support reviewing pull requests opened from forks. When using the `pull_request_target` trigger, the workflow MUST NOT check out PR-head code onto the runner (see "Base-only checkout" requirement). PR contents are read via `forgejo-mcp` API tools, which never load PR-controlled files into runner filesystem or execution context.

#### Scenario: Fork PR triggers workflow with secret access

- **WHEN** a fork PR is opened against the target repository
- **THEN** the workflow SHALL be eligible for `pull_request_target` execution so it has access to `ANTHROPIC_API_KEY` and `FORGEJO_TOKEN`
- **NOTE** C4 feasibility spike (2026-05-13) confirmed that `${{ secrets.GITEA_TOKEN }}` is honored for fork PRs on Codeberg Forgejo v11.x. PAT override is documented for consumers on other Forgejo versions or those wanting human-attributed bot identity.

#### Scenario: PR-controlled files do not reach the runner

- **WHEN** the workflow runs for any PR (same-repo or fork)
- **THEN** no `actions/checkout` step SHALL fetch the PR-head ref
- **AND** no step SHALL execute PR-controlled scripts (build scripts, postinstall hooks, lifecycle scripts)
- **AND** the secrets SHALL be exposed only to steps that consume API-mediated PR content (via `forgejo-mcp`) or workflow-author-controlled content

### Requirement: Configurable inputs

The workflow SHALL expose the following inputs, each with a documented default: `model` (Claude model tier), `confidence_threshold` (minimum confidence for posted findings), `max_turns` (soft cap on Claude Code main-loop turns; recommended floor `25`), `additional_instructions` (free-form extra prompt content appended to the `/code-review` invocation; MUST come from workflow-author-controlled sources only), and `forgejo_mcp_version` (the forgejo-mcp release tag to install).

#### Scenario: Caller overrides the model tier

- **WHEN** a consuming repository sets `model: claude-haiku-4-5` in its workflow inputs
- **THEN** Claude Code SHALL run with that model for this review

#### Scenario: Caller raises max_turns

- **WHEN** a consuming repository sets `max_turns: 30`
- **THEN** Claude Code SHALL be invoked with `--max-turns 30`

#### Scenario: Defaults documented

- **WHEN** a maintainer reads the workflow file or its README
- **THEN** each input's default value SHALL be visible in the file (`default:` field) and documented in the accompanying setup notes

### Requirement: Cost cap via max-turns

The workflow SHALL impose a soft upper bound on Claude Code's main-loop turn count via the `--max-turns` flag, set to the `max_turns` input. The cap bounds the number of subagent dispatches, not the token spend within each subagent. The workflow SHALL surface per-run `cost_usd` from `--output-format json` in the step summary, so consumers can monitor empirical cost. Reaching the cap MUST NOT cause an indefinite hang; Claude Code SHALL terminate.

#### Scenario: max_turns reached

- **WHEN** Claude Code consumes its configured `max_turns` budget without finishing organically
- **THEN** Claude Code SHALL terminate
- **AND** the workflow step SHALL exit zero
- **AND** the step summary SHALL record a `max_turns_exhausted` status with whatever findings were posted (which may be none, because the skill posts at the final pipeline step which may not have been reached)

#### Scenario: cost_usd surfaced in step summary

- **WHEN** Claude Code finishes (organically or via `max_turns`)
- **THEN** the workflow SHALL parse the final JSON result block and write `cost_usd`, model, `num_turns`, and findings-posted count to `$GITHUB_STEP_SUMMARY` (or the Forgejo Actions equivalent step summary file)

### Requirement: Required secrets

The workflow SHALL document and require two repository secrets: `ANTHROPIC_API_KEY` (for Claude Code) and `FORGEJO_TOKEN` (for `forgejo-mcp` to read the PR and post the review). The `FORGEJO_TOKEN` SHALL be a scoped token with no permissions beyond PR read and PR-review write on the target repository.

#### Scenario: Missing ANTHROPIC_API_KEY

- **WHEN** the workflow runs without `ANTHROPIC_API_KEY` set
- **THEN** the workflow SHALL fail fast with a clear error before invoking Claude Code

#### Scenario: Missing FORGEJO_TOKEN

- **WHEN** the workflow runs without `FORGEJO_TOKEN` set
- **THEN** the workflow SHALL fail fast with a clear error before invoking Claude Code

#### Scenario: Token scope documented

- **WHEN** a maintainer reads the setup documentation shipped with the workflow
- **THEN** the recommended `FORGEJO_TOKEN` scope SHALL be limited to PR read + PR-review write on the target repository

#### Scenario: FORGEJO_TOKEN broader than required

- **WHEN** the workflow runs with a `FORGEJO_TOKEN` whose scope exceeds PR read + PR-review write on the target repository (e.g., org-wide scope, contents-write, admin)
- **THEN** the workflow SHOULD detect this in preflight and SHALL emit a warning in the step summary recommending scope tightening
- **NOTE** Detection is best-effort because Forgejo's PAT scope-introspection endpoint is not universally available; the documentation MUST explicitly warn against over-scoped tokens regardless of detection.

### Requirement: Dedup contract via SHA trailer

The workflow SHALL append a machine-parseable trailer of the form `<!-- forgejo-mcp-review: <head_sha> -->` to every posted review body. The workflow's pre-flight step SHALL parse trailers from `list_pull_reviews` results and skip the Claude invocation if the current head SHA matches an existing bot review's trailer.

Because the shipped `/code-review` skill does not currently emit this trailer, a tasks.md entry SHALL amend skill Step 12 body templates to append the trailer when the action passes `--head-sha <SHA>` (or equivalent argument). Until the skill amendment lands, the workflow MAY pass the trailer template via `additional_instructions` as a bridge.

#### Scenario: Skip when current head SHA already reviewed

- **WHEN** the pre-flight step calls `list_pull_reviews` for the PR
- **AND** any review body contains the literal trailer `<!-- forgejo-mcp-review: <current_head_sha> -->`
- **THEN** the workflow SHALL exit zero with a notice "skipped, already reviewed at <head_sha>"
- **AND** Claude Code SHALL NOT be invoked

#### Scenario: Post with trailer on new review

- **WHEN** the skill posts a review via `create_pull_review`
- **THEN** the review body SHALL contain the trailer `<!-- forgejo-mcp-review: <head_sha> -->` (either via the skill amendment or via the `additional_instructions` bridge)

### Requirement: Allowed tools restriction

The workflow SHALL invoke Claude Code with `--allowedTools` restricted to the MCP tools enumerated in design decision D12. The list MUST NOT include any `Bash(...)` entries, MUST NOT include `Read`, and MUST NOT include `AskUserQuestion`.

#### Scenario: Bash tools rejected

- **WHEN** a maintainer modifies the workflow to add `Bash(...)` to `--allowedTools`
- **THEN** a code-review CI check on this repository SHALL fail with a message referencing D12

#### Scenario: Allowed tools minimal

- **WHEN** the workflow invokes Claude Code
- **THEN** `--allowedTools` SHALL contain exactly the MCP tool identifiers needed for PR review (`get_pull_request_by_index`, `list_pull_request_files`, `get_pull_request_diff`, `get_file_content`, `list_pull_reviews`, `create_pull_review`) plus `Task` for subagent dispatch
- **AND** no other tool identifiers SHALL be included

### Requirement: Base-only checkout, no PR-head execution

The workflow SHALL check out the base branch only. The workflow MUST NOT check out the PR head ref. PR contents (diff, files, metadata) SHALL be read exclusively through `forgejo-mcp` tools, which access the Forgejo API and never load PR-controlled files onto the runner filesystem.

#### Scenario: actions/checkout uses base ref

- **WHEN** the workflow runs an `actions/checkout` step
- **THEN** the `ref` SHALL be the base ref (e.g. `${{ github.event.pull_request.base.ref }}`) or omitted (default branch)
- **AND** the `ref` MUST NOT be `${{ github.event.pull_request.head.sha }}` or `${{ github.event.pull_request.head.ref }}` or any expression that resolves to a fork-controlled ref

#### Scenario: PR content read via MCP

- **WHEN** the skill needs PR diff or file contents
- **THEN** it SHALL call `mcp__codeberg__get_pull_request_diff` or `mcp__codeberg__get_file_content` (at the PR head SHA passed via tool arguments), never read from the runner filesystem
