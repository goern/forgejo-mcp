## Why

There is no Claude Code skill for performing automated code reviews on Forgejo pull requests. The GitHub ecosystem has a well-designed [code review plugin](https://github.com/anthropics/claude-code/blob/main/plugins/code-review/README.md) that uses multi-agent pipelines with confidence scoring to surface high-signal issues. Forgejo users have no equivalent. Building this skill is critical for making Claude Code viable on open-source Git platforms — reducing a key reason teams stay on GitHub.

The forgejo-mcp server already exposes all necessary PR review tools (`create_pull_review` with inline comments, `get_pull_request_by_index`, `get_file_content`, `list_pull_reviews`). The gap is a **skill layer** that orchestrates these tools into a review workflow.

## What Changes

- Add a new `/code-review` slash command (Claude Code skill) that performs automated PR review on Forgejo repositories
- The skill uses a multi-agent pipeline: pre-screening, diff analysis, parallel review (CLAUDE.md compliance + bug detection + logic/security), independent confidence scoring, and false-positive filtering
- Reviews are posted as Forgejo PR reviews with inline comments on specific code lines via `create_pull_review`
- Supports `--comment` flag to post findings to the PR (vs. terminal-only output)
- Accepts PR number as argument, or auto-detects from current branch

## Capabilities

### New Capabilities
- `code-review-skill`: The Claude Code slash command definition (`/.claude/commands/code-review.md`) that orchestrates multi-agent PR review using forgejo-mcp tools. Covers the prompt pipeline, agent model selection, confidence scoring threshold, output formatting, and posting behavior.

### Modified Capabilities
<!-- No existing spec requirements change. The skill consumes existing MCP tools as-is. -->

## Impact

- **New files**: `.claude/commands/code-review.md` (the skill prompt file)
- **Dependencies**: Requires forgejo-mcp running as an MCP server with PR review tools available. No new Go code or SDK changes needed.
- **Existing tools used**: `get_pull_request_by_index`, `get_file_content`, `list_pull_reviews`, `create_pull_review` (with inline comments), `list_repo_issues`
- **Agent model usage**: Haiku for pre-screening, Sonnet for summarization/compliance, Opus for bug/logic review — mirrors the GitHub plugin's tiered approach
- **No breaking changes**: This is purely additive
