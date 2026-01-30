---
description: Stage all changes, run pre-commit hooks, fix failures, and create conventional commit with emoji
argument-hint: [all | staged | amend | split | dry-run] [message]
allowed-tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion, Task
model: sonnet
---

<objective>
Create a well-formatted git commit with conventional commit messages and emoji.

**Modes:**

- *(no args)* or `all` - Stage all changes and commit (default)
- `staged` - Commit only already-staged files (skip `git add -A`)
- `amend` - Amend the previous commit (with safety checks)
- `split` - Interactive mode to split changes into multiple commits
- `dry-run` - Preview what would be committed without committing

**Optional message:** Provide a commit message to skip auto-generation.

**Examples:**

- `/commit` - Stage all, generate message, commit
- `/commit staged` - Commit only staged files
- `/commit staged "fix: typo"` - Commit staged with custom message
- `/commit dry-run` - Preview without committing
- `/commit split` - Guide through splitting changes
</objective>

<context>
Git status: !`git status --short`
Recent commits (for style matching): !`git log --oneline -5`
Pre-commit config exists: !`test -f .pre-commit-config.yaml && echo "yes" || echo "no"`
Staged files: !`git diff --cached --name-only`
</context>

<process>

## Mode Detection

Parse `$ARGUMENTS` to determine mode and optional message:

1. **If empty or "all"**: Default mode - stage all, commit
2. **If "staged"**: Skip staging, commit only what's already staged
3. **If "amend"**: Amend previous commit (go to Amend Mode)
4. **If "split"**: Interactive splitting (go to Split Mode)
5. **If "dry-run"**: Preview only, no commit (go to Dry-Run Mode)
6. **Remaining args**: Treat as custom commit message

---

## Default Mode (all)

1. **Stage all changes**
   - Run `git add -A` to stage all modified, new, and deleted files

## Staged Mode

1. **Verify staged files exist**
   - Run `git diff --cached --name-only`
   - If empty, report "No staged files" and exit
   - Do NOT run `git add -A` - use existing staged files only

2. **Check for pre-commit hooks**
   - If `.pre-commit-config.yaml` exists, run `pre-commit run --all-files`
   - If pre-commit is not installed, skip this step

3. **Handle pre-commit failures**
   - If pre-commit fails, spawn a **Haiku subagent** to fix simple issues:

   ```
   Task tool with:
   - subagent_type: "general-purpose"
   - model: "haiku"
   - prompt: "Fix pre-commit failures.

     **Errors:**
     {paste pre-commit error output}

     **Instructions:**
     1. Read failing files
     2. Fix formatting, linting, markdown, whitespace issues
     3. For security/architectural issues: return as 'escalated'

     **Return JSON:**
     {\"fixed\": [\"file1.md\", ...], \"escalated\": [\"security issue in X\", ...]}"
   ```

   - If subagent returns escalated issues, spawn **Sonnet subagent**:

   ```
   Task tool with:
   - subagent_type: "general-purpose"
   - model: "sonnet"
   - prompt: "Fix complex pre-commit issues requiring reasoning.

     **Escalated issues:**
     {paste escalated issues}

     **Instructions:**
     1. Analyze root cause
     2. Implement fix with explanation

     **Return:** Summary of changes and reasoning"
   ```

   - After fixing, do NOT stage the fixes yet

