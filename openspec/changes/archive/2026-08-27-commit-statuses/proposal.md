# SPDX-License-Identifier: GPL-3.0-or-later

## Why

`forgejo://repo/{owner}/{repo}/commit/{sha}/status` already names `get_commit_statuses` as the paginated list when the embedded statuses are truncated. The tool did not exist. Agents that hit the cap had a named escape hatch that returned nothing.

Commit statuses are per-context CI checks (`success` / `failure` / `pending`), not Actions workflow runs. Combined aggregate state stays on the resource. The pinned `forgejo-sdk/v3` already has `ListStatuses`.

## What Changes

- Add MCP tool `get_commit_statuses` wrapping SDK `ListStatuses`.
- Require a full 40-character hex `sha` (`resource.ValidateSHA`); a short SHA is an error and makes no request.
- Server-paged `page` (default 1) and `limit` (default 30 = `EmbeddedListCap`, maximum 50). Envelope `{sha, statuses, page, limit, count}`.
- Each item uses the resource field names (`state`, not the SDK JSON key `status`).
- README Commits row, one `--cli` example in the existing resource-templates demo §5, `extension/manifest.json`.

## Capabilities

### New Capabilities

- `commit-statuses`: Paginated per-context commit statuses for a full SHA via existing SDK `ListStatuses`. Combined aggregate stays on the status resource.

### Modified Capabilities

- None. The status resource URI and payload are unchanged.

## Impact

- **Affected code**: `operation/repo/statuses.go`, `resource.ValidateSHA`, README, `demos/mcp-resource-templates.md` §5, `extension/manifest.json`.
- **APIs / SDK**: `ListStatuses` → `GET /repos/{owner}/{repo}/commits/{sha}/statuses`.
- **Output bounding**: list (`page`/`limit`, envelope `{sha, statuses, page, limit, count}`). Default `limit` matches the resource cap so `page=2` continues after a truncated resource read.
