<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

Forgejo sets an `X-Total-Count` response header on paginated list/search API
endpoints, carrying the total number of matching rows across every page —
distinct from `count`, which is the row count on the current page alone. None
of the tools that already build a pagination envelope (`page`/`limit`/`count`/
`has_next`) surface it, so answering "how many total?" means paging through
everything by hand and counting.

`search-issues/spec.md`'s own "Resumable response envelope" requirement
currently states the tool "SHALL NOT report a total result count, because the
upstream response does not carry one that the client parses" — true when that
requirement was written (the SDK's raw-list helpers didn't expose response
headers), but Forgejo does send the header, and `search_issues` goes through
`DoJSONList`, which can reach it. That line becomes false with this change and
needs to move with it.

## What Changes

- New shared helper `pkg/forgejo.TotalCount` / `TotalCountPtr`, parsing
  `X-Total-Count` from an `*http.Response`. Returns `(0, false)` /
  `(nil, false)` when the header is absent or unparsable — callers key
  `omitempty` off the boolean, not off a zero value, so a genuine `total_count:
  0` (confirmed empty result) is never confused with "unknown."
- Add `total_count` (`omitempty`) to the response envelope of the seven tools
  that already build one: `search_issues`, `list_wiki_pages`,
  `get_wiki_revisions`, `list_branch_protections`, `list_repo_hooks`,
  `list_issue_dependencies`, `list_issue_dependents`.
- `operation/issue/resources_list.go`'s pre-existing `headerTotalCount` now
  delegates to the shared helper instead of duplicating the parse.
- Document the convention once in `docs/design/output-bounding.md` (sub-rule
  3) rather than per-tool, and correct the now-stale "SHALL NOT report a
  total" line in `search-issues/spec.md`.

Tools that still return a bare array with no pagination envelope at all (most
other `list_*`/`search_*` tools — the retrofit umbrella is
[#124](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/124)) are
unaffected; adding an envelope to those is a separate, larger change. That
umbrella also covers `list_repo_labels`, which has the same "no envelope, hard
page cap" shape as a standalone report
([#289](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/289)) —
happy to fold that one in as a follow-up if wanted, since it's the same fix.

## Capabilities

### Modified Capabilities

- `search-issues`: envelope gains `total_count`; the "no total" prohibition in
  the Resumable response envelope requirement is removed.
- `wiki-tools`: `list_wiki_pages` and `get_wiki_revisions` envelopes gain
  `total_count`.
- `branch-protection`: `list_branch_protections` envelope gains `total_count`.
- `repo-webhook-tools`: `list_repo_hooks` envelope gains `total_count`.

Issue-dependency listing (`list_issue_dependencies` / `list_issue_dependents`)
also gains `total_count`, but has no `openspec/specs/` capability of its own
yet to carry a delta against — noted here rather than silently applied.

### New Capabilities

_None._ This is additive to existing envelopes only.

## Impact

| Area | Change |
|---|---|
| `pkg/forgejo/pagination.go` | New: `TotalCount`, `TotalCountPtr` |
| `pkg/forgejo/pagination_test.go` | New: header present/absent/unparsable cases |
| `operation/issue/issue.go`, `dependency.go`, `resources_list.go` | `total_count` on `search_issues`, `list_issue_dependencies`, `list_issue_dependents`; shared helper adopted |
| `operation/wiki/wiki.go` | `total_count` on `list_wiki_pages`, `get_wiki_revisions` |
| `operation/branchprotection/branchprotection.go` | `total_count` on `list_branch_protections` |
| `operation/hook/hook.go` | `total_count` on `list_repo_hooks` |
| `*_test.go` in each of the above | Coverage for header present / absent / unparsable |
| `README.md` | Tool table rows note the new field |
| `docs/design/output-bounding.md` | Convention documented once, sub-rule 3 |
| Dependencies | None |
| Token scope | No change — read-only, same scope each tool already needs |

No breaking changes: `total_count` is additive and omitted (never `0`) when
Forgejo doesn't send the header, so existing callers that don't look for the
key see no behavior change.
