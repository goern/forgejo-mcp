---
description: Multi-agent PR code review for Forgejo repositories
argument-hint: [PR#] [--comment]
model: sonnet
allowed-tools: Read, Bash(git branch:*), Bash(git remote:*), AskUserQuestion, Task, mcp__codeberg__get_pull_request_by_index, mcp__codeberg__list_repo_pull_requests, mcp__codeberg__get_file_content, mcp__codeberg__create_pull_review, mcp__codeberg__list_pull_reviews, mcp__codeberg__list_pull_review_comments
---

# Code Review

Automated multi-agent PR review for Forgejo. Dispatches specialized agents in parallel, scores findings independently, and optionally posts inline comments to the PR.

## Step 1: Parse Arguments

Parse `$ARGUMENTS` to extract:

- **PR number**: First numeric argument (optional)
- **`--comment` flag**: If present, post findings to the Forgejo PR; otherwise terminal-only

Examples:
- `/code-review` — auto-detect PR, terminal output
- `/code-review 42` — review PR #42, terminal output
- `/code-review 42 --comment` — review PR #42, post to Forgejo
- `/code-review --comment` — auto-detect PR, post to Forgejo

## Step 2: Determine Repository Owner and Name

Run `git remote get-url origin` and parse the owner and repo name from the URL. Support both SSH (`git@codeberg.org:owner/repo.git`) and HTTPS (`https://codeberg.org/owner/repo.git`) formats.

## Step 3: Resolve PR

**If PR number was provided:**
- Call `mcp__codeberg__get_pull_request_by_index` with the owner, repo, and PR number

**If no PR number:**
1. Run `git branch --show-current` to get the current branch
2. Call `mcp__codeberg__list_repo_pull_requests` with owner and repo
3. Filter results for open PRs where the head branch matches the current branch
4. If exactly one match, use it
5. If zero matches, report "No open PR found for branch '<branch>'" and stop
6. If multiple matches, use AskUserQuestion to let the user select

## Step 4: Pre-screening

Validate the PR before running a full review. Use a **Haiku** subagent via the Task tool:

```text
Task tool with:
- subagent_type: "general-purpose"
- model: "haiku"
- prompt: "Evaluate this PR for reviewability.

  **PR metadata:**
  {paste PR title, state, draft status, changed files count, additions, deletions}

  **Rules:**
  - If PR is a draft: return {\"skip\": true, \"reason\": \"Skipping draft PR\"}
  - If PR is closed or merged: return {\"skip\": true, \"reason\": \"Skipping closed/merged PR\"}
  - If total changed lines < 5: return {\"skip\": true, \"reason\": \"Skipping trivial PR (< 5 lines changed)\"}
  - If total changed lines > 500: return {\"skip\": false, \"warn\": \"PR exceeds 500 changed lines - review may be incomplete or hit rate limits\"}
  - Otherwise: return {\"skip\": false}

  Return ONLY the JSON object, nothing else."
```

- If `skip: true`, display the reason and stop
- If `warn` is present, display the warning and use AskUserQuestion with options "Continue review" / "Cancel"
- Otherwise, proceed

## Step 5: Gather Context

1. **Read project guidelines**: Call `mcp__codeberg__get_file_content` for `CLAUDE.md` and `AGENTS.md` at the PR's base branch ref. If a file doesn't exist, skip it.

2. **Read changed files**: For each file listed in the PR's changed files, call `mcp__codeberg__get_file_content` at the PR's head branch ref to get the current version. Collect all file contents for the review agents.

3. **Get PR diff context**: Store the PR title, description, base branch, head branch, and the list of changed files with their additions/deletions counts.

## Step 6: Summarize Changes

Spawn a **Haiku** subagent to create a structured summary:

```text
Task tool with:
- subagent_type: "general-purpose"
- model: "haiku"
- prompt: "Create a structured summary of this PR's changes.

  **PR title:** {title}
  **PR description:** {description}
  **Changed files and their contents:**
  {for each file: path, additions count, deletions count, file content}

  **Output format - return ONLY this structure:**
  For each changed file:
  - File path
  - Change type: added / modified / deleted
  - Summary of what changed (1-2 sentences)
  - Key line numbers where changes occur

  Keep it factual and concise. This will be passed to review agents as context."
```

## Step 7: Parallel Review

Dispatch three review agents simultaneously using the Task tool. All three MUST be launched in a single message (parallel tool calls).

### Agent 1: Compliance (Sonnet)

```text
Task tool with:
- subagent_type: "general-purpose"
- model: "sonnet"
- prompt: "You are a compliance reviewer. Check the changed code against the project's guidelines.

  **Project guidelines (CLAUDE.md / AGENTS.md):**
  {paste guidelines content, or 'No guidelines found' if neither file exists}

  **Change summary:**
  {paste summary from Step 6}

  **Changed file contents:**
  {paste each changed file with path and content}

  **Instructions:**
  - ONLY review code that was changed in this PR - ignore pre-existing code
  - For each violation found, identify the SPECIFIC rule from the guidelines
  - If no guidelines exist, return an empty findings array

  **Output - return ONLY this JSON array:**
  [
    {
      \"file\": \"path/to/file.go\",
      \"line\": 42,
      \"rule\": \"The specific guideline rule text\",
      \"description\": \"What the code does wrong and how to fix it\"
    }
  ]

  Return [] if no violations found."
```

### Agent 2: Bug Hunter (Opus)

```text
Task tool with:
- subagent_type: "general-purpose"
- model: "opus"
- prompt: "You are a bug hunter. Analyze the changed code for defects.

  **Change summary:**
  {paste summary from Step 6}

  **Changed file contents:**
  {paste each changed file with path and content}

  **Instructions:**
  - ONLY analyze code that was changed in this PR - ignore pre-existing code
  - Look for: obvious bugs, missing error handling, off-by-one errors, nil/null dereferences, resource leaks, edge cases
  - Do NOT flag style issues, naming conventions, or missing comments
  - Do NOT flag issues that linters would catch (unused imports, formatting)

  **Output - return ONLY this JSON array:**
  [
    {
      \"file\": \"path/to/file.go\",
      \"line\": 42,
      \"description\": \"Clear description of the bug and its impact\"
    }
  ]

  Return [] if no bugs found."
```

### Agent 3: Logic/Security (Opus)

```text
Task tool with:
- subagent_type: "general-purpose"
- model: "opus"
- prompt: "You are a logic and security reviewer. Analyze the changed code for deeper issues.

  **Change summary:**
  {paste summary from Step 6}

  **Changed file contents:**
  {paste each changed file with path and content}

  **Instructions:**
  - ONLY analyze code that was changed in this PR - ignore pre-existing code
  - Look for: logic errors, security vulnerabilities (injection, XSS, auth bypass), race conditions, data validation gaps, unsafe deserialization, hardcoded secrets
  - Assess severity for each finding: critical / high / medium / low

  **Output - return ONLY this JSON array:**
  [
    {
      \"file\": \"path/to/file.go\",
      \"line\": 42,
      \"severity\": \"high\",
      \"description\": \"Clear description of the issue, its risk, and suggested fix\"
    }
  ]

  Return [] if no issues found."
```

## Step 8: Confidence Scoring and Filtering

After all three agents return, collect all findings into a single list. Then spawn a scoring agent:

```text
Task tool with:
- subagent_type: "general-purpose"
- model: "sonnet"
- prompt: "You are an independent confidence scorer for code review findings.

  **PR changed files:** {list of file paths that were changed}

  **All findings from review agents:**
  {paste combined findings JSON array, each tagged with source: compliance/bug/logic}

  **Scoring rules:**
  - Score each finding from 0 to 100
  - Score 0 if the finding refers to code NOT changed in this PR (pre-existing issue)
  - Score 0 if the finding describes something linters/formatters catch (unused imports, formatting, trailing whitespace)
  - Score 0 if the finding is a style nitpick or personal preference
  - Score 25 if the finding is vague or speculative without concrete evidence
  - Score 50-75 for real but minor issues
  - Score 80+ only for findings with clear evidence pointing to a real problem
  - Score 100 for definite bugs or security vulnerabilities with proof

  **Output - return ONLY this JSON array (same findings with added score):**
  [
    {
      \"file\": \"...\",
      \"line\": ...,
      \"source\": \"compliance|bug|logic\",
      \"severity\": \"...\",
      \"description\": \"...\",
      \"confidence\": 85
    }
  ]"
```

**Filter**: Remove all findings with `confidence` < 80.

## Step 9: Output Results

### Terminal Output (always shown)

If findings remain after filtering:

```markdown
## Code Review: PR #<number> - <title>

### <file-path>

**[<source>] Line <line>** (confidence: <score>)
<description>

### <next-file-path>
...

---
Summary: <N> issue(s) found, <M> filtered out (below confidence threshold)
```

If no findings remain:

```markdown
## Code Review: PR #<number> - <title>

No significant issues found. (<M> findings filtered out below confidence threshold)
```

### Post to Forgejo (only if `--comment` flag was provided)

**If findings exist above threshold:**

Call `mcp__codeberg__create_pull_review` with:
- `owner`: repository owner
- `repo`: repository name
- `index`: PR number
- `state`: `COMMENT`
- `body`: Summary line, e.g., "Automated review found N issue(s) (M filtered)"
- `comments`: JSON array where each finding becomes:
  `{"path": "<file>", "body": "[<source>] <description> (confidence: <score>)", "new_position": <line>}`

**If no findings above threshold:**

Call `mcp__codeberg__create_pull_review` with:
- `owner`: repository owner
- `repo`: repository name
- `index`: PR number
- `state`: `COMMENT`
- `body`: "No significant issues found"
