<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

### Requirement: List wiki pages is bounded and resumable

`list_wiki_pages` SHALL accept required `owner` and `repo` string parameters and optional
`page` (1-indexed, default 1) and `limit` (default server page size) integer parameters.
It SHALL call `GET /repos/{owner}/{repo}/wiki/pages` via `DoJSONList` and return each
page's `title`, `page_name`, and `sub_url`, plus a `page` echo, a `has_next` boolean, and
an optional `total_count`.

Because the raw-HTTP helper does not expose response headers through its return value (so
`X-Total-Count` / `Link` are unreachable from the list itself), `has_next` SHALL use a
pagination-preserving next-page probe. The handler requests the current page with exactly
`limit`. If fewer rows arrive, `has_next=false`. If exactly `limit` rows arrive, it requests
`page+1` with the same `limit` and sets `has_next` according to whether the probe contains
any rows. It MUST NOT request `limit+1`: Forgejo derives page offsets from the page size,
and changing it causes later pages to skip rows. A repository with no wiki SHALL return an
empty list, not an error (the `404`→empty mapping is correct **only** for this list
endpoint).

`total_count` SHALL be populated from the current-page request's `X-Total-Count` response
header (available via the header-returning variant of the list helper) using
`pkg/forgejo.TotalCountPtr`, and SHALL be omitted (`omitempty`) — never `0` — when the
header is absent or unparsable. The next-page probe request's header, if any, SHALL NOT be
used for `total_count`; only the current page's header is authoritative.

Upstream computes this header from the raw wiki tree entry count, before it filters out
non-regular entries and names that are not valid wiki filenames. `total_count` is
therefore an UPPER BOUND on the number of listable pages, not an exact total: on a wiki
with subdirectories a client paging to exhaustion will list fewer pages than
`total_count`. The tool description and the README row SHALL say so, so a client does not
treat "listed < total_count" as a missing page.

#### Scenario: Repository with pages
- **WHEN** a client calls `list_wiki_pages` for a repo whose wiki has pages
- **THEN** the response SHALL list each page's `title`, `page_name`, and `sub_url`
- **AND** SHALL include the echoed `page` and a `has_next` boolean

#### Scenario: More pages than limit signals has_next
- **WHEN** a client calls `list_wiki_pages` with `limit=N` against a wiki with more than
  `N` pages
- **THEN** the response SHALL contain exactly `N` pages
- **AND** `has_next` SHALL be `true` (derived from a non-empty same-limit next-page probe)

#### Scenario: Pagination never changes the page size behind the caller's back
- **WHEN** a client requests page 1 with `limit=30` from a 32-page wiki
- **THEN** the current-page request and next-page probe SHALL both use `limit=30`
- **AND** a subsequent client request for page 2 with `limit=30` SHALL begin at item 31

#### Scenario: Repository without a wiki returns empty list
- **WHEN** a client calls `list_wiki_pages` for a repo with no wiki (upstream `404`)
- **THEN** the response SHALL be an empty page list
- **AND** SHALL NOT be an error

#### Scenario: Total count surfaced when Forgejo reports it
- **WHEN** the current page's upstream response carries `X-Total-Count: 9`
- **THEN** the response SHALL contain `total_count: 9`

#### Scenario: Total count omitted, not zeroed, when unavailable
- **WHEN** the current page's upstream response carries no `X-Total-Count` header
- **THEN** the response SHALL NOT contain a `total_count` key

#### Scenario: Total count is documented as an upper bound
- **WHEN** a client reads the `list_wiki_pages` tool description or its README row
- **THEN** both SHALL state that `total_count` counts raw upstream tree entries and may
  exceed the number of pages actually listed

### Requirement: Get wiki revisions is bounded, resumable, and errors on missing pages

`get_wiki_revisions` SHALL accept required `owner`, `repo`, `page_name` and optional
`page` / `limit` parameters, call `GET /repos/{owner}/{repo}/wiki/revisions/{pageName}`
via `DoJSON` (**not** `DoJSONList`), and return each revision's `sha`, `author`, and
`message`, plus the echoed `page`, a `has_next` boolean derived by the same exact-limit,
same-limit next-page probe as `list_wiki_pages`, and a `total_count`.

Unlike `list_wiki_pages`, `total_count` here SHALL be read from the response body's
`count` field, NOT from the `X-Total-Count` header. The two carry the same number, but the
body field is always present and cannot be stripped by an intermediary, and
`operation/wiki/resources.go` already treats it as the authoritative total for the same
endpoint. Because the body always carries it, `total_count` is a plain integer that is
always emitted, not an omitted-when-unknown field.

Unlike `list_wiki_pages`, a `404` here means the page does not exist (every existing wiki
page has at least one revision), so it SHALL be reported as a not-found error, **not** as
an empty list.

#### Scenario: Page with multiple revisions
- **WHEN** a client calls `get_wiki_revisions` for a page edited more than once
- **THEN** the response SHALL list each revision's `sha`, `author`, and `message`
- **AND** SHALL include the echoed `page` and `has_next`

#### Scenario: Revisions of a nonexistent page is an error
- **WHEN** a client calls `get_wiki_revisions` for a page that does not exist (upstream `404`)
- **THEN** the tool SHALL return a not-found error
- **AND** SHALL NOT return an empty revision list

#### Scenario: Total count comes from the response body
- **WHEN** the upstream response body reports `"count": 4`
- **THEN** the response SHALL contain `total_count: 4`

#### Scenario: The header does not override the body count
- **WHEN** the upstream response body reports `"count": 1` and also carries a conflicting
  `X-Total-Count: 99` header
- **THEN** the response SHALL contain `total_count: 1`
