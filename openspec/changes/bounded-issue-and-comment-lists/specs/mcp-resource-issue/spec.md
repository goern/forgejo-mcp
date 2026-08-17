<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## ADDED Requirements

### Requirement: Issue collection resource

The server SHALL expose a bounded collection resource
`forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}` returning issues as
**rows**: index, title, state, author login, label names, assignee logins, milestone
title, comment count, created and updated timestamps, due date when set, and a flag
marking pull requests.

Rows SHALL NOT contain issue bodies. A caller that wants a body reads
`forgejo://repo/{owner}/{repo}/issue/{index}`, which already provides it. This
separation is the point of the resource: the cost of asking "what is open?" must scale
with the number of issues, not with how much prose they contain.

`state` SHALL accept `open`, `closed`, or `all` and SHALL default to `open`. An
unrecognised `state` SHALL fall back to `open` rather than failing the read, since a
resource URI carries no schema validation on the client side. `labels` SHALL be a
comma-separated list, whitespace-trimmed around each entry.

The collection SHALL bound its items with `EmbeddedListCap` and SHALL append the
standard truncation sentinel naming `list_repo_issues`, which remains the unbounded
enumeration path and SHALL NOT be removed or altered.

The upstream page size SHALL equal the effective limit exactly. Upstream computes the
offset as `(page - 1) * pageSize`, so requesting more rows per page than are shown
would make page N+1 begin past the last row of page N and leave the rows in between
unreachable from any page a client can request. "More items exist" SHALL therefore be
taken from the response's `Link … rel="next"` header rather than from an over-fetched
extra row, and the sentinel's total SHALL come from `X-Total-Count` when the server
reports it, and SHALL be omitted rather than guessed when it does not.

#### Scenario: Rows carry no bodies
- **WHEN** a client reads the issue collection for a repo whose issues have long bodies
- **THEN** the payload SHALL contain each issue's index, title, state, labels and
  comment count
- **AND** SHALL NOT contain any issue body

#### Scenario: State defaults to open
- **WHEN** a client reads the collection with no `state` parameter
- **THEN** the upstream request SHALL be made with `state=open`

#### Scenario: Unrecognised state falls back rather than failing
- **WHEN** a client reads the collection with `state=banana`
- **THEN** the read SHALL succeed
- **AND** the upstream request SHALL be made with `state=open`

#### Scenario: Filters and bounds reach the API
- **WHEN** a client reads the collection with `state=all&labels=a,%20b&page=2&limit=5`
- **THEN** the upstream request SHALL carry `state=all`, both trimmed label names and
  `page=2`
- **AND** SHALL request exactly `limit` items, not more

#### Scenario: Successive pages cover every row exactly once
- **WHEN** a client walks pages 1..N of a collection larger than `N * limit`
- **THEN** the union of the rows returned SHALL be the first `N * limit` matching rows
- **AND** no matching row SHALL be skipped at a page boundary
- **AND** no row SHALL appear on two pages

#### Scenario: Over-cap collection is truncated with a sentinel
- **WHEN** more issues match than the effective limit
- **THEN** the payload SHALL contain at most `limit` rows
- **AND** SHALL set `truncated`
- **AND** SHALL name `list_repo_issues` as the tool that enumerates the remainder

#### Scenario: Truncation is detected from the response, not the row count
- **WHEN** a page is exactly `limit` rows and the response carries `rel="next"`
- **THEN** the payload SHALL set `truncated` and emit the sentinel
- **AND** the sentinel SHALL report the `X-Total-Count` total when the server sends one
- **AND** SHALL omit the total rather than guessing when it does not

#### Scenario: Final page is not reported as truncated
- **WHEN** a page's response carries no `rel="next"` link
- **THEN** the payload SHALL NOT set `truncated` and SHALL emit no sentinel

#### Scenario: Limit above the ceiling is clamped
- **WHEN** a client requests a `limit` greater than `EmbeddedListCap`
- **THEN** the effective limit SHALL be `EmbeddedListCap`

### Requirement: Comment thread resource

The server SHALL expose a bounded collection resource
`forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}` returning comments
with **full bodies**, where `kind` is `issue` or `pr`. PR comments SHALL resolve through
the Forgejo issue-comment API, consistent with the existing single-comment resource.

This complements, and does not replace, the excerpted `recent_comments` embedded in the
single-issue and single-PR resources: those answer "is this thread worth reading", this
one answers "read it".

The collection SHALL bound its items with `EmbeddedListCap`, SHALL support `page` and
`limit`, and SHALL append the standard truncation sentinel naming `list_issue_comments`.

