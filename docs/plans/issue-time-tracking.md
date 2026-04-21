# Issue & PR Time Tracking — Implementation Plan

**Beads**: `forgejo-mcp-ya6`
**Branch**: `feature/issue-time-tracking` *(suggested)*

## Summary

Add MCP tools that expose Forgejo's **tracked time** and **stopwatch** APIs for issues and pull requests. Currently the server has no way to read, add, or remove logged time — agents cannot account for the work they do and users cannot query an issue's time ledger through the MCP.

Unlike the attachments work (see `issue-attachments.md`), **`forgejo-sdk/v3@v3.0.0` ships complete method coverage** for both APIs. No raw HTTP helper is needed; this spec is a straightforward SDK-wrapping job.

## Use Case

> As an agent or user working through `forgejo-mcp`, I want to read the time ledger of any issue or PR, log time against it, clear or remove individual entries, and drive a live stopwatch for timing in-progress work. I want to do this with one set of tools that works transparently for both issues and PRs, because Forgejo shares index space between them.

## Current State

- `operation/issue/issue.go` registers 14 issue tools; none touch tracked time or stopwatches.
- `forgejo-sdk/v3@v3.0.0` provides:
  - **Tracked time** (`issue_tracked_time.go`): `AddTime`, `ListIssueTrackedTimes`, `ListRepoTrackedTimes`, `GetMyTrackedTimes`, `ResetIssueTime`, `DeleteTime`.
  - **Stopwatch** (`issue_stopwatch.go`): `StartIssueStopWatch`, `StopIssueStopWatch`, `DeleteIssueStopwatch`, `GetMyStopwatches`.
- `pkg/forgejo/forgejo.go` singleton client is already suitable — nothing new needed at the client layer.

## Key Design Decisions

### 1. SDK-wrapping, not raw HTTP

The SDK covers every call. No `rawhttp` helper. Handlers are thin: parse args → call `forgejo.Client().X(...)` → `to.TextResult`.

### 2. Verbs mirror API primitives (no `set_issue_time`)

The Forgejo API has **no "set total" operation**. To change the total for a user on an issue you must reset + add, and `ResetIssueTime` wipes **all** entries including other users'. We deliberately do **not** expose a `set_issue_time` convenience wrapper: the composite is non-atomic (another user could log time between reset and add) and destructive to other users' history. Agents needing "set" semantics compose `reset_issue_time` + `add_issue_time` themselves and accept the risk explicitly.

### 3. One tool set for issues **and** PRs

Forgejo's issue index namespace includes PRs (a PR is just an issue with a PR payload attached). Every tracked-time and stopwatch endpoint is `/repos/{owner}/{repo}/issues/{index}/…`, so a PR index works identically to an issue index. Every tool description must literally state **"issue or pull request index"** so agents don't go looking for a `pr_*` variant.

### 4. Accept both `seconds` and `duration` for `add_issue_time`

The API takes integer seconds. Agents naturally think in minutes or hours. We accept **either** `seconds` (number) **or** `duration` (string parsed via `time.ParseDuration`, e.g. `"15m"`, `"1h30m"`). Exactly one must be provided. Negative values rejected. Parsing uses stdlib only; no extra dependency.

## Target State

### 10 New MCP Tools

**Tracked time (6):**

| Tool | HTTP | SDK method | Returns |
|------|------|------------|---------|
| `list_issue_tracked_times` | GET `/repos/{o}/{r}/issues/{index}/times` | `ListIssueTrackedTimes` | `[]TrackedTime` |
| `list_repo_tracked_times` | GET `/repos/{o}/{r}/times` | `ListRepoTrackedTimes` | `[]TrackedTime` |
| `list_my_tracked_times` | GET `/user/times` | `GetMyTrackedTimes` | `[]TrackedTime` |
| `add_issue_time` | POST `/repos/{o}/{r}/issues/{index}/times` | `AddTime` | `TrackedTime` |
| `reset_issue_time` | DELETE `/repos/{o}/{r}/issues/{index}/times` | `ResetIssueTime` | `"Reset tracked time success"` |
| `delete_issue_time_entry` | DELETE `/repos/{o}/{r}/issues/{index}/times/{id}` | `DeleteTime` | `"Delete time entry success"` |

**Stopwatches (4):**

| Tool | HTTP | SDK method | Returns |
|------|------|------------|---------|
| `start_issue_stopwatch` | POST `/repos/{o}/{r}/issues/{index}/stopwatch/start` | `StartIssueStopWatch` | `"Stopwatch started"` |
| `stop_issue_stopwatch` | POST `/repos/{o}/{r}/issues/{index}/stopwatch/stop` | `StopIssueStopWatch` | `"Stopwatch stopped; elapsed time recorded"` |
| `cancel_issue_stopwatch` | DELETE `/repos/{o}/{r}/issues/{index}/stopwatch/delete` | `DeleteIssueStopwatch` | `"Stopwatch cancelled"` |
| `list_my_stopwatches` | GET `/user/stopwatches` | `GetMyStopwatches` | `[]StopWatch` |

