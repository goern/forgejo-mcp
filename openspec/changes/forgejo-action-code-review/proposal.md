# Forgejo Action for Claude Code Review

## Why

The `/code-review` skill (from the `forgejo-code-review-skill` change) works interactively in Claude Code. But the real value for teams is **automated review on every PR** — triggered by Forgejo Actions, posting findings directly as PR review comments. GitHub has `anthropics/claude-code-action` for this; Forgejo has nothing. This action bridges that gap, making AI-assisted code review available to any Forgejo instance (Codeberg, self-hosted).

## What Changes

- Add a reusable Forgejo Actions workflow (`.forgejo/workflows/claude-code-review.yml`) that runs Claude Code with forgejo-mcp on PR events
- The workflow installs Claude Code, configures forgejo-mcp as an MCP server, and invokes `/code-review <PR#>` in non-interactive (`-p`) mode (skill posts by default; `--dry-run` is not passed)
- Findings are posted as a Forgejo PR review with inline comments via `create_pull_review`
- Supports `pull_request_target` trigger for fork PRs (secrets access) with safe checkout patterns
- Configurable via workflow inputs: model tier, confidence threshold, max turns, additional review instructions
- Invokes Claude Code with `--allowedTools` restricted to forgejo-mcp PR review tools only (no Bash, no Read) — see design D12

## Capabilities

### New Capabilities

- `action-code-review`: The Forgejo Actions workflow definition, environment setup (Claude Code installation, MCP server configuration, secret management), base-only checkout (PR contents read via forgejo-mcp API, never executed on runner), and integration between Claude Code's `-p` mode and forgejo-mcp's review tools. Covers trigger configuration, runner requirements, cost controls (`--max-turns` as soft cap, `--allowedTools` restricted to MCP review tools per design D12), `cost_usd` surfacing, dedup via SHA trailer, and output handling.

### Modified Capabilities

<!-- None. This consumes the code-review skill and existing MCP tools without changing them. -->

## Impact

- **New files**: `.forgejo/workflows/claude-code-review.yml` (the action workflow), documentation for setup
- **Dependencies**: Requires the `/code-review` skill from `forgejo-code-review-skill` change. Requires a Forgejo runner with Node.js (for Claude Code installation). Requires `ANTHROPIC_API_KEY` and `FORGEJO_TOKEN` as repository secrets.
- **Security considerations**: Uses `pull_request_target` for fork PR support — must checkout PR code carefully to avoid code injection. The workflow runs forgejo-mcp with a scoped token (only PR review permissions needed).
- **Cost implications**: Each PR review run costs ~$0.10-$5.00 depending on diff size, model tier, and subagent dispatch fan-out. `--max-turns` caps main-loop turns (soft cost cap); per-run `cost_usd` is surfaced in the step summary for monitoring. Large PRs scale cost super-linearly with subagent count.
- **No breaking changes**: Purely additive. The workflow is opt-in per repository.
- **Dependency on other change**: Depends on `forgejo-code-review-skill` being completed first (the skill prompt is what Claude Code executes)
