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
- **AND** SHALL request one more item than the limit so truncation can be detected

#### Scenario: Over-cap collection is truncated with a sentinel
- **WHEN** more issues match than the effective limit
- **THEN** the payload SHALL contain at most `limit` rows
- **AND** SHALL set `truncated`
- **AND** SHALL name `list_repo_issues` as the tool that enumerates the remainder

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
