# Output Bounding for MCP Tools

Architectural invariant for `forgejo-mcp` tool design. Any new tool that returns data
proportional to repository or upstream state MUST satisfy the rules below before
landing.

The same invariant applies to data-proportional MCP resource content blocks: cap the block
explicitly and include a marker naming a range- or page-bounded tool that retrieves the remainder.

This paragraph used to open "Because `resources/read` has no caller-controlled range parameter".
That is no longer true and should not be cited as a reason to omit one. `resources/read` takes a
URI, and an RFC 6570 query expansion on the registered template (`…/labels{?page,limit}`) gives
the caller a range parameter through it. A collection resource is therefore held to the same
client-controlled bound as a tool — see the "Collection resource" requirement in
`mcp-resources-core`. The cap remains, as a ceiling rather than as the only bound.

## Why

MCP tool outputs flow into an LLM context window. A single unbounded response
(diff, file content, commit list, log stream) can blow the window or silently
truncate at the transport envelope. The caller then sees partial data with no
signal and no way to fetch the remainder. Issue
[#124](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/124) surfaced this on
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

**`total_count`:** Forgejo sets an `X-Total-Count` response header on paginated
list/search API endpoints, carrying the total number of matching rows across
every page (distinct from `count`, which is the row count on the current page
alone). Tools whose envelope already carries `page`/`limit`/`has_next` SHOULD
also surface this as a `total_count` field, parsed with the shared
`pkg/forgejo.TotalCount` / `TotalCountPtr` helpers rather than a bespoke
per-tool copy. When the header is absent or unparsable, OMIT the key — never
emit `total_count: 0`, which reads as "confirmed zero rows" rather than
"unknown." Today this covers `search_issues`, `list_repo_hooks`,
`list_wiki_pages`, and `get_wiki_revisions`.

A pagination envelope is not on its own a reason to add the field: the
endpoint has to actually send the header. Forgejo's handlers call
`SetTotalCountHeader` per endpoint, and several paginated ones do not —
`/repos/{owner}/{repo}/branch_protections`, `/issues/{index}/dependencies`
and `/issues/{index}/blocks` return no `X-Total-Count` at all. Their tools
therefore do NOT carry `total_count`: a key that is structurally always
absent is a promise the tool cannot keep, and a mock that injects the header
proves only the plumbing, not the availability. Check the upstream handler
(or a live response) before adding the field to a new tool.

Tools that still return a bare array with no pagination envelope at all
(most `list_*`/`search_*` tools — see the retrofit umbrella below) are out of
scope for `total_count` until they gain an envelope in the first place.

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

Tracked as the umbrella in [#124](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/124).
Sub-issues should target one tool at a time and reference this document.
