## Context

forgejo-mcp has full PR lifecycle tools (create, update, list, review) but no way to merge a PR. The Forgejo SDK provides `Client().MergePullRequest(owner, repo, index, MergePullRequestOption)` which wraps `POST /repos/{owner}/{repo}/pulls/{index}/merge`. This is a single tool addition in the existing `operation/pull/` domain.

The `pull.go` file currently has 7 tools (284 lines) plus `review.go` has 6 tools. Adding one more tool to `pull.go` is reasonable — it's a core PR operation, not a review operation.

## Goals / Non-Goals

**Goals:**
- Expose `merge_pull_request` as an MCP tool with all merge options from the SDK
- Follow existing patterns in `pull.go` exactly

**Non-Goals:**
- Splitting `pull.go` further — one more tool is fine
- Exposing `MergeCommitID` or `HeadCommitId` parameters — these are advanced fields that the API typically handles automatically; can add later if requested

## Decisions

### 1. Add the tool directly in `pull.go`

**Decision**: Add the tool definition, handler, and registration to `operation/pull/pull.go`.

**Rationale**: This is a single tool that belongs alongside create/update/list PR operations. A separate file would be over-engineering. The file grows by ~40 lines, staying well under 350 lines total.

**Alternative considered**: New `merge.go` file — unnecessary for a single tool.

### 2. Expose merge style as a string enum parameter

**Decision**: Accept `style` as a string parameter with values `merge`, `rebase`, `rebase-merge`, `squash`. These map directly to `forgejo_sdk.MergeStyle`.

**Rationale**: The SDK uses string-typed `MergeStyle` constants. Direct passthrough avoids any conversion layer. The tool description documents valid values.

### 3. Boolean options as optional parameters

**Decision**: Expose `delete_branch_after_merge`, `force_merge`, and `merge_when_checks_succeed` as optional boolean parameters using `mcp.WithBoolean()`.

**Rationale**: These are straightforward flags. Making them optional with no default means the Forgejo server applies its own defaults (all false). This keeps the simplest call (`owner`, `repo`, `index`, `style`) minimal.

### 4. Optional title and message parameters

**Decision**: Expose `title` and `message` as optional string parameters for the merge commit.

**Rationale**: Useful for squash merges where the user wants to customize the commit message. For regular merges, Forgejo generates a default message. Only set on the option struct when non-empty, consistent with `UpdatePullRequestFn` pattern.

### 5. Return success boolean, not the merged PR

**Decision**: Return `"Pull request merged successfully"` on success (the SDK returns `(bool, *Response, error)`).

**Rationale**: The SDK's `MergePullRequest` returns a bool, not the PR object. Returning a clear text message is more useful than `true`. On failure, the SDK error propagates as usual.

## Risks / Trade-offs

- **Merge is destructive** — once merged, it cannot be undone via the API. This is inherent to the operation; the tool description should note this. → Mitigated by requiring explicit `style` parameter (no accidental default merge style).
- **`force_merge` bypasses checks** — could be misused. → Mitigated by making it optional/false by default. The tool description documents what it does.
- **`merge_when_checks_succeed` is async** — the merge doesn't happen immediately. → The tool should return a message indicating the merge was scheduled, not completed. The SDK returns `true` in both cases; the response message should note this parameter was set.
