## 1. Tool surface

- [x] 1.1 Add `SearchIssuesToolName = "search_issues"` to the tool-name const block in `operation/issue/issue.go`
- [x] 1.2 Define `SearchIssuesTool` with `mcp.NewTool()`: required `owner`; optional `state` (default `open`), `type`, `labels`, `milestones`, `q`, `created_by`, `assigned_by`, `mentioned_by`; `page` (default 1) and `limit` (default 20) using `params.Page` / `params.Limit`
- [x] 1.3 Write the tool description so it states the owner scope, the response envelope shape, and that `has_next` true means a further page may exist — do not add a `team` parameter (design Decision 5)
- [x] 1.4 Register `SearchIssuesTool` with `SearchIssuesFn` in the existing `RegisterTool` function

## 2. Handler and envelope

- [x] 2.1 Define the response envelope type with JSON fields `issues`, `page`, `limit`, `count`, `has_next`
- [x] 2.2 Implement `SearchIssuesFn`: read arguments, reject empty `owner` with an error naming the parameter before any upstream call
- [x] 2.3 Clamp `page` and `limit` to the documented defaults when absent or below 1, and report the values actually used in the envelope
- [x] 2.4 Build `forgejo_sdk.ListIssueOption` with `Owner`, `State`, `Type`, `KeyWord`, `CreatedBy`, `AssignedBy`, `MentionedBy`, comma-split `Labels` / `Milestones`, and `ListOptions{Page, PageSize}`; leave omitted filters unset
- [x] 2.5 Call `client.ListIssues(opt)` and set `count` to the issue-slice length
- [x] 2.6 Derive `has_next` by a same-limit next-page probe: false when `count` is 0, otherwise re-call at `page+1` with the **same** `limit` and set `has_next` from whether the probe returned rows — never request `limit+1` (see design Decision 3)
- [x] 2.7 Return the envelope via `to.TextResult`

## 3. Repo-scoped validation fix

- [x] 3.1 In `ListRepoIssuesFn`, return an error naming `owner` when it is empty, before the SDK call
- [x] 3.2 In `ListRepoIssuesFn`, return an error naming `repo` when it is empty, directing the caller to `search_issues` for owner-wide listing; the message must not contain the phrase `path segment`

## 4. Tests

- [x] 4.1 Test that `search_issues` sends the `owner` query filter and returns issues from multiple repositories
- [x] 4.2 Test the envelope: `issues`, `page`, `limit`, `count`, `has_next` all present, `count` equals the issue-slice length
- [x] 4.3 Test `has_next` true when the probe returns rows, false when it does not, and false on an empty page
- [x] 4.4 Test default paging (1 / 20), a caller-raised `limit`, and clamping of values below 1
- [x] 4.5 Test that missing or empty `owner` errors without an upstream call
- [x] 4.6 Test filter pass-through for `state`, `labels`, and `q`; assert omitted filters carry no value in the query, except `type`, which is asserted present and empty
- [x] 4.7 Test `list_repo_issues` with an empty `repo`: error names `repo`, mentions `search_issues`, and omits `path segment`
- [x] 4.8 Test `list_repo_issues` with an empty `owner` errors, and that a well-formed call is unchanged
- [x] 4.9 Test that a `limit=100` request answered with 50 issues still reports `has_next` true when a further page exists (the clamping falsifier from battle-test.md)

## 5. Documentation

- [x] 5.1 Add a `search_issues` row to the README tool table naming `page` and `limit` as its bound parameters
- [x] 5.2 Answer the `docs/design/output-bounding.md` new-tool checklist in the PR description (bound parameters, sub-rule 2 row, resumption path, documentation locations)
- [x] 5.3 File an upstream issue against forgejo-sdk v3 v3.0.0 for `QueryEncode` sending `opt.MentionedBy` under the `team` key (`issue.go:150`), and link it from design Decision 5 — DRAFT ONLY per scope carve-out: text written to `upstream-sdk-issue.md`, not filed; design.md Decision 5 links it

## 6. Verification

- [x] 6.1 Run `make build` and the Go test suite; both pass
- [x] 6.2 Manually verify against a live instance that `search_issues` with `owner` set to an organization answers the #452 scenario — see `specs/search-issues/search-issues.demo.md`, "Scenario: Listing open issues for an organization" (live capture against `agentic-forges` on `git.b4mad.industries`, 15 issues across 2 repos)
- [x] 6.3 Manually verify that `list_repo_issues` with an empty `repo` returns the new validation error — see `specs/issue-listing-validation/issue-listing-validation.demo.md`, "Scenario: Empty repo argument" (live capture, error names `repo`, mentions `search_issues`, no `path segment` text)
