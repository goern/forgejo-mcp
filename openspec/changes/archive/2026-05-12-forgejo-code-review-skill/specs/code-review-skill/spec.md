# Code Review Skill

## ADDED Requirements

### Requirement: Skill invocation via slash command

The system SHALL provide a `/code-review` Claude Code command defined in `.claude/commands/code-review.md`. The command SHALL accept an optional PR number as its first argument and an optional `--comment` flag.

#### Scenario: Invoke with explicit PR number

- **WHEN** user runs `/code-review 42`
- **THEN** the skill SHALL review pull request #42 in the current repository

#### Scenario: Invoke with auto-detection

- **WHEN** user runs `/code-review` without a PR number
- **THEN** the skill SHALL determine the current branch via `git branch --show-current`, query `list_repo_pull_requests` for open PRs matching that head branch, and use the matching PR
- **AND** if zero or multiple PRs match, the skill SHALL prompt the user to select one

#### Scenario: Invoke with comment flag

- **WHEN** user runs `/code-review 42 --comment`
- **THEN** the skill SHALL post review findings to the Forgejo PR via `create_pull_review`
- **AND** without `--comment`, findings SHALL be displayed in the terminal only

### Requirement: PR pre-screening

The system SHALL validate the target PR before performing a full review. Pre-screening SHALL use the Haiku model.

#### Scenario: Skip draft PR

- **WHEN** the target PR has draft status
- **THEN** the skill SHALL report "Skipping draft PR" and exit without reviewing

#### Scenario: Skip closed PR

- **WHEN** the target PR is closed or merged
- **THEN** the skill SHALL report "Skipping closed/merged PR" and exit without reviewing

#### Scenario: Skip trivial PR

- **WHEN** the PR diff contains fewer than 5 changed lines
- **THEN** the skill SHALL report "Skipping trivial PR (< 5 lines changed)" and exit

#### Scenario: Skip oversized PR

- **WHEN** the PR diff contains more than 500 changed lines
- **THEN** the skill SHALL warn "PR exceeds 500 changed lines — review may be incomplete or hit rate limits" and ask the user whether to proceed

### Requirement: Context gathering

The system SHALL gather project context before review agents run. Context includes CLAUDE.md/AGENTS.md content, PR metadata, and the full diff.

#### Scenario: Gather CLAUDE.md guidelines

- **WHEN** the repository contains a `CLAUDE.md` or `AGENTS.md` file
- **THEN** the skill SHALL read those files via `get_file_content` and provide their content to the compliance review agent

#### Scenario: Gather PR metadata and diff

- **WHEN** pre-screening passes
- **THEN** the skill SHALL retrieve PR details via `get_pull_request_by_index` and read changed file contents via `get_file_content` for each file in the diff

### Requirement: Change summarization

The system SHALL produce a structured summary of the PR changes before dispatching to review agents. Summarization SHALL use the Haiku model.

#### Scenario: Summarize changes with line references

- **WHEN** context gathering completes
- **THEN** the skill SHALL produce a summary that lists each changed file, the nature of the change (added/modified/deleted), and line number ranges for each hunk

### Requirement: Parallel multi-agent review

The system SHALL dispatch three specialized review agents in parallel using the Task tool. Each agent SHALL operate independently with no shared state.

#### Scenario: Compliance agent reviews guideline adherence

- **WHEN** parallel review begins
- **THEN** a Sonnet-model agent SHALL check the changed code against rules in CLAUDE.md and AGENTS.md
- **AND** each finding SHALL reference the specific rule violated and the file path + line number

#### Scenario: Bug hunter agent finds defects

- **WHEN** parallel review begins
- **THEN** an Opus-model agent SHALL analyze changed code only for obvious bugs, missing error handling, off-by-one errors, nil/null dereferences, and edge cases
- **AND** each finding SHALL include the file path, line number, and a description of the defect

#### Scenario: Logic/security agent finds deeper issues

- **WHEN** parallel review begins
- **THEN** an Opus-model agent SHALL analyze changed code for logic errors, security vulnerabilities, race conditions, and data validation issues
- **AND** each finding SHALL include the file path, line number, severity, and a description

### Requirement: Confidence scoring and filtering

The system SHALL independently score each finding on a 0-100 confidence scale and filter out findings below the threshold of 80.

#### Scenario: Score and filter findings

- **WHEN** all three review agents return their findings
- **THEN** a scoring pass SHALL assign each finding a confidence score from 0-100
- **AND** findings with score below 80 SHALL be removed from the output

#### Scenario: Filter pre-existing issues

- **WHEN** a finding refers to code that was not changed in the PR
- **THEN** the finding SHALL receive a confidence score of 0

#### Scenario: Filter linter-catchable issues

- **WHEN** a finding describes a problem that standard linters or formatters would catch (unused imports, formatting, trailing whitespace)
- **THEN** the finding SHALL receive a confidence score of 0

### Requirement: Terminal output formatting

The system SHALL present filtered findings in a structured terminal format.

#### Scenario: Display findings in terminal

- **WHEN** the review completes with findings above threshold
- **THEN** the skill SHALL display each finding grouped by file, showing: file path, line number, severity, agent source (compliance/bug/logic), confidence score, and description

#### Scenario: Display clean result

- **WHEN** the review completes with no findings above threshold
- **THEN** the skill SHALL display "No significant issues found" with a count of how many findings were filtered out

### Requirement: Post review to Forgejo

The system SHALL post findings as a Forgejo PR review with inline comments when the `--comment` flag is provided.

#### Scenario: Post inline comments

- **WHEN** `--comment` is specified and findings exist above threshold
- **THEN** the skill SHALL call `create_pull_review` with state `COMMENT`, a summary body, and a comments array where each entry has `path`, `body`, and `new_position` fields

#### Scenario: Post clean review

- **WHEN** `--comment` is specified and no findings exist above threshold
- **THEN** the skill SHALL call `create_pull_review` with state `COMMENT` and body "No significant issues found"
