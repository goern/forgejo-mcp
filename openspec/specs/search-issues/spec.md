# search-issues Specification

## Purpose
TBD - created by openspec-sync-specs from change add-search-issues-tool. Update Purpose after archive.
## Requirements
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

The server SHALL attempt to discover the instance's pagination ceiling
(`max_response_items`, via `GET /api/v1/settings/api`) and cache it per instance. When the
ceiling is known and the effective `limit` (after defaulting) exceeds it, the tool SHALL
return an error, before making any upstream search request, naming both the requested limit
and the ceiling. When the ceiling cannot be determined (endpoint unreachable, non-2xx, or the
instance predates the endpoint), the tool SHALL proceed without rejecting on `limit`; a failed
lookup SHALL be treated as "ceiling unknown", never as a ceiling of 0.

#### Scenario: Default bound applied

- **WHEN** `search_issues` is called without `page` or `limit`
- **THEN** at most 20 issues are returned
- **AND** the response reports `page` 1 and `limit` 20

#### Scenario: Caller raises the bound within the ceiling

- **WHEN** `search_issues` is called with `limit` 50 against an instance whose known ceiling is
      50 or higher, and an owner with more than 20 matching issues
- **THEN** up to 50 issues are returned in one response
- **AND** the response reports `limit` 50

#### Scenario: Caller exceeds a known ceiling

- **WHEN** `search_issues` is called with `limit` 100 against an instance whose known ceiling
      (`max_response_items`) is 50
- **THEN** the tool returns an error naming both `100` and `50`
- **AND** no upstream search request is made

#### Scenario: Ceiling unknown

- **WHEN** `search_issues` is called with `limit` 100 and the instance's `max_response_items`
      cannot be determined
- **THEN** the request proceeds without a client-side limit rejection

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

`limit` in the envelope SHALL report the value actually sent to the upstream search request
(the effective limit), sourced from the same value used to build that request, not
recomputed independently from the caller's raw input.

`has_next` derivation depends on whether the instance's pagination ceiling is known:

- When the ceiling is known, the Caller-controlled bound requirement guarantees the effective
  `limit` never exceeds it, so the request is always honored in full and `has_next` SHALL be
  `count == limit`.
- When the ceiling is unknown, `has_next` SHALL be derived by a pagination-preserving
  next-page probe: when `count` is 0, `has_next` SHALL be false; otherwise the handler SHALL
  request `page+1` at the **same** `limit` and set `has_next` according to whether the probe
  returned any issues. The probe MUST NOT request `limit+1`: Forgejo derives page offsets from
  the page size, and changing it causes later pages to skip rows. This is the rule
  `openspec/specs/wiki-tools/spec.md:31-38` establishes for `list_wiki_pages`, widened here
  from "probe on a full page" to "probe on any non-empty page", because `search_issues`
  advertises a `limit` callers can set above the (here, unconfirmed) ceiling.

The probe SHALL NOT run when the ceiling is known and the request was accepted: a short page
is unambiguous in that case (the limit was honored in full, so nothing was clamped), and
running it would cost an unnecessary upstream request.

The tool SHALL NOT report a total result count, because the upstream response does not carry
one that the client parses.

#### Scenario: More results available (ceiling unknown)

- **WHEN** the instance's ceiling is unknown, a call returns issues, and the next-page probe
      at the same `limit` returns at least one issue
- **THEN** `has_next` is true
- **AND** the caller can retrieve the remainder by re-issuing the call with `page` incremented

#### Scenario: Last page reached (ceiling unknown)

- **WHEN** the instance's ceiling is unknown and a call returns no issues, or the next-page
      probe at the same `limit` returns none
- **THEN** `has_next` is false

#### Scenario: Full page under a known ceiling still signals continuation

- **WHEN** the instance's ceiling is known, a call with an accepted `limit` returns exactly
      `limit` issues, and further matching issues exist
- **THEN** `has_next` is true
- **AND** no next-page probe request is made

#### Scenario: Clamped page still signals continuation (unknown-ceiling fallback only)

- **WHEN** the instance's ceiling is unknown, a call with `limit` 100 is answered with 50
      issues because the instance silently clamps at its own `MAX_RESPONSE_ITEMS`, and
      further matching issues exist
- **THEN** `has_next` is true, as determined by the next-page probe
- **AND** this scenario cannot occur when the ceiling is known, because a `limit` that would
      be clamped is rejected before the upstream call (see Caller-controlled bound)

#### Scenario: Envelope always present

- **WHEN** any successful `search_issues` call completes
- **THEN** the response object contains `issues`, `page`, `limit`, `count`, and `has_next`
- **AND** `count` equals the length of `issues`
- **AND** `limit` equals the value actually sent upstream

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
