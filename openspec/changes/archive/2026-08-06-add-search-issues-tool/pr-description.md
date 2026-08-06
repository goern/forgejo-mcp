# PR description draft — add-search-issues-tool

## `docs/design/output-bounding.md` new-tool checklist

- **Is output size bounded by the tool's own semantics (one fixed-shape object)?**
  No. `search_issues` returns a list of issues drawn from every repository of an
  owner; size depends on data, not tool semantics. In scope for the bounding
  contract.
- **Which bound parameter(s) does the tool expose?**
  `page` (1-based, default 1) and `limit` (page size, default 20), matching
  `list_repo_issues`.
- **Which sub-rule 2 row matches the data type?**
  "List of entities" → `page` + `limit`.
- **How does the caller resume / fetch the remainder?**
  The response envelope `{issues, page, limit, count, has_next}` carries a
  `has_next` boolean. `has_next` is derived by a same-limit next-page probe
  (request `page+1` at the same `limit` and check whether it returns rows) —
  never `count == limit`, because an instance clamping `limit` at
  `MAX_RESPONSE_ITEMS` would otherwise make a clamped short page falsely
  report the end of the data (see design.md Decision 3). When `has_next` is
  true, the caller re-issues the call with `page` incremented.
- **Are bound parameters documented in the tool description and the README
  tool table?**
  Yes. `page` and `limit` each carry a `mcp.Description` in `SearchIssuesTool`
  (`operation/issue/issue.go`), and the README tool table's `search_issues`
  row names both.

## Summary for reviewers

- New tool `search_issues`: owner-scoped, cross-repository issue search via
  `GET /repos/issues/search` (`client.ListIssues`), filling the gap left by
  `list_repo_issues`, which requires a `repo` and cannot answer "which issues
  are open across owner X's repositories?" (#452).
- `list_repo_issues` (`ListRepoIssuesFn`) now validates `owner` and `repo` are
  non-empty before calling upstream, returning a purpose-written error instead
  of leaking the SDK's `path segment [1] is empty` message. The empty-`repo`
  error names `search_issues` as the tool to use instead.
- No existing tool's response shape changes; `search_issues` is additive.
