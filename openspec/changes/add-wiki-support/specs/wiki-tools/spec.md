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
It SHALL call `GET /repos/{owner}/{repo}/wiki/pages` and return each page's `title`,
`page_name`, and `sub_url`, plus a `page` echo and a `has_next` boolean so the caller can
fetch the next page. A repository with no wiki SHALL return an empty list, not an error.

#### Scenario: Repository with pages
- **WHEN** a client calls `list_wiki_pages` for a repo whose wiki has pages
- **THEN** the response SHALL list each page's `title`, `page_name`, and `sub_url`
- **AND** SHALL include the echoed `page` and a `has_next` boolean

#### Scenario: Repository without a wiki returns empty list
- **WHEN** a client calls `list_wiki_pages` for a repo with no wiki (upstream `404`)
- **THEN** the response SHALL be an empty page list
- **AND** SHALL NOT be an error

### Requirement: Get wiki page decodes content and bounds by line range

`get_wiki_page` SHALL accept required `owner`, `repo`, and `page_name` parameters and
optional 1-indexed inclusive `start_line` / `end_line` integers. It SHALL call
`GET /repos/{owner}/{repo}/wiki/page/{pageName}`, base64-decode the `content_base64`
field into UTF-8 markdown, and return the page `title`, decoded `content`, `commit_sha`,
and `total_lines`. When `start_line`/`end_line` are supplied, `content` SHALL contain
only that inclusive line range (clamped to the file extent) and the response SHALL echo
the returned range so the caller can request the remainder.

#### Scenario: Full page read reports total lines
- **WHEN** a client calls `get_wiki_page` without a line range
- **THEN** `content` SHALL be the full decoded markdown
- **AND** the response SHALL include `total_lines`

#### Scenario: Line range slices the content
- **WHEN** a client calls `get_wiki_page` with `start_line=1` and `end_line=5` on a page
  with more than five lines
- **THEN** `content` SHALL contain exactly lines 1–5
- **AND** the response SHALL report `total_lines` greater than 5 and echo the returned range

#### Scenario: Undecodable content is an explicit error
- **WHEN** the upstream `content_base64` field is not valid base64
- **THEN** the tool SHALL return an error
- **AND** SHALL NOT return the raw base64 string as content

### Requirement: Get wiki revisions is bounded and resumable

`get_wiki_revisions` SHALL accept required `owner`, `repo`, `page_name` and optional
`page` / `limit` parameters, call `GET /repos/{owner}/{repo}/wiki/revisions/{pageName}`,
and return each revision's `sha`, `author`, and `message`, plus the echoed `page` and a
`has_next` boolean.

#### Scenario: Page with multiple revisions
- **WHEN** a client calls `get_wiki_revisions` for a page edited more than once
- **THEN** the response SHALL list each revision's `sha`, `author`, and `message`
- **AND** SHALL include the echoed `page` and `has_next`

### Requirement: Create wiki page base64-encodes content

`create_wiki_page` SHALL accept required `owner`, `repo`, `title`, `content` and optional
`message` parameters. It SHALL base64-encode `content` into the `content_base64` field of
a `POST /repos/{owner}/{repo}/wiki/new` request. When `message` is empty the handler
SHALL supply a default commit message naming the page.

#### Scenario: Create with default message
- **WHEN** a client calls `create_wiki_page` with a title and content but no message
- **THEN** the request body SHALL carry the base64-encoded content
- **AND** a non-empty default commit message naming the page SHALL be sent

### Requirement: Update wiki page base64-encodes content

`update_wiki_page` SHALL accept required `owner`, `repo`, `page_name`, `content` and
optional `title` / `message` parameters. It SHALL base64-encode `content` and send a
`PATCH /repos/{owner}/{repo}/wiki/page/{pageName}`. When `title` is omitted the existing
page name SHALL be retained.

#### Scenario: Update without retitling
- **WHEN** a client calls `update_wiki_page` with new content but no `title`
- **THEN** the page SHALL keep its current name
- **AND** the request body SHALL carry the base64-encoded new content

### Requirement: Delete wiki page

`delete_wiki_page` SHALL accept required `owner`, `repo`, `page_name` parameters and send
`DELETE /repos/{owner}/{repo}/wiki/page/{pageName}`. A `204 No Content` response SHALL be
reported as success.

#### Scenario: Delete existing page
- **WHEN** a client calls `delete_wiki_page` for an existing page
- **AND** the upstream returns `204`
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

#### Scenario: README documents the bounds
- **WHEN** a reader views the README tool table
- **THEN** a `Wiki` group SHALL list all six wiki tools
- **AND** the bound parameters of `list_wiki_pages`, `get_wiki_page`, and
  `get_wiki_revisions` SHALL be named in their description column
