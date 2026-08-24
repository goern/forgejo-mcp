# SPDX-License-Identifier: GPL-3.0-or-later

# Demo: cancel/delete a workflow run and inspect its artifacts

Stop a runaway Actions run, delete a completed one, and list that run's
artifacts without downloading zips. Cancel and delete are not the same
operation: cancel is 204 even when the run already finished; delete only
succeeds for a completed run and removes it.

Do **not** run `cancel_workflow_run` or `delete_workflow_run` against a
repo you do not own. This walkthrough shows `--help` and the read-only
list/get surface.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"
make build
```

Spec: `openspec/specs/workflow-run-control/spec.md`

## 1. Tool surface

```bash
${FORGEJO_MCP_BIN} --cli list 2>/dev/null | grep -E "cancel_workflow_run|delete_workflow_run|list_action_run_artifacts|get_action_artifact"
```

```
  cancel_workflow_run                      Cancel a pending or running workflow run. Forgejo also returns 204 for a run that has already finished and leaves that run unchanged; that is success, not an error. Same Actions write as dispatch_workflow; a read-only token returns 403.
  delete_workflow_run                      Delete a completed workflow run (succeeded, failed, or cancelled). The run is removed, its job logs become unreadable, and Forgejo marks that run's artifacts deleted (storage reclaimed later). A live run is an API error (never mapped to success). No preflight GET. Same Actions write as dispatch_workflow; a read-only token returns 403.
  get_action_artifact                      Get metadata for one Actions artifact (id, name, run_id, size_in_bytes, expired, timestamps, archive_download_url). Does not download the zip.
  list_action_run_artifacts                List artifacts of a workflow run. Server-paged: page (default 1) and limit (default 30, maximum 50) are sent as query parameters, optional name filters by artifact name. Returns {run_id, artifacts, page, limit, count} and total_count when Forgejo sets X-Total-Count. A missing run is an error, not an empty list. Metadata only — does not download zips.
```

```bash
${FORGEJO_MCP_BIN} --cli cancel_workflow_run --help
${FORGEJO_MCP_BIN} --cli delete_workflow_run --help
${FORGEJO_MCP_BIN} --cli list_action_run_artifacts --help
${FORGEJO_MCP_BIN} --cli get_action_artifact --help
```

`cancel_workflow_run` / `delete_workflow_run` take `owner`, `repo`, `run_id`.
`list_action_run_artifacts` adds `page`, `limit`, and optional `name`.
`get_action_artifact` takes `artifact_id` instead of `run_id`.

## 2. Asymmetry

- **Cancel** POSTs `…/actions/runs/{run_id}/cancel`. HTTP 204 is success
  for a running run *and* for a run that already finished (Forgejo leaves
  the finished run unchanged).
- **Delete** DELETEs `…/actions/runs/{run_id}` with no GET beforehand. Only
  a completed run succeeds. A live run is an error (status may be 400 or
  500 depending on the instance). The run, its job logs, and its artifacts
  (marked deleted, storage reclaimed later) go away.

Do not invoke those two against this demo's sample repo.

## 3. List artifacts (read-only)

Server-paged. `count` is this page; `total_count` is present only when
Forgejo sends `X-Total-Count`. A missing run is an error, not `[]`.

```bash
${FORGEJO_MCP_BIN} --cli list_action_run_artifacts \
  --args '{"owner":"OWNER","repo":"REPO","run_id":RUN_ID,"page":1,"limit":30}'
```

Envelope: `{run_id, artifacts, page, limit, count, total_count?}`.
Each artifact includes `id`, `name`, `run_id`, `size_in_bytes`, `expired`,
timestamps, and `archive_download_url`. This tool does not fetch the zip.

## 4. Get one artifact (read-only)

```bash
${FORGEJO_MCP_BIN} --cli get_action_artifact \
  --args '{"owner":"OWNER","repo":"REPO","artifact_id":ARTIFACT_ID}'
```

Same metadata as one list row. Path is `…/actions/artifacts/{id}`, never
`…/zip`.

## End-to-end

An agent that needs to stop a stuck deploy: `list_workflow_runs` →
`cancel_workflow_run` on the running id → `list_action_run_artifacts` on
the run that already finished to see what it produced. To drop a failed
run after inspecting logs: confirm it is completed, then
`delete_workflow_run`. Never delete a live run; never treat that error as
success.
