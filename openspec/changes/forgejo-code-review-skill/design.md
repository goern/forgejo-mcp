# Design

## Context

The forgejo-mcp project provides MCP tools for interacting with Forgejo instances. Claude Code skills (`.claude/commands/*.md`) orchestrate these tools into higher-level workflows. Existing skills (`/commit`, `/catchup`) demonstrate the pattern: frontmatter declares model and allowed tools, the body describes a multi-step workflow Claude follows.

The Forgejo MCP server already exposes all necessary PR review primitives: `get_pull_request_by_index`, `get_file_content`, `list_pull_reviews`, `create_pull_review` (with inline comment support via JSON array of `{path, body, new_position}`). No new Go code is needed.

The [Anthropic GitHub code review plugin](https://github.com/anthropics/claude-code/blob/main/plugins/code-review/README.md) provides a proven reference architecture: prerequisite validation, context gathering, change summarization, parallel multi-agent review, and confidence-based filtering. This design adapts that pipeline for Forgejo's API surface and the Claude Code skill format.

## Goals / Non-Goals

**Goals:**

- Single `/code-review [PR#] [--comment]` command that performs a full PR review
- Multi-agent pipeline with specialized reviewers running in parallel
- Confidence scoring (0-100) with threshold filtering to suppress false positives
- Inline comments on specific diff lines via `create_pull_review`
- Auto-detection of PR number from current branch when omitted
- Terminal-only output by default; `--comment` flag to post to Forgejo

**Non-Goals:**

- No new Go code, SDK changes, or MCP tool additions
- No CI/CD integration (that's the separate `forgejo-action-code-review` change)
- No review of draft PRs or closed PRs
- No custom threshold configuration (hardcoded at 80 initially)
- No incremental/re-review of already-reviewed commits

## Decisions

### 1. Skill format: Claude Code command file

**Choice:** `.claude/commands/code-review.md`

Matches existing project patterns (`/commit`, `/catchup`). Commands are the standard extensibility mechanism — no plugin framework, no external dependencies. Users invoke with `/code-review`.

**Alternatives considered:**

- Standalone script calling Claude API directly — requires API key management, loses MCP tool access
- MCP tool in Go — overengineered; the orchestration logic is prompt-level, not SDK-level

### 2. Pipeline stages: 5-stage sequential-then-parallel

```text
1. Validate PR (skip draft/closed/trivial)
2. Gather context (CLAUDE.md, PR metadata, diff)
3. Summarize changes (single agent)
4. Parallel review (3 specialized agents)
5. Filter + format + optionally post
```

Mirrors the proven GitHub plugin architecture. The sequential prefix (validate → gather → summarize) builds shared context that parallel reviewers consume. Parallel review maximizes coverage without groupthink.

**Alternatives considered:**

- Single-agent review — misses the specialization benefit
- Serial review chain — slower, and later agents are biased by earlier findings

### 3. Three parallel reviewers (not four)

| Agent          | Model  | Focus                                                           |
|----------------|--------|-----------------------------------------------------------------|
| Compliance     | Sonnet | CLAUDE.md / AGENTS.md guideline violations                      |
| Bug Hunter     | Opus   | Obvious bugs, error handling, edge cases in changed code only   |
| Logic/Security | Opus   | Logic errors, security issues, race conditions                  |

The GitHub plugin uses 4 agents (2 compliance + 1 bug + 1 historical). We drop the second compliance agent (diminishing returns) and the historical context agent (Forgejo MCP lacks `git blame` tooling). Three agents balance coverage vs. cost.

**Alternatives considered:**

- Four agents matching GitHub plugin — historical context agent needs `git log`/`git blame` tools not exposed via MCP
- Two agents — insufficient specialization

### 4. Model tiering

Haiku for pre-screening/summarization (cheap, fast). Sonnet for compliance checking (pattern-matching against rules). Opus for bug and logic review (deep reasoning).

### 5. Confidence scoring: independent scorer, threshold at 80

Each finding gets an independent confidence score (0-100). Findings below 80 are filtered out.

**Scale:** 0 = false positive, 25 = possibly real, 50 = moderate, 75 = confident, 100 = certain.

Independent scoring prevents reviewers from self-inflating confidence. The 80 threshold matches the GitHub plugin and filters stylistic nitpicks, pre-existing issues, and linter-catchable problems.

### 6. Output: terminal-only by default, `--comment` to post

Posting reviews has side effects (notifications, PR state changes). Default to safe terminal output. `--comment` opts in to posting via `create_pull_review` with state `COMMENT` and inline comment array.

### 7. PR auto-detection from current branch

When no PR number argument is given:

1. Get current branch via `git branch --show-current`
2. List open PRs via `list_repo_pull_requests` filtered by head branch
3. If exactly one match, use it. If zero or multiple, ask user.

## Risks / Trade-offs

**[Cost]** Three parallel Opus/Sonnet agents per review is expensive.
→ Pre-screening with Haiku skips trivial PRs early. Tiered models assign cheap models to cheap tasks.

**[Line number accuracy]** Mapping findings to `new_position` in the review comment API requires correct diff line numbering.
→ The summarization stage provides structured diff with line numbers that agents reference directly.

**[Rate limits]** Multiple `get_file_content` calls could hit Forgejo API limits on large PRs.
→ Pre-screening skips PRs with >500 changed lines with a warning.

**[CLAUDE.md false positives]** Compliance agent may flag rules irrelevant to changed code.
→ Confidence scorer checks each compliance finding references a specific, relevant rule.

**[Duplicate reviews]** Re-running on the same PR posts duplicate comments.
→ Documented non-goal. Future work could check `list_pull_reviews` for existing reviews.
