## Why

An agent asked "which issues are open in my organization?" and got
`get issues list err: path segment [1] is empty`
([#452](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/452)). No tool on the
server can answer that question: every issue-listing tool is repo-scoped, so the agent
called `list_repo_issues` with an empty `repo`, which the SDK's path guard rejected with a
message that names neither the missing parameter nor the real gap.

Two defects sit behind one error string: a **missing capability** (no owner-wide issue
listing) and a **leaked internal error** (`list_repo_issues` forwards an empty `repo` to the
SDK instead of validating it). The upstream endpoint for the missing capability already
exists and is already reachable from the vendored SDK, so the gap is ours, not upstream's.

## What Changes

- **New `search_issues` tool** in `operation/issue/`, wrapping
  `client.ListIssues` (`GET /repos/issues/search`). Scopes results with the `owner` query
  filter, so `owner=my-org` returns issues across every repo in that org the token can see.
- **Filters carried over** from `list_repo_issues` so agents reuse one vocabulary:
  `state`, `type`, `labels`, `milestones`, plus `q` (keyword) and `created_by` /
  `assigned_by` / `mentioned_by`, which the search endpoint supports and the repo-scoped
  tool cannot offer.
- **Bounded + resumable response** per [output-bounding.md](../../../docs/design/output-bounding.md):
  `page` / `limit` parameters and a response envelope carrying `page`, `limit`, `count`, and
  `has_next`. `has_next` is derived by a same-limit next-page probe, not from a full page —
  the instance may clamp `limit` at `MAX_RESPONSE_ITEMS`, which would make a count-based rule
  falsely signal the end of the data. This matches the envelope and probe rule
  `list_wiki_pages` already follows (`openspec/specs/wiki-tools/spec.md:24-38`).
- **`list_repo_issues` validates `repo` and `owner`** before the SDK call, returning a
  message that names the empty parameter and points at `search_issues` for the owner-wide
  case. No behavior change for well-formed calls.
- **Documentation**: README tool table gains `search_issues` and its bound parameters.

Not in scope: the `team` filter. The vendored SDK's `ListIssueOption.QueryEncode` sends
`opt.MentionedBy` under the `team` key (forgejo-sdk v3 `issue.go:150`) — an upstream bug.
Exposing `team` would surface it as ours. Recorded in design.md, filed separately.

No breaking changes: no tool is removed and no existing signature changes.

## Capabilities

### New Capabilities
- `search-issues`: Owner-scoped (org or user) issue search across repositories, bounded by
  caller-controlled paging and resumable via an explicit continuation signal.
- `issue-listing-validation`: Input validation for repo-scoped issue listing, including the
  redirect to `search_issues` when the caller wants owner-wide results.

### Modified Capabilities
_None._ `list_repo_issues` gains input validation, but no spec in `openspec/specs/` currently
governs it — only `cli-mode/spec.md:48` mentions it, incidentally. So nothing is *modified*;
the validation fix is a delta of its own, filed under the new `issue-listing-validation`
capability rather than folded into `search-issues`, which is named for a different tool.

## Impact

| Area | Change |
|---|---|
| `operation/issue/issue.go` | New tool definition, handler, response envelope; `repo`/`owner` guard in `ListRepoIssuesFn` |
| `operation/issue/issue_test.go` | Coverage for owner filter, paging, `has_next`, and the validation error |
| `operation/operation.go` | No change — `issue.RegisterTool` already wired; new tool registers inside it |
| `README.md` | Tool table entry with bound parameters (required by the documentation contract) |
| Dependencies | None. `client.ListIssues` is already in the vendored `forgejo-sdk/forgejo/v3`. |
| Token scope | Read-only; same scope `list_repo_issues` needs. |

Two upstream constraints worth knowing before design, both detailed there:

- The SDK does not parse `X-Total-Count` for issue searches, so a `total_count` field cannot
  be populated honestly. `has_next` is probe-derived instead.
- Nothing clamps `limit` client-side, and the instance may cap it at `MAX_RESPONSE_ITEMS`
  (forgejo-sdk v3 `list_options.go:22`). This is why `has_next` cannot be inferred from a
  full page.
