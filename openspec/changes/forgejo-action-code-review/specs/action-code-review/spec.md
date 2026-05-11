## ADDED Requirements

### Requirement: Reusable Forgejo Actions workflow definition

The change SHALL ship a reusable workflow file at `.forgejo/workflows/claude-code-review.yml`. The workflow SHALL be triggered by pull-request events on the consuming repository. The workflow SHALL be opt-in: a repository that does not import or copy the file MUST observe no behavioral change.

#### Scenario: PR opened on a repository that installs the workflow

- **WHEN** a contributor opens a pull request against a repository that has copied or referenced `.forgejo/workflows/claude-code-review.yml`
- **THEN** Forgejo Actions SHALL schedule the workflow run on the PR event
- **AND** the workflow SHALL execute against the merge commit (or a safely-checked-out PR head, per the fork-handling requirement below)

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
- **THEN** the prompt passed by the workflow SHALL invoke the `/code-review` skill (provided by the `forgejo-code-review-skill` change) against the PR identified by the workflow event context

### Requirement: Findings posted as a Forgejo PR review

Review findings SHALL be posted back to the pull request as a Forgejo PR review (not as free-form issue comments). Inline comments SHALL be attached to specific files and lines using `create_pull_review` from `forgejo-mcp`. The review state SHALL be `COMMENT` by default; the workflow MAY allow callers to override this (see configurable-inputs requirement).

#### Scenario: Review submitted with inline comments

- **WHEN** Claude Code finishes with at least one finding
- **THEN** the workflow's MCP call SHALL submit a single PR review via `create_pull_review` containing all inline comments for the affected files and lines

#### Scenario: No findings

- **WHEN** Claude Code finishes with zero findings
- **THEN** the workflow SHALL either skip the review submission or submit a `COMMENT` review with a body that explicitly states no findings were produced
- **AND** the workflow SHALL exit with a success status

### Requirement: Fork-PR support via pull_request_target

The workflow SHALL support reviewing pull requests opened from forks. When using the `pull_request_target` trigger, the workflow MUST check out PR code in a safe pattern that does not allow PR-controlled files (workflow files, build scripts) to gain access to secrets.

#### Scenario: Fork PR triggers workflow with secret access

- **WHEN** a fork PR is opened against the target repository
- **THEN** the workflow SHALL be eligible for `pull_request_target` execution so it has access to `ANTHROPIC_API_KEY` and `FORGEJO_TOKEN`

#### Scenario: PR-controlled files do not influence workflow logic

- **WHEN** the workflow checks out the PR head for review
- **THEN** the checkout SHALL use a pattern that does not execute PR-controlled scripts or load PR-controlled workflow files
- **AND** the secrets SHALL not be exposed to any step that consumes PR-controlled content

### Requirement: Configurable inputs

The workflow SHALL expose the following inputs, each with a documented default: `model` (Claude model tier), `confidence_threshold` (minimum confidence for posted findings), `max_turns` (hard cap on Claude Code turns), and `additional_instructions` (free-form extra prompt content appended to the `/code-review` invocation).

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

The workflow SHALL enforce a hard upper bound on Claude Code execution through the `--max-turns` flag, set to the `max_turns` input. Reaching the cap MUST NOT cause an indefinite hang; the run SHALL terminate and the workflow SHALL post whatever findings were produced up to that point.

#### Scenario: max_turns reached

- **WHEN** Claude Code consumes its configured `max_turns` budget without finishing organically
- **THEN** Claude Code SHALL terminate
- **AND** the workflow SHALL still attempt to submit the partial review (if any findings exist) before exiting

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
