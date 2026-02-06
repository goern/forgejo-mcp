# Tasks

## 1. Command File Scaffold

- [x] 1.1 Create `.claude/commands/code-review.md` with frontmatter (description, argument-hint, model, allowed-tools) matching existing command patterns (`/commit`, `/catchup`)
- [x] 1.2 Write argument parsing section: extract optional PR number and `--comment` flag from `$ARGUMENTS`

## 2. PR Resolution and Pre-screening

- [x] 2.1 Implement PR auto-detection: `git branch --show-current` → `list_repo_pull_requests` lookup → prompt if ambiguous
- [x] 2.2 Implement explicit PR number path: parse number from arguments, call `get_pull_request_by_index`
- [x] 2.3 Add pre-screening gate: skip draft, closed/merged, trivial (<5 lines), and warn on oversized (>500 lines) PRs

## 3. Context Gathering

- [x] 3.1 Read `CLAUDE.md` and `AGENTS.md` from the repository via `get_file_content` (handle missing files gracefully)
- [x] 3.2 Retrieve PR metadata via `get_pull_request_by_index` and read each changed file's content via `get_file_content`

## 4. Change Summarization

- [x] 4.1 Write the Haiku-model summarization prompt: produce structured summary with file paths, change types (added/modified/deleted), and line number ranges per hunk

## 5. Parallel Review Agents

- [x] 5.1 Write compliance agent prompt (Sonnet): check changed code against CLAUDE.md/AGENTS.md rules, output findings as JSON with `{file, line, rule, description}`
- [x] 5.2 Write bug hunter agent prompt (Opus): analyze changed code for bugs, error handling gaps, edge cases, output findings as JSON with `{file, line, description}`
- [x] 5.3 Write logic/security agent prompt (Opus): analyze changed code for logic errors, security issues, race conditions, output findings as JSON with `{file, line, severity, description}`
- [x] 5.4 Wire all three agents as parallel Task tool calls with `run_in_background: true`

## 6. Confidence Scoring and Filtering

- [x] 6.1 Write the scoring prompt: independently score each finding 0-100, auto-zero pre-existing issues and linter-catchable problems
- [x] 6.2 Add threshold filter: remove findings with confidence < 80

## 7. Output Formatting

- [x] 7.1 Write terminal output format: group findings by file, show path, line, severity, agent source, confidence, and description
- [x] 7.2 Write clean-result output: "No significant issues found" with filtered count

## 8. Forgejo Posting

- [x] 8.1 Implement `--comment` path: build `create_pull_review` call with state `COMMENT`, summary body, and inline comments array (`path`, `body`, `new_position`)
- [x] 8.2 Implement clean-review posting: call `create_pull_review` with "No significant issues found" body when no findings pass threshold

## 9. Integration and Testing

- [ ] 9.1 End-to-end manual test: run `/code-review` against a real PR in terminal-only mode
- [ ] 9.2 End-to-end manual test: run `/code-review --comment` against a test PR and verify inline comments appear on Forgejo
