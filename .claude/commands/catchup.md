---
description: Review last commit and branch changes
argument-hint: [commit-ref]
model: haiku
allowed-tools: Bash(git status:*), Bash(git log:*), Bash(git show:*), Bash(git diff:*), Bash(git branch:*), Read
---

# Catchup Command

Smart review of recent changes: reviews working tree changes if present, otherwise reviews the last commit. Can also review specific commits when provided.

## Purpose

This command helps bring Claude up to speed after a session restart (`/clear`) by:

- **Without arguments**:
  - If working tree has changes → Review current working changes (modified, staged, and untracked files)
  - If working tree is clean → Review the last commit (HEAD)
- **With commit-ref**: Analyze the specified commit

## Arguments

- `commit-ref` (optional) - Git commit reference to analyze
  - If omitted: Check working tree status and review accordingly
  - If provided: Analyze the specified commit
  - Examples: `HEAD`, `HEAD~1`, `abc123`, `main~2`

## Workflow

### Step 1: Determine what to review

1. Run `git status --porcelain` to check working tree status
2. If no argument provided ($1 is empty):
   - If output is empty (clean working tree) → Set target to `HEAD`
   - If output has changes → Review working changes only
3. If argument provided → Use $1 as target

### Step 2: Review target commit (if applicable)

- If reviewing a commit (HEAD or specific ref):
  1. Use `git log -1 --stat <ref>` to see commit message and files changed
  2. Use `git show <ref>` to see the actual changes (diff)

### Step 3: Review working changes (if applicable)

- If working tree has changes:
  1. Parse `git status --porcelain` output
  2. For modified/staged files (`M`, `M`, `MM` prefix): Use `git diff HEAD` to see changes
  3. For untracked files (`??` prefix): Read each file to understand new additions

### Step 4: Provide summary

- Summarize what was found based on what was reviewed

## Usage

Run this command after:

- Starting a new session
- Using `/clear` to reset context
- Switching to a different branch with existing work

### Examples

- `/catchup` - Smart review: working changes if present, otherwise last commit
- `/catchup HEAD` - Review last commit only
- `/catchup HEAD~1` - Review previous commit only
- `/catchup abc123` - Review specific commit by hash only

## Implementation

### Step 1: Check working tree status

1. Run `git status --porcelain` to get current status
2. Store the output to determine if working tree is clean

### Step 2: Determine review target

- If `$1` is empty (no argument provided):
  - If status output is empty → Review HEAD commit
  - If status output has changes → Review working changes only
- If `$1` is provided → Review the specified commit

### Step 3: Review commit (if applicable)

- When reviewing a commit (either HEAD or specified ref):
  1. Run `git log -1 --stat <ref>` to see commit summary
  2. Run `git show <ref>` to see detailed diff

### Step 4: Review working changes (if applicable)

- When working tree has changes:
  1. Parse `git status --porcelain` output
  2. For modified/staged files: Run `git diff HEAD` to see changes
  3. For untracked files (`??` prefix): Read each file content

### Step 5: Summarize

- Provide concise summary of findings:
  - If reviewed commit: What was accomplished and why
  - If reviewed working changes: Current modifications and additions
  - State clearly which mode was used

**Important**: Do not make any modifications - this is a read-only analysis
