# SPDX-License-Identifier: GPL-3.0-or-later

## Context

Job-log tools already use raw HTTP because the SDK does not cover Actions jobs. The same gap exists for run cancel/delete and artifacts. Forgejo's swagger (Codeberg / b4mad) documents the four endpoints below. `ListActionRunArtifacts` calls `SetTotalCountHeader`. `DeleteRun` refuses a run that is not `IsDone()` and marks that run's artifacts deleted.

The MCP Go SDK still has no `WithArray` concern here: arguments are scalars.

## Goals / Non-Goals

**Goals**

- Four tools wrapping the four REST endpoints.
- Honest cancel vs delete: 204 on cancel of a finished run is success; a live-run delete is an error the tool must surface.
- Server-side paging for the artifact list; `total_count` only from `X-Total-Count`.
- Metadata-only get; zip download stays out.

**Non-Goals**

- Repo-wide `GET …/actions/artifacts`.
- `DELETE …/actions/artifacts/{id}`.
- Zip download (`…/artifacts/{id}/zip`) and run-level log zip (`…/runs/{id}/logs`).
- `rerun` (handler exists upstream; not in the public Codeberg swagger this repo tracks).
- Secrets, runners, variables, workflow yaml listing.

## Decisions

### D1: Four tools, names next to existing Actions tools

`cancel_workflow_run` / `delete_workflow_run` sit beside `get_workflow_run`. `list_action_run_artifacts` / `get_action_artifact` sit beside `list_action_run_jobs`.

### D2: Raw HTTP, `APIPath`, export `DoJSONWithHeader`

No SDK methods. Paths go through `forgejo.APIPath`. List must not use `DoJSONList`: a 404 for a missing run is an error, not `[]`. List still needs `X-Total-Count`, so export `DoJSONWithHeader` (404 remains an error). Cancel/delete/get use `DoJSON`.

### D3: Cancel 204 is always success

Forgejo: pending/running jobs are cancelled; a finished run is left unchanged; both respond 204. The tool returns `{run_id, status: "cancelled"}` and does not GET the run first.

### D4: Delete has no preflight and never swallows errors

Forgejo rejects a live run (`cannot delete run … because it has not completed yet`). The HTTP status is 400 in swagger and 500 from the current handler. The tool DELETEs and returns whatever 4xx/5xx the instance sends. No GET-then-skip. Success payload `{run_id, status: "deleted"}`. Description tells the caller the run, its job logs, and its artifacts (marked deleted, reclaimed later) go away.

### D5: Artifact list is server-paged

Pass `page`, `limit`, and optional `name` as query parameters. Default limit 30, max 50 (Forgejo's page size). Envelope `{run_id, artifacts, page, limit, count, total_count?}`. `count` is this page. `total_count` is `TotalCountPtr` of the header — omit the key when the header is absent. Do not reuse jobs' `total_count` = local full-list length.

### D6: Get is metadata only

Return swagger `ActionArtifact` fields, including `archive_download_url`. Do not GET the zip. The URL is metadata; fetching it is a later slice.

### D7: Permissions and versions

Same Actions write as `dispatch_workflow`; a read-only token returns 403 and the tool shows it. Do not invent a Forgejo version tag. Missing endpoints surface as HTTP 404.

## Risks / Trade-offs

- Older instances 404 these routes; that is an error, not an empty success.
- Delete's live-run status code differs across swagger and handler; we pass it through instead of translating it.
- Exposing `archive_download_url` may tempt an agent to fetch a zip. The tool description forbids that fetch on this slice.