### Tool Parameter Schemas

Shared parameters (added to `pkg/params/params.go`):

```go
TimeID         = "Tracked time entry ID"
TimeSeconds    = "Time in seconds to log (positive integer). Provide exactly one of seconds or duration."
TimeDuration   = "Time to log as a duration string, e.g. \"15m\", \"1h30m\", \"45s\". Provide exactly one of seconds or duration."
TimeCreatedAt  = "Optional RFC3339 timestamp for when the work happened (defaults to server time)"
TimeUserName   = "Optional username to log time on behalf of (requires admin; omit for self)"
TimeUserFilter = "Filter results to this username (list_repo_tracked_times only)"
```

Reuse existing `params.Since` and `params.Before` for RFC3339 time filters on list tools (same pattern as `list_issue_comments`).

Parameter matrix:

| Tool | `owner` | `repo` | `index` | `time_id` | `seconds` | `duration` | `created_at` | `user_name` | `since` | `before` | `user` |
|------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `list_issue_tracked_times` | ✓ | ✓ | ✓ | | | | | | opt | opt | — |
| `list_repo_tracked_times` | ✓ | ✓ | | | | | | | opt | opt | opt |
| `list_my_tracked_times` | | | | | | | | | | | |
| `add_issue_time` | ✓ | ✓ | ✓ | | *a* | *a* | opt | opt | | | |
| `reset_issue_time` | ✓ | ✓ | ✓ | | | | | | | | |
| `delete_issue_time_entry` | ✓ | ✓ | ✓ | ✓ | | | | | | | |
| `start_issue_stopwatch` | ✓ | ✓ | ✓ | | | | | | | | |
| `stop_issue_stopwatch` | ✓ | ✓ | ✓ | | | | | | | | |
| `cancel_issue_stopwatch` | ✓ | ✓ | ✓ | | | | | | | | |
| `list_my_stopwatches` | | | | | | | | | | | |

*a* = exactly one of `seconds` or `duration` is required.

The `user` filter is only honored by the repo-level list endpoint; the SDK documents this explicitly (`// User filter is only used by ListRepoTrackedTimes !!!`). We do not expose it on `list_issue_tracked_times` to avoid a silently-ignored parameter that would mislead agents.

### Architecture

```
operation/
 ├─ issue/        (existing — untouched)
 ├─ attachment/   (proposed in issue-attachments.md)
 └─ time/          ← NEW
     └─ time.go    (10 tools + handlers)
```

Handlers follow the style used in `operation/issue/issue.go`. Example:

```go
func AddIssueTimeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    owner, _ := req.GetArguments()["owner"].(string)
    repo, _  := req.GetArguments()["repo"].(string)
    index, _ := to.Float64(req.GetArguments()["index"])

    secsF, hasSecs := to.Float64Ok(req.GetArguments()["seconds"])
    durStr, hasDur := req.GetArguments()["duration"].(string)

    var seconds int64
    switch {
    case hasSecs && hasDur:
        return to.ErrorResult(fmt.Errorf("provide exactly one of 'seconds' or 'duration'"))
    case hasSecs:
        if secsF <= 0 {
            return to.ErrorResult(fmt.Errorf("seconds must be positive"))
        }
        seconds = int64(secsF)
    case hasDur:
        d, err := time.ParseDuration(durStr)
        if err != nil {
            return to.ErrorResult(fmt.Errorf("invalid duration %q: %v", durStr, err))
        }
        if d <= 0 {
            return to.ErrorResult(fmt.Errorf("duration must be positive"))
        }
        seconds = int64(d / time.Second)
    default:
        return to.ErrorResult(fmt.Errorf("one of 'seconds' or 'duration' is required"))
    }

    opt := forgejo_sdk.AddTimeOption{Time: seconds}
    if u, ok := req.GetArguments()["user_name"].(string); ok && u != "" {
        opt.User = u
    }
    if ts, ok := req.GetArguments()["created_at"].(string); ok && ts != "" {
        parsed, err := time.Parse(time.RFC3339, ts)
        if err != nil {
            return to.ErrorResult(fmt.Errorf("invalid created_at (RFC3339): %v", err))
        }
        opt.Created = parsed
    }

    entry, _, err := forgejo.Client().AddTime(owner, repo, int64(index), opt)
    if err != nil {
        return to.ErrorResult(fmt.Errorf("add issue time err: %v", err))
    }
    return to.TextResult(entry)
}
```

A small helper `to.Float64Ok(any) (float64, bool)` is added to `pkg/to/number.go` since the current `Float64` returns `(float64, error)` without distinguishing "absent" from "malformed". This helper is required by `add_issue_time` to do the `hasSecs` check cleanly.

## Implementation Steps

### Part 1 — `pkg/to` helper

1. Add `Float64Ok(v any) (float64, bool)` to `pkg/to/number.go`.
2. Unit test: returns `(0, false)` for `nil`, `(n, true)` for numeric inputs, `(0, false)` for non-numeric.

### Part 2 — `operation/time/time.go`

