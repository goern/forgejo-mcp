## ADDED Requirements

### Requirement: Wiki tools use direct REST calls, not the SDK

The wiki tool handlers SHALL be implemented on top of the raw-HTTP helpers in
`pkg/forgejo` (`DoJSON` / `DoJSONList`) and SHALL NOT import or depend on any
`forgejo-sdk` wiki type or method. The `operation/wiki` package SHALL compile in the
default build (no `//go:build wiki` tag) and SHALL be registered from
`operation/operation.go` via `RegisterWikiTool`.

#### Scenario: Default build includes wiki tools
- **WHEN** the server is built with `make build` (no build tags)
- **THEN** the binary SHALL compile
- **AND** `tools/list` SHALL include `list_wiki_pages`, `get_wiki_page`,
  `get_wiki_revisions`, `create_wiki_page`, `update_wiki_page`, and `delete_wiki_page`

#### Scenario: No SDK wiki dependency
- **WHEN** the `operation/wiki` package is compiled
- **THEN** it SHALL NOT reference any `forgejo-sdk` wiki symbol

### Requirement: List wiki pages is bounded and resumable

`list_wiki_pages` SHALL accept required `owner` and `repo` string parameters and optional
`page` (1-indexed, default 1) and `limit` (default server page size) integer parameters.
It SHALL call `GET /repos/{owner}/{repo}/wiki/pages` via `DoJSONList` and return each
page's `title`, `page_name`, and `sub_url`, plus a `page` echo and a `has_next` boolean.

Because the raw-HTTP helper does not expose response headers (so `X-Total-Count` /
`Link` are unreachable), `has_next` SHALL be derived by **over-fetching one extra row**:
the handler requests `limit + 1` items, sets `has_next = (rows_received > limit)`, and
returns at most `limit` rows. This mirrors the existing embedded-list pattern in
`operation/issue/resources.go`. A repository with no wiki SHALL return an empty list, not
an error (the `404`→empty mapping is correct **only** for this list endpoint).

#### Scenario: Repository with pages
- **WHEN** a client calls `list_wiki_pages` for a repo whose wiki has pages
- **THEN** the response SHALL list each page's `title`, `page_name`, and `sub_url`
- **AND** SHALL include the echoed `page` and a `has_next` boolean

#### Scenario: More pages than limit signals has_next
- **WHEN** a client calls `list_wiki_pages` with `limit=N` against a wiki with more than
  `N` pages
- **THEN** the response SHALL contain exactly `N` pages
- **AND** `has_next` SHALL be `true` (derived from the `N+1`-th over-fetched row)

#### Scenario: Repository without a wiki returns empty list
- **WHEN** a client calls `list_wiki_pages` for a repo with no wiki (upstream `404`)
- **THEN** the response SHALL be an empty page list
- **AND** SHALL NOT be an error

### Requirement: Get wiki page decodes content and bounds by a deterministic line range

`get_wiki_page` SHALL accept required `owner`, `repo`, and `page_name` parameters and
optional 1-indexed inclusive `start_line` / `end_line` integers (the same parameter
names `get_file_content` exposes). It SHALL call
`GET /repos/{owner}/{repo}/wiki/page/{pageName}`, base64-decode the `content_base64`
field into UTF-8 markdown, and return the page `title`, decoded `content`, `commit_sha`,
and `total_lines`.

`total_lines` is a field **introduced by this tool** (`get_file_content` does not expose
it). To guarantee one line-range dialect across both tools, the line operations SHALL
call the **same** routine `get_file_content` uses (currently the unexported `sliceLines`
in `operation/repo/file.go`); since a cross-package call requires it, the routine SHALL be
exported in place (`repo.SliceLines`) or lifted to a shared package, with
`get_file_content` left calling the identical routine (no behavior change). It SHALL NOT
be copy-pasted (a divergent copy would re-break the single-dialect guarantee). `total_lines`
SHALL be defined as `len(strings.Split(decoded, "\n"))` — the count produced by that same
split. The split is on `"\n"`, so a trailing newline yields a final empty line that **is**
counted, and CRLF content keeps its `"\r"` on each line. When `start_line`/`end_line` are supplied,
`content` SHALL contain only that inclusive range (clamped to the file extent) and the
response SHALL echo the returned range. An inverted range after clamping SHALL be
reported as an error, matching `sliceLines`.

