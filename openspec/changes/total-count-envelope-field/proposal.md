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
- Add `total_count` (`omitempty`) to the response envelope of the four tools
  that both build one and hit an endpoint that actually sends the header:
  `search_issues`, `list_repo_hooks`, `list_wiki_pages`, `get_wiki_revisions`.
- Deliberately NOT added to `list_branch_protections`,
  `list_issue_dependencies` and `list_issue_dependents`. Those tools have a
  pagination envelope, but the upstream Forgejo handlers behind
  `/branch_protections`, `/issues/{index}/dependencies` and
  `/issues/{index}/blocks` never call `SetTotalCountHeader`, so the header is
  structurally absent and the field could never be populated against a real
  server. A field that is always missing is a false promise in the tool
  description, and a test that injects the header into a mock proves the
  plumbing rather than the availability. If Forgejo starts sending the header
  for those endpoints, adding it back is a one-line change through the shared
  helper.
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
  `total_count`. Two wiki-specific notes: upstream computes the wiki-pages
  header from the raw tree entry count before filtering non-page entries, so
  `list_wiki_pages`'s `total_count` is documented as an upper bound rather
  than an exact total; and `get_wiki_revisions` reads its total from the
  response body's `count` field instead of the header — the same number, but
  always present, not strippable, and already the authoritative total for
  `operation/wiki/resources.go`.
- `repo-webhook-tools`: `list_repo_hooks` envelope gains `total_count`.

`branch-protection` and issue-dependency listing are unchanged: their upstream
endpoints do not emit `X-Total-Count` (see "What Changes"), so no delta is
raised against `branch-protection/spec.md`.

### New Capabilities

_None._ This is additive to existing envelopes only.

## Impact

| Area | Change |
|---|---|
| `pkg/forgejo/pagination.go` | New: `TotalCount`, `TotalCountPtr` |
| `pkg/forgejo/pagination_test.go` | New: header present/absent/unparsable cases |
| `operation/issue/issue.go`, `resources_list.go` | `total_count` on `search_issues`; shared helper adopted |
| `operation/wiki/wiki.go` | `total_count` on `list_wiki_pages`, `get_wiki_revisions` |
| `operation/hook/hook.go` | `total_count` on `list_repo_hooks` |
| `*_test.go` in each of the above | Coverage for header present / absent / unparsable |
| `README.md` | Tool table rows note the new field |
| `docs/design/output-bounding.md` | Convention documented once, sub-rule 3 |
| Dependencies | None |
| Token scope | No change — read-only, same scope each tool already needs |

No breaking changes: `total_count` is additive and omitted (never `0`) when
Forgejo doesn't send the header, so existing callers that don't look for the
key see no behavior change.