1. Declare the 10 tool constants and `mcp.NewTool(...)` definitions, matching the style in `operation/issue/issue.go`.
2. Implement each handler as a thin SDK wrapper.
3. Implement `RegisterTool(s *server.MCPServer)` that `AddTool`s all 10.

### Part 3 — Wire-up

1. Add `time.RegisterTool(s)` in `operation/operation.go`.
2. Add new parameter descriptions in `pkg/params/params.go`.
3. Add the 10 tool rows to the `README.md` tool table.

### Part 4 — Tests

- `operation/time/time_test.go` with mock SDK calls for:
  - `seconds` vs `duration` validation branches (both, neither, negative, malformed).
  - RFC3339 parsing for `created_at`.
  - Happy path for one `add` + one `list` + one `delete` to verify wiring.
- No live-API integration tests in CI.

### Part 5 — Demos (Showboat format)

Two files in `demos/` following the `issue-labels.md` format, including the `showboat-id` front-matter header:

1. **`demos/issue-time-tracking.md`** — end-to-end tracked-time lifecycle:
   - Create scratch issue (`create_issue`).
   - `add_issue_time --duration 15m` → shows returned entry.
   - `add_issue_time --seconds 1800` → second entry.
   - `list_issue_tracked_times` → shows both entries with IDs and user names.
   - `delete_issue_time_entry --time_id <id>` → removes one.
   - `list_issue_tracked_times` → confirms only one remains.
   - `reset_issue_time` → wipes all.
   - `list_issue_tracked_times` → empty.
   - `list_my_tracked_times`, `list_repo_tracked_times` with `--since` filter for completeness.
   - Close scratch issue.

2. **`demos/issue-stopwatch.md`** — end-to-end stopwatch lifecycle:
   - Create scratch issue.
   - `start_issue_stopwatch`.
   - `list_my_stopwatches` → shows the running stopwatch.
   - Short sleep (3 s, just enough to produce a nonzero entry).
   - `stop_issue_stopwatch` → converts to tracked-time entry.
   - `list_issue_tracked_times` → confirms ~3 s entry appears.
   - Second run: `start` → `cancel_issue_stopwatch` → `list_issue_tracked_times` confirms no new entry.
   - `list_my_stopwatches` → empty.
   - Close scratch issue.

## Deliverables

- [ ] `operation/time/time.go` + tests.
- [ ] `pkg/to/number.go` updated with `Float64Ok` helper + tests.
- [ ] `pkg/params/params.go` updated with new `Time*` constants.
- [ ] `operation/operation.go` registers `time` tools.
- [ ] `README.md` tool table updated.
- [ ] `demos/issue-time-tracking.md`.
- [ ] `demos/issue-stopwatch.md`.
- [ ] CHANGELOG entry under next version.

## Acceptance Criteria

1. `./forgejo-mcp --cli add_issue_time --owner goern --repo forgejo-mcp --index 106 --duration 15m` returns a `TrackedTime` JSON with `time: 900`.
2. `./forgejo-mcp --cli add_issue_time … --seconds 1800` same shape.
3. Supplying both `seconds` and `duration`, or neither, or a negative value, returns a structured error — not a 500 from the API.
4. `list_issue_tracked_times` returns entries for both issues and PRs (round-tripped by passing a PR index).
5. `delete_issue_time_entry` removes exactly one entry; `reset_issue_time` removes all.
6. Stopwatch `start → stop` produces a tracked-time entry visible via `list_issue_tracked_times`; `start → cancel` produces none.
7. `list_my_stopwatches` reflects currently running stopwatches accurately.
8. `make build` and `go test ./...` pass.
9. Both demo files run cleanly end-to-end against a live Forgejo (manual verification).

## Open Questions

1. **Admin-only `user_name` on `add_issue_time`**: Forgejo restricts logging time on behalf of another user to admins. Expose the param with a clear description and let the API return 403 for non-admins (recommendation). Alternative: drop the param entirely until someone needs it (YAGNI).
2. **Stopwatch `stop` response**: the SDK returns only `*Response`, not the created `TrackedTime`. Demo recovers the new entry via `list_issue_tracked_times`. Not a blocker, just noted.
3. **`list_my_tracked_times` pagination**: the SDK method takes no options. If Forgejo paginates, the first page is all we get. Accept as a known limitation; revisit if SDK adds options.

## Risk Assessment

- **Low risk**: all 10 tools are additive; no existing tool changes behavior.
- **Low risk**: SDK-wrapping means breakage only if the SDK itself breaks, which is centrally addressed at migration time.
- **Medium-low risk**: `reset_issue_time` deletes **all** entries for the issue (including other users'). Tool description must state this explicitly to prevent accidental data loss. Recommend surfacing this in the tool description, not just the parameter docs.

## Follow-ups (not in this spec)

- Expose `ListTrackedTimesOptions.User` consistently on `list_issue_tracked_times` (SDK allows it on repo-level but not issue-level per the `User filter is only used by ListRepoTrackedTimes !!!` comment in the SDK source). File an upstream issue if we want issue-level filtering.
- Project-level time reports (sum across many issues) — out of scope here, would need a new aggregation tool.