#### Scenario: Full page read reports total lines
- **WHEN** a client calls `get_wiki_page` without a line range
- **THEN** `content` SHALL be the full decoded markdown
- **AND** `total_lines` SHALL equal `len(strings.Split(content, "\n"))`

#### Scenario: Line range slices the content
- **WHEN** a client calls `get_wiki_page` with `start_line=1` and `end_line=5` on a page
  whose `total_lines` exceeds 5
- **THEN** `content` SHALL contain exactly lines 1–5 under the shared split rule
- **AND** the response SHALL report `total_lines` and echo the returned range so the
  caller can request the remainder

#### Scenario: Line counting is identical for CRLF and LF content
- **WHEN** `get_wiki_page` decodes a body `"a\r\nb\r\n"` and another body `"a\nb\n"`
- **THEN** both SHALL report the same `total_lines` (3, including the trailing empty line)
- **AND** the count SHALL be produced by the same split routine `get_file_content` uses

#### Scenario: Undecodable content is an explicit error
- **WHEN** the upstream `content_base64` field is not valid base64
- **THEN** the tool SHALL return an error
- **AND** SHALL NOT return the raw base64 string as content

### Requirement: Get wiki revisions is bounded, resumable, and errors on missing pages

`get_wiki_revisions` SHALL accept required `owner`, `repo`, `page_name` and optional
`page` / `limit` parameters, call `GET /repos/{owner}/{repo}/wiki/revisions/{pageName}`
via `DoJSON` (**not** `DoJSONList`), and return each revision's `sha`, `author`, and
`message`, plus the echoed `page` and a `has_next` boolean derived by the same `limit+1`
over-fetch as `list_wiki_pages`.

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

### Requirement: Create wiki page base64-encodes content

`create_wiki_page` SHALL accept required `owner`, `repo`, `title`, `content` and optional
`message` parameters. It SHALL base64-encode `content` into the `content_base64` field of
a `POST /repos/{owner}/{repo}/wiki/new` request. When `message` is empty the handler
SHALL supply a default commit message naming the page. The response SHALL surface the
server-assigned `page_name` (the server normalizes the title into a page name), so
callers can address the page in subsequent calls. Callers SHALL use the returned
`page_name` verbatim for `get`/`update`/`delete`; they MUST NOT derive it from `title`
(the server may transform it, e.g. spaces → dashes).

The behavior of `POST …/wiki/new` against an already-existing title is fixed by live
verification. If the upstream rejects a duplicate (e.g. `409`/`422`), `create_wiki_page`
SHALL surface a guided error naming `update_wiki_page` as the way to modify an existing
page (it SHALL NOT leak an opaque transport error). If the upstream instead overwrites,
`create_wiki_page`'s description SHALL warn that creating an existing title replaces it,
so callers do not silently clobber a page by confusing create with update.

#### Scenario: Create with default message
- **WHEN** a client calls `create_wiki_page` with a title and content but no message
- **THEN** the request body SHALL carry the base64-encoded content
- **AND** a non-empty default commit message naming the page SHALL be sent
- **AND** the response SHALL include the server-assigned `page_name`

#### Scenario: Create on an existing title is not a silent clobber
- **WHEN** a client calls `create_wiki_page` with a `title` that already exists
- **THEN** the tool SHALL either return a guided error pointing at `update_wiki_page`
  (if the upstream rejects the duplicate)
- **OR** report success only if the upstream's documented behavior is overwrite, in
  which case the destructive replacement SHALL be stated in the tool description

### Requirement: Update wiki page base64-encodes content and never silently renames