4. **User approval for agent changes**
   - If any files were modified by pre-commit or by fixing failures:
     - Run `git diff` to show unstaged changes (these are the agent's fixes)
     - Use AskUserQuestion to ask user to review the diff
     - Options: "Approve changes" or "Reject and abort"
   - If rejected, run `git checkout -- .` to discard fixes and abort

5. **Stage fixes and analyze changes**
   - Run `git add -A` to stage all fixes
   - Run `git diff --stat --cached` for overview
   - **For large diffs (>300 lines)**, spawn Haiku subagent to analyze:

   ```
   Task tool with:
   - subagent_type: "general-purpose"
   - model: "haiku"
   - prompt: "Analyze staged changes and recommend commit message.

     **Staged files:**
     {paste git diff --stat --cached output}

     **Recent commits (for style):**
     {paste git log --oneline -5 output}

     **Instructions:**
     1. Read key changed files to understand scope
     2. Summarize the nature of changes (feature, fix, refactor, etc.)
     3. Match project commit style from recent commits
     4. Consider if changes should be split

     **Return:**
     - Recommended commit message: <emoji> <type>: <description>
     - Summary of changes (2-3 bullet points)
     - Split recommendation: yes/no with reasoning"
   ```

   - For small diffs: analyze directly without subagent

6. **Match project commit style**
   - Use subagent's analysis or analyze recent commits for: language, emoji usage, format conventions
   - Follow the existing style (don't impose a standard)

7. **Generate commit message**
   - Use subagent's recommendation or generate based on analysis
   - Use emoji conventional commit format: `<emoji> <type>: <description>`
   - First line < 72 characters, imperative mood, present tense
   - Focus on WHAT changed and WHY, not HOW
   - For complex changes, use multi-line format with bullet points
   - Consider if changes should be split into multiple commits

8. **Create the commit**
   - Run `git commit -m "<message>"` using HEREDOC for proper formatting
   - Verify commit was created successfully with `git log -1`

---

## Amend Mode

**Safety checks before amending:**

1. Verify HEAD commit was made by user (not a merge commit)
2. Verify commit has NOT been pushed to remote: `git status` shows "Your branch is ahead"
3. If pushed, WARN and ask for confirmation (requires force push)

**Process:**

1. Stage changes (all or staged-only based on additional args)
2. Run pre-commit hooks
3. Show diff of what will be amended
4. Use `AskUserQuestion` for confirmation
5. Run `git commit --amend` (with `-m` if message provided, otherwise keep existing)
6. Verify with `git log -1`

---

## Split Mode

Guide user through splitting changes into multiple logical commits.

1. **Analyze changes**
   - Run `git status` to see all changes
   - Categorize by: file type, concern area, change type (feat/fix/refactor)

2. **Propose split strategy**
   - Suggest how to group changes into separate commits
   - Example: "I see 3 logical groups: config changes, new feature, test updates"

3. **Interactive staging**
   - For each group, guide user through:
     - `git add <specific-files>` or `git add -p` for partial staging
     - Generate commit message for that group
     - Create commit
   - Repeat until all changes committed

4. **Verify**
   - Show `git log --oneline -n` for the new commits
   - Confirm all changes are committed

---

## Dry-Run Mode

Preview what would be committed without actually committing.

1. **Stage changes** (unless `staged` also specified)
   - Run `git add -A` to stage all

2. **Run pre-commit hooks**
   - Show what would pass/fail
   - Do NOT fix failures (just report)

3. **Show preview**

   ```
   ## Dry-Run Preview

   **Files to commit:**
   {git diff --stat --cached}

   **Proposed commit message:**
   <emoji> <type>: <description>

   **Pre-commit status:** PASS / FAIL (details)

   Run `/commit` to actually commit these changes.
   ```

4. **Unstage if we staged**
   - If mode was `dry-run` (not `staged dry-run`), run `git reset HEAD` to unstage

</process>

<commit_types>

- ✨ `feat`: New feature
- 🐛 `fix`: Bug fix
- 📝 `docs`: Documentation
- 💄 `style`: Formatting/style
- ♻️ `refactor`: Code refactoring
- ⚡️ `perf`: Performance improvements
- ✅ `test`: Tests
- 🔧 `chore`: Tooling, configuration
- 🚀 `ci`: CI/CD improvements
- 🚨 `fix`: Fix compiler/linter warnings
- 🔒️ `fix`: Fix security issues
- 🏗️ `refactor`: Architectural changes
- 🔥 `fix`: Remove code or files
- 🎨 `style`: Improve structure/format
- 💚 `fix`: Fix CI build
- ✏️ `fix`: Fix typos
</commit_types>

<splitting_guidance>
Consider splitting commits when changes touch:

- Different concerns (unrelated parts of codebase)
- Different types (mixing features, fixes, refactoring)
- Different file patterns (source vs docs vs config)
- Large changes that would be clearer if broken down

If splitting is needed, guide the user through selective staging with `git add -p` or file-by-file staging.
</splitting_guidance>

<grep_strategy>
Grep is faster than reading entire files. Use it to quickly assess impact before deciding which files to read in detail.

**Patterns for analyzing changes:**

- Find function/method calls: `grep -n "function_name("`
- Count occurrences to gauge scope: `grep -c "pattern" file`
- Get context around matches: `grep -C 3 "function_name"`
- Find all files with pattern: `grep -l "pattern" --include="*.py"`

**When to use grep vs full read:**

- Use grep first to identify which files have significant changes
- Read full file only when grep shows complex modifications
- For simple additions/deletions, grep context is often sufficient
- Prioritize reading files with highest match counts (most impactful)
</grep_strategy>

<success_criteria>

- All files staged appropriately
- Pre-commit hooks pass (if present)
- User approved any agent-made changes
- Commit message follows project conventions
- Commit message uses emoji conventional commit format
- Commit created successfully
- No uncommitted changes remain (unless intentionally excluded)
</success_criteria>

<verification>
After commit, verify:
- `git status` shows clean working directory
- `git log -1 --stat` shows expected changes
- Commit message is properly formatted
</verification>
