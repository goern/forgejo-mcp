## Why

Two existing MCP tools require numeric IDs that cannot be discovered programmatically:

- `update_issue` accepts a `milestone` parameter that must be a milestone **ID**
- `add_issue_labels` accepts a `labels` parameter that must be numeric label **IDs**

There are no counterpart tools to list milestones or labels for a repository, making it impossible for an AI agent to autonomously resolve names to IDs without a separate API call outside the MCP interface.

Addresses [Codeberg issue #80](https://codeberg.org/goern/forgejo-mcp/issues/80) (reported by byteflavour).

## What Changes

- Add `list_repo_milestones` MCP tool — returns all milestones for a repository with their names and numeric IDs
- Add `list_repo_labels` MCP tool — returns all labels for a repository with their names and numeric IDs
- Register both tools in `operation/issue/issue.go` alongside existing issue tools

## Capabilities

### New Capabilities

- `list_repo_milestones`: discover milestone names → IDs for use with `update_issue`
- `list_repo_labels`: discover label names → IDs for use with `add_issue_labels`

### Modified Capabilities

_(none — additive only, no existing behaviour changes)_

## Impact

- **Code**: `operation/issue/issue.go` — two new tool definitions, handler functions, and registration entries
- **APIs**: Wraps existing Forgejo SDK methods `ListRepoMilestones` and `ListRepoLabels` — no new external dependencies
- **Risk**: Low. Read-only, additive change only.