`update_wiki_page` SHALL accept required `owner`, `repo`, `page_name`, `content` and
optional `title` / `message` parameters. It SHALL base64-encode `content` and send a
`PATCH /repos/{owner}/{repo}/wiki/page/{pageName}`. When `title` is omitted, the update
SHALL NOT silently rename the page: the page addressed by `page_name` SHALL remain
reachable under the same name after the edit. The mechanism that preserves the name
(server-side retention vs. echoing the existing title) is fixed by live verification
(see the live-verification tasks).

`update_wiki_page` performs a read-modify-write whose last writer wins. Whether the
upstream `PATCH …/wiki/page/{pageName}` accepts an optimistic-concurrency precondition
(e.g. a base `commit_sha` / `If-Match`) is fixed by live verification. If the API accepts
such a field, `update_wiki_page` SHALL accept an optional `last_commit_sha` parameter
(sourced from `get_wiki_page`'s `commit_sha`) and forward it so a stale write is rejected
rather than silently clobbering a concurrent edit. If the API accepts no precondition, the
tool SHALL document the lost-update window in its description so callers know concurrent
edits overwrite without warning. (This mirrors `update_file`/`delete_file`, which require
a base `sha`.)

#### Scenario: Update without retitling preserves the page name
- **WHEN** a client calls `update_wiki_page` with new content but no `title`
- **THEN** the page SHALL remain reachable under its original `page_name` after the edit
- **AND** the request body SHALL carry the base64-encoded new content

#### Scenario: Stale update is rejected when the API supports a precondition
- **WHEN** the upstream accepts a base-commit precondition
- **AND** a client calls `update_wiki_page` with a `last_commit_sha` that is no longer current
- **THEN** the tool SHALL surface the upstream conflict as an error
- **AND** SHALL NOT silently overwrite the newer revision

### Requirement: Delete wiki page

`delete_wiki_page` SHALL accept required `owner`, `repo`, `page_name` parameters and send
`DELETE /repos/{owner}/{repo}/wiki/page/{pageName}`. Any `2xx` response SHALL be reported
as success (`204 No Content` is the canonical case; the handler MUST NOT hard-require
exactly `204`, since some instances return `200`).

#### Scenario: Delete existing page
- **WHEN** a client calls `delete_wiki_page` for an existing page
- **AND** the upstream returns a `2xx`
- **THEN** the tool SHALL report success

#### Scenario: Unauthorized write maps to error
- **WHEN** a wiki write tool is called with a token lacking write access (upstream `403`)
- **THEN** the tool SHALL return an error reflecting the unauthorized status

### Requirement: Wiki tools and their bounds are documented for discovery

Every wiki tool SHALL carry an `mcp.WithDescription` that states its purpose and names
its bound parameters. The README tool table SHALL include a `Wiki` group listing all six
tools, and `AGENTS.md` SHALL note the `operation/wiki/` package. Bound parameters
(`page`, `limit`, `start_line`, `end_line`) SHALL appear in both the tool description and
the README per `docs/design/output-bounding.md`.

To keep the surface legible to an agent picking a tool without trial-and-error, the
descriptions SHALL also state the discovery affordances surfaced by the agent-ergonomics
review:
- `AGENTS.md` SHALL state the naming convention `list_*` enumerates / `get_*` fetches one
  entity by name (so an agent does not guess `get_wiki_pages`).
- `create_wiki_page`'s description SHALL tell callers to address the page by the returned
  `page_name`, never by deriving it from `title`.
- `get_wiki_page`'s description SHALL state that `total_lines` is always returned (with or
  without a range), so an agent can call once unbounded to learn the size, then request a
  `start_line`/`end_line` window for a large page.

#### Scenario: README documents the bounds
- **WHEN** a reader views the README tool table
- **THEN** a `Wiki` group SHALL list all six wiki tools
- **AND** the bound parameters of `list_wiki_pages`, `get_wiki_page`, and
  `get_wiki_revisions` SHALL be named in their description column
