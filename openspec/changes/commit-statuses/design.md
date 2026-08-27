# SPDX-License-Identifier: GPL-3.0-or-later

## Context

The commit status resource embeds up to `EmbeddedListCap` (30) per-context items and, when truncated, sets `list_tool` to `get_commit_statuses`. That name was a promise, not a tool. Combined state (`success`/`failure`/`pending`/`unknown`) is computed on the resource from the over-fetched list and must not be recomputed here.

`list_workflow_runs` already filters Actions runs by `head_sha`. Commit statuses are a different REST surface (`…/commits/{sha}/statuses`).

## Goals / Non-Goals

**Goals**

- One list tool wrapping SDK `ListStatuses`.
- Same SHA rule as the resource (exactly 40 hex).
- Bounding envelope; default page size matches the resource cap.
- Item shape matches `statusItem` (`state`, not SDK `status`).

**Non-Goals**

- `create_status` / POST a new context.
- A second combined-status tool (aggregate stays on the resource).
- Changing the resource payload or URI.
- Actions inventory, rerun, or artifact zip.
- Packages.

## Decisions

### D1: One tool, the name the resource already uses

`get_commit_statuses`. Do not add `get_combined_commit_status`. Callers that want the aggregate read `forgejo://…/commit/{sha}/status`.

### D2: Full SHA, same function as the resource

Export `validateSHA` as `resource.ValidateSHA`. The tool calls it before `Client()`. A short or non-hex SHA is an error with zero HTTP. Forgejo accepts abbreviated SHAs; this MCP surface does not, so a truncated resource and a tool call cannot disagree on the key.

### D3: Envelope and bounds

`page` default 1, `limit` default 30 (`EmbeddedListCap`), maximum 50 (same ceiling as `list_action_run_artifacts`). Response `{sha, statuses, page, limit, count}`. `count` is this page's length. Re-issue with `page` incremented. No `total_count` — the SDK list call does not expose `X-Total-Count` the way the raw-HTTP artifact list does.

### D4: Field names follow the resource

Map SDK `Status.State` (JSON `status`) to `state`. Item keys: `context`, `state`, `target_url`, `description`, `created_at`. A nil SDK slice becomes `[]`, not JSON null.

### D5: Description names the Actions boundary once

The tool description states that these are commit statuses, not Actions runs (`list_workflow_runs`). Do not repeat that in every README sentence.

## Risks / Trade-offs

- Rejecting a short SHA that Forgejo would resolve is stricter than the REST API. Same as the resource; intentional.
- Default `limit` 30 is smaller than many list tools (100). It exists so page 2 is the continuation of a truncated resource read, not a second copy of page 1.
