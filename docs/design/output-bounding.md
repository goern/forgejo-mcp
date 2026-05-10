# Output Bounding for MCP Tools

Architectural invariant for `forgejo-mcp` tool design. Any new tool that returns data
proportional to repository or upstream state MUST satisfy the rules below before
landing.

## Why

MCP tool outputs flow into an LLM context window. A single unbounded response
(diff, file content, commit list, log stream) can blow the window or silently
truncate at the transport envelope. The caller then sees partial data with no
signal and no way to fetch the remainder. Issue
[#124](https://codeberg.org/goern/forgejo-mcp/issues/124) surfaced this on
`get_pull_request_diff`, `get_file_content`, `list_pull_request_files`,
`list_pull_reviews`. This document generalizes the fix.

## The Rule

**Every tool output must be bounded by the caller, not the server. If the
output size depends on data rather than tool semantics, the tool MUST expose at
least one client-controlled bound AND a way to fetch the remainder.**

A tool whose output is bounded by its own semantics (e.g. `get_my_user_info`
returns one fixed-shape user object) is exempt. Everything else is in scope.

## Sub-rules

### 1. No silent truncation

A server-side envelope cap (e.g. 16 kB) without a caller-visible knob is a
trap: the caller receives partial data with no signal. Either:

- Expose the cap as a parameter the caller can raise / lower, or
- Replace the cap with proper paging / range params (sub-rule 2), or
- Return an explicit truncation marker (sub-rule 3) when the cap fires.

Never silently drop bytes.

### 2. Bound by domain shape, not bytes

Pick the natural unit for the data type. Byte ranges are a last-resort
fallback because they cut mid-token.

| Data type                 | Preferred bound                         | Parameter shape                              |
|---------------------------|-----------------------------------------|----------------------------------------------|
| Code / text file          | Line range                              | `start_line`, `end_line`                     |
| Diff (multi-file)         | Per-file slice (then optional paging)   | `file_path`; index via `list_*_files`        |
| List of entities          | Page + limit                            | `page`, `limit`                              |
| Log stream                | Tail / head + line or byte cap          | `tail_bytes` (or `tail_lines`) + marker      |
| Single binary blob        | Byte range fallback                     | `offset`, `max_bytes`                        |

Reuse parameter names across tools — agents learn one vocabulary, not many.

### 3. Always resumable

When the caller hits the bound, the response must carry a continuation signal
so a follow-up call can retrieve the rest. Acceptable shapes:

- **Paging**: response includes `page`, `total_count`, or `has_next`.
- **Range**: response includes the range actually returned (e.g. lines 1–500
  of 2300) so caller can issue the next slice.
- **Truncation marker**: a sentinel like `[truncated, N more bytes]` for log
  / byte-range tails when paging is unsuitable.
- **Index tool**: a sibling list tool (e.g. `list_pull_request_files`) so the
  caller can enumerate slices before requesting any.

"Got 4 KB of N" beats "got 4 KB."

## Documentation contract

Every bound parameter MUST appear in:

1. The tool's `mcp.NewTool()` description (per-parameter doc).
2. The README tool table.

An undocumented cap is the same trap as no cap.

## Checklist for new tools

When adding a tool in `operation/{domain}/`, answer in the PR description:

- [ ] Is output size bounded by the tool's own semantics (one fixed-shape
      object)? If yes, exempt — note this and skip the rest.
- [ ] If no: which bound parameter(s) does the tool expose?
- [ ] Which sub-rule 2 row matches the data type?
- [ ] How does the caller resume / fetch the remainder?
- [ ] Are bound parameters documented in the tool description and the README
      tool table?

If any answer is "none" or "unclear", the tool is not ready to merge.

## Retrofitting existing tools

Tracked as the umbrella in [#124](https://codeberg.org/goern/forgejo-mcp/issues/124).
Sub-issues should target one tool at a time and reference this document.
