# SPDX-License-Identifier: GPL-3.0-or-later

## Why

`forgejo-mcp` can dispatch a workflow and read runs, jobs, and job logs, but it cannot stop a runaway run, remove a completed one, or inspect the artifacts that run produced. Agents fall back to raw HTTP for `POST …/actions/runs/{id}/cancel`, `DELETE …/actions/runs/{id}`, and the run-scoped artifact endpoints. The pinned `forgejo-sdk/v3` has none of those methods.

Cancel and delete are not symmetric. Forgejo answers cancel with 204 even when the run already finished and leaves it unchanged. Delete only succeeds for a completed run; a live run is an API error. Delete also removes the run, makes that run's job logs unreadable, and marks the run's artifacts deleted (storage reclaimed in the background). Pretending they are the same operation, or mapping a live-run delete to success, would lie to the caller.

## What Changes

- Add MCP tools `cancel_workflow_run`, `delete_workflow_run`, `list_action_run_artifacts`, `get_action_artifact`.
- Call Forgejo over raw HTTP (`APIPath` + `DoJSON` / `DoJSONWithHeader`). Do not add an SDK method.
- `cancel_workflow_run`: POST cancel; HTTP 204 is success, including when the run already finished.
- `delete_workflow_run`: DELETE the run; 4xx/5xx stay errors; no GET preflight; never map a live-run failure to success.
- `list_action_run_artifacts`: server-paged GET with `page`/`limit`/`name`; envelope `{run_id, artifacts, page, limit, count, total_count?}`. A missing run is 404, not an empty list.
- `get_action_artifact`: metadata only. Do not GET `…/artifacts/{id}/zip`.
- README Actions table, `demos/workflow-run-control.md`, `extension/manifest.json`.

## Capabilities

### New Capabilities

- `workflow-run-control`: Cancel or delete a workflow run, list that run's artifacts with a bounding envelope, and get one artifact's metadata. Raw HTTP against current Forgejo Actions endpoints. Cancel is idempotent-at-204; delete is not.

### Modified Capabilities

- None.

## Impact

- **Affected code**: `operation/actions/run_control.go`, `operation/actions/artifacts.go`, `pkg/forgejo` (`DoJSONWithHeader`), README, demos, `extension/manifest.json`.
- **APIs / SDK**: no new SDK methods. `POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel`, `DELETE /repos/{owner}/{repo}/actions/runs/{run_id}`, `GET /repos/{owner}/{repo}/actions/runs/{run_id}/artifacts`, `GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}`.
- **Output bounding**: cancel, delete, and get return one object or success — exempt. List is server-paged (`page`/`limit`, default 30, max 50) with `count` plus `total_count` from `X-Total-Count` (handler calls `SetTotalCountHeader`).
