<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

Two everyday reads have no cheap shape today, and both cost payload proportional to
the prose in a repository rather than to the question being asked.

**Triage.** "What is open, and which of it is labelled X?" is answered by
`list_repo_issues`, which returns whole issue objects — every body, plus a full user
object per issue. Measured against one real repository: 53 open issues rendered as
index/title/labels is about 5 KB; the same 53 as full issue objects is two orders of
magnitude larger, dominated by bodies the caller did not ask for. For an MCP client
with a context budget, that difference decides whether the question can be asked at
all.

**Reading a thread.** `forgejo://repo/{owner}/{repo}/issue/{index}` embeds recent
comments, but excerpts each at 200 characters — exactly right for deciding whether a
thread is worth reading, and unusable for actually reading it. The alternatives are
one resource read per comment id, or the `list_issue_comments` tool with its
per-comment user objects.

The resource layer already has the right answer for both shapes — bounded collection
resources with row-shaped payloads, established by `…/labels{?page,limit}` — so this
change applies that existing pattern to issues and comment threads rather than
inventing anything.

## What Changes

- **New resource** `forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}` —
  a bounded list of issue **rows**: index, title, state, author login, label names,
  assignee logins, milestone title, comment count, created/updated timestamps, due
  date, and an `is_pull` flag. **No bodies.** `state` accepts `open|closed|all` and
  defaults to `open`; an unrecognised value falls back to `open` rather than erroring.
  `labels` is comma-separated and whitespace-trimmed.
- **New resource** `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}` —
  a bounded comment thread with **full bodies**, `kind ∈ {issue, pr}` (PR comments use
  the same Forgejo issue-comment API, matching the existing single-comment resource).
- Both use `operation/resource.Bounded` with `EmbeddedListCap`, so truncation is never
  silent and the sentinel names the existing tool (`list_repo_issues` /
  `list_issue_comments`) that enumerates the remainder.
- Two new URI parsers, `resource.ParseIssues` and `resource.ParseIssueComments`, which
  reject the neighbouring singular forms (`…/issue/{index}`, `…/comment/{id}`) rather
  than resolving a near-miss URI to the wrong resource.

No tool is changed or removed; both list tools remain the unbounded enumeration path.

## Capabilities

### New Capabilities

- **Issue collection resource**: row-shaped, filterable, bounded issue listing for
  triage without paying for bodies.
- **Comment thread resource**: full-body, paged comment reading between "excerpted
  preview" and "one read per comment id".

### Modified Capabilities

None. Additive only.

## Impact

- **Code**: `operation/issue/resources_list.go` (new), two template registrations in
  `operation/issue/resources.go`, two parsers in `operation/resource/parse.go`.
- **Tests**: `operation/issue/resources_list_test.go` (22 cases: row mapping and the
  no-body property, default and explicit filters reaching the API, unknown-state
  fallback, multi-page coverage against a mock with real offset semantics,
  header-driven truncation with and without `X-Total-Count`, an untruncated final
  page, cap clamping, error mapping, PR kind, full-body preservation, bad kind,
  non-numeric index) and `operation/resource/parse_issuelist_test.go` (8 cases,
  including both near-miss URIs). Mutation-checked: ignoring the caller's `state`,
  restoring the `limit+1` page size, forcing "more exists" to false, and dropping the
  `X-Total-Count` read each fail the suite.
- **APIs**: one GET per read, the same call the corresponding list tool already makes.
- **Dependencies**: none added.
- **Risk**: low — read-only, additive, no tool surface changed.

## Output-bounding checklist

Per `docs/design/output-bounding.md`:

- **Is output size bounded by the resource's own semantics?** No — it depends on how
  many issues or comments exist.
- **Which bound parameters?** `page` and `limit` for both; `state` and `labels` further
  narrow the issue list. `limit` clamps to `EmbeddedListCap` (30).
- **Which sub-rule 2 row?** Item-count bounding for a list of entities.
- **How does the caller resume?** `page` for the next slice; on truncation the payload
  carries `truncated`, `list_tool` and the standard sentinel naming the tool that
  enumerates the remainder.
- **Documented?** Resource table rows in `README.md` and `AGENTS.md`, plus the bound
  parameters and ceiling in each template description.
