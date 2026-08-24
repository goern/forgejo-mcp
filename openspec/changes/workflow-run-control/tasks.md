# SPDX-License-Identifier: GPL-3.0-or-later

## 1. HTTP helper

- [x] 1.1 Export `DoJSONWithHeader` (404 remains an error, unlike `DoJSONList`)
- [x] 1.2 Test: 404 is an error; 200 returns headers including `X-Total-Count`

## 2. Tools

- [x] 2.1 Implement `cancel_workflow_run` (POST `…/runs/{id}/cancel`; 204 → `{run_id, status: "cancelled"}`)
- [x] 2.2 Implement `delete_workflow_run` (DELETE `…/runs/{id}`; 4xx/5xx stay errors; no preflight)
- [x] 2.3 Implement `list_action_run_artifacts` (server `page`/`limit`/`name`; envelope with `count` and optional `total_count`)
- [x] 2.4 Implement `get_action_artifact` (metadata only; no zip)
- [x] 2.5 Register the four tools from `actions.RegisterTool`

## 3. Tests

- [x] 3.1 cancel running → POST `…/runs/42/cancel`, 204 → `status: "cancelled"`
- [x] 3.2 cancel already-finished → same POST, 204 → success, not error
- [x] 3.3 delete completed → DELETE, 204 → `status: "deleted"`
- [x] 3.4 delete live → 4xx/5xx → MCP error, no success, DELETE still sent
- [x] 3.5 list query + envelope; `X-Total-Count: 4` → `"total_count":4`; header absent → key omitted
- [x] 3.6 list 404 run → error, not empty success
- [x] 3.7 get metadata fields; path has no `/zip`
- [x] 3.8 missing owner/repo/run_id/artifact_id → error, zero requests
- [x] 3.9 owner with `/` → `APIPath` does not retarget

## 4. Wrap-up

- [x] 4.1 README Actions table + CLI example + `demos/workflow-run-control.md` + `demos/README.md` + `extension/manifest.json`
- [x] 4.2 `go test ./operation/actions/ ./pkg/forgejo/`