Forgejo's issue-comment endpoint ignores `page` and `limit` and returns the entire
thread, sending `X-Total-Count` but no `Link` header, so this resource SHALL apply the
page offset itself whenever the upstream returns more rows than were requested, and
SHALL take the total from the rows in hand. When the upstream returns no more rows than
were requested it SHALL treat them as already paged and SHALL NOT apply the offset a
second time, so the resource stays correct if Forgejo begins honouring the bounds.

The bound here is on the **number of comments, not on bytes**: a page of full bodies
can still be a large payload. This is deliberate and is the bound
`docs/design/output-bounding.md` sub-rule 2 prescribes for a list of entities, and it
matches what the `list_issue_comments` tool already does — stated here so a later
reader does not re-derive it and mistake it for an oversight. A caller who needs a
byte bound instead of a count bound reads the excerpted `recent_comments` on the
single-issue resource.

#### Scenario: Full bodies are preserved
- **WHEN** a client reads a thread containing a comment longer than the 200-character
  excerpt used by the single-issue resource
- **THEN** the payload SHALL contain that comment's complete body

#### Scenario: PR kind is accepted
- **WHEN** a client reads `…/pr/{index}/comments`
- **THEN** the read SHALL succeed and the payload SHALL echo `kind` as `pr`

#### Scenario: Invalid kind is rejected
- **WHEN** a client reads a comments URI whose kind is neither `issue` nor `pr`
- **THEN** the read SHALL fail with an invalid-parameters error

#### Scenario: Pages cover the thread when upstream ignores the bounds
- **WHEN** a client walks the pages of a thread longer than `limit` and the upstream
  returns the whole thread for every request
- **THEN** each page SHALL contain that page's slice of the thread and no other
- **AND** the union of the pages SHALL be the whole thread, with no comment skipped
  or repeated
- **AND** a page past the end of the thread SHALL be empty and SHALL NOT be truncated

#### Scenario: Upstream-paged rows are not offset a second time
- **WHEN** the upstream returns no more rows than were requested for page 2
- **THEN** the payload SHALL contain those rows unchanged

### Requirement: Collection templates are registered with their query expansions

Each collection resource template SHALL be registered with its RFC 6570 query expansion included in the template string — `…/issues{?state,labels,page,limit}` and `…/{kind}/{index}/comments{?page,limit}` — and not with the bare path alone.

The MCP server matches a `resources/read` against an anchored regular expression built
from the registered template string. A template registered without its query expansion
therefore matches only the bare URI, and every read carrying `state`, `labels`, `page`
or `limit` fails with "resource not found" before any handler runs — making the bound
and filter parameters, which are the entire purpose of these resources, unreachable.
This is invisible to tests that call the handlers directly.

#### Scenario: Bare collection URI resolves
- **WHEN** a client issues `resources/read` for `forgejo://repo/{owner}/{repo}/issues`
- **THEN** the read SHALL be routed to the issue-collection handler

#### Scenario: Query-bearing collection URI resolves
- **WHEN** a client issues `resources/read` for
  `forgejo://repo/{owner}/{repo}/issues?state=all&labels=a,b&page=2&limit=5`
- **THEN** the read SHALL be routed to the issue-collection handler
- **AND** the effective state, labels, page and limit SHALL be those from the query

#### Scenario: Query-bearing comment-thread URI resolves
- **WHEN** a client issues `resources/read` for
  `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments?page=2&limit=5`
- **THEN** the read SHALL be routed to the comment-thread handler
- **AND** the effective page and limit SHALL be those from the query

#### Scenario: Over-cap thread is truncated with a sentinel
- **WHEN** more comments exist than the effective limit
- **THEN** the payload SHALL contain at most `limit` comments
- **AND** SHALL set `truncated`
- **AND** SHALL name `list_issue_comments` as the tool that enumerates the remainder

### Requirement: Collection URIs do not collide with singular resources

Collection URI parsing SHALL reject the neighbouring singular forms rather than
resolving them: `…/issues` SHALL NOT accept `…/issue/{index}`, and
`…/{kind}/{index}/comments` SHALL NOT accept `…/{kind}/{index}/comment/{id}`. A
near-miss URI is the realistic client error, and resolving it to a different resource
would return a plausible payload for a question the client did not ask.

#### Scenario: Singular issue URI is not a collection
- **WHEN** `forgejo://repo/{owner}/{repo}/issue/42` is parsed as an issue collection
- **THEN** parsing SHALL fail with an invalid-parameters error

#### Scenario: Singular comment URI is not a thread
- **WHEN** `forgejo://repo/{owner}/{repo}/issue/42/comment/9` is parsed as a comment
  thread
- **THEN** parsing SHALL fail with an invalid-parameters error
