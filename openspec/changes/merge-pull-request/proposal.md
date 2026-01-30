## Why

There is no MCP tool to merge a pull request. Users (and AI agents) can create, update, list, and review PRs, but cannot complete the merge — forcing a manual step in an otherwise automated workflow. The forgejo SDK already provides `MergePullRequest`, so this is a straightforward gap to close.

Addresses [Codeberg issue #54](https://codeberg.org/goern/forgejo-mcp/issues/54).

## What Changes

- Add a new `merge_pull_request` MCP tool that calls the Forgejo API endpoint `POST /repos/{owner}/{repo}/pulls/{index}/merge`
- Expose merge options: merge style (merge, squash, rebase), title, message, delete-branch-after-merge, force-merge, and merge-when-checks-succeed
- Register the tool alongside existing PR tools in `operation/pull/pull.go`

## Capabilities

### New Capabilities

- `merge-pull-request`: MCP tool to merge a pull request with configurable merge style and options

### Modified Capabilities

_(none — this is additive, no existing behavior changes)_

## Impact

- **Code**: `operation/pull/pull.go` — new tool definition, handler function, and registration entry
- **Code**: `operation/params/params.go` — possible new shared parameter descriptions for merge-specific fields
- **APIs**: Wraps existing Forgejo SDK method `Client().MergePullRequest()` — no new external dependencies
- **Risk**: Low. Additive change only; no modifications to existing tools or behavior
