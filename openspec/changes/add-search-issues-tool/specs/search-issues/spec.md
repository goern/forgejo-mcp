## ADDED Requirements

### Requirement: Owner-scoped issue search tool

The server SHALL expose an MCP tool named `search_issues` that lists issues across every
repository belonging to a single owner (organization or user), without requiring a
repository name.

The tool SHALL require an `owner` parameter and SHALL NOT accept a `repo` parameter.
Results SHALL be restricted to repositories the authenticated token can read.

#### Scenario: Listing open issues for an organization

- **WHEN** `search_issues` is called with `owner` set to an organization the token can read
- **THEN** the response contains issues drawn from multiple repositories in that organization
- **AND** each issue identifies its source repository

#### Scenario: Owner omitted

- **WHEN** `search_issues` is called without `owner`, or with `owner` set to an empty string
- **THEN** the tool returns an error naming `owner` as the missing parameter
- **AND** no upstream request is made

#### Scenario: Owner has no visible issues

- **WHEN** `search_issues` is called with an owner whose repositories contain no issues
      matching the filters
- **THEN** the tool returns an empty result set with `has_next` false
- **AND** the call does not error

### Requirement: Caller-controlled bound

`search_issues` SHALL expose `page` and `limit` parameters that bound the number of issues
returned, per `docs/design/output-bounding.md`. The server SHALL NOT truncate the result set
by any bound the caller cannot control.

`page` SHALL default to 1 and `limit` SHALL default to 20, matching `list_repo_issues`.

#### Scenario: Default bound applied

- **WHEN** `search_issues` is called without `page` or `limit`
- **THEN** at most 20 issues are returned
- **AND** the response reports `page` 1 and `limit` 20

#### Scenario: Caller raises the bound

- **WHEN** `search_issues` is called with `limit` 100 against an owner with more than 20
      matching issues
- **THEN** up to 100 issues are returned in one response
- **AND** the count may be lower if the instance clamps `limit` at `MAX_RESPONSE_ITEMS`

#### Scenario: Caller pages forward

- **WHEN** `search_issues` is called with `page` 2 and `limit` 20
- **THEN** the response contains the second page of results
- **AND** the response reports `page` 2

#### Scenario: Non-positive paging values

- **WHEN** `search_issues` is called with `page` or `limit` less than 1
- **THEN** the tool substitutes the documented default for that parameter
- **AND** the response reports the value actually used

### Requirement: Resumable response envelope

`search_issues` SHALL return a JSON object, not a bare array, carrying the issues together
with a continuation signal: the `page` and `limit` actually used, the `count` of issues in
this response, and a boolean `has_next`.

`has_next` SHALL be derived by a pagination-preserving next-page probe. When `count` is 0,
`has_next` SHALL be false. Otherwise the handler SHALL request `page+1` at the **same**
`limit` and set `has_next` according to whether the probe returned any issues.

The probe MUST NOT request `limit+1`: Forgejo derives page offsets from the page size, and
changing it causes later pages to skip rows.

`has_next` SHALL NOT be inferred from `count == limit`. The instance may clamp `limit` at its
`MAX_RESPONSE_ITEMS` setting, so a clamped short page would falsely signal the end of the
data. This is the rule `openspec/specs/wiki-tools/spec.md:31-38` establishes for
`list_wiki_pages`, widened here from "probe on a full page" to "probe on any non-empty page"
for that reason — `list_wiki_pages` defaults `limit` to the server page size and never raises
it past the clamp, whereas `search_issues` advertises a `limit` callers can set above it.

The tool SHALL NOT report a total result count, because the upstream response does not carry
one that the client parses.

#### Scenario: More results available

- **WHEN** a call returns issues and the next-page probe at the same `limit` returns at least
      one issue
- **THEN** `has_next` is true
- **AND** the caller can retrieve the remainder by re-issuing the call with `page` incremented

#### Scenario: Last page reached

- **WHEN** a call returns no issues, or the next-page probe at the same `limit` returns none
- **THEN** `has_next` is false

#### Scenario: Clamped page still signals continuation

- **WHEN** a call with `limit` 100 is answered with 50 issues because the instance clamps at
      `MAX_RESPONSE_ITEMS`, and further matching issues exist
- **THEN** `has_next` is true

#### Scenario: Envelope always present

- **WHEN** any successful `search_issues` call completes
- **THEN** the response object contains `issues`, `page`, `limit`, `count`, and `has_next`
- **AND** `count` equals the length of `issues`

### Requirement: Search filters

`search_issues` SHALL accept the filter parameters `state`, `type`, `labels`, `milestones`,
`q`, `created_by`, `assigned_by`, and `mentioned_by`, and SHALL apply them together with the
owner scope. Filter parameters shared with `list_repo_issues` SHALL keep the same names,
value formats, and defaults.

`state` SHALL default to `open`. `labels` and `milestones` SHALL accept comma-separated
values. Comma-separated `labels` SHALL be matched with OR semantics: an issue carrying any
one of the listed labels is returned. `q` SHALL match against issue title, body, and
comments, not title alone.

Filters the caller omits SHALL be left unset on `ListIssueOption`. Note that the SDK emits
`type` unconditionally (forgejo-sdk v3 `issue.go:124`), so an omitted `type` reaches the wire
as an empty `type=`, which upstream treats as "no filter"; this is the only filter the tool
cannot suppress.

#### Scenario: Filtering by state

- **WHEN** `search_issues` is called with `state` `closed`
- **THEN** only closed issues are returned

#### Scenario: Filtering by label

- **WHEN** `search_issues` is called with `labels` set to two comma-separated label names
- **THEN** only issues carrying those labels are returned

#### Scenario: Keyword search

- **WHEN** `search_issues` is called with `q` set to a search term
- **THEN** only issues matching that term are returned

#### Scenario: Excluding pull requests

- **WHEN** `search_issues` is called with `type` `issues`
- **THEN** no pull requests appear in the results

### Requirement: Documented bounds

The `page` and `limit` parameters SHALL be documented in the tool's own MCP description and
in the README tool table, per the documentation contract in
`docs/design/output-bounding.md`.

#### Scenario: Tool description lists bounds

- **WHEN** a client reads the `search_issues` tool definition
- **THEN** `page` and `limit` each carry a description explaining their effect

#### Scenario: README lists the tool

- **WHEN** the README tool table is read
- **THEN** it contains a `search_issues` row naming its bound parameters
