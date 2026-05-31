## ADDED Requirements

### Requirement: Wiki page resource template

The server SHALL register a resource template with URI
`forgejo://repo/{owner}/{repo}/wiki/{pageName}` and MIME type `application/json`
describing a single Forgejo wiki page, registered from `operation/operation.go` via
`RegisterWikiResource`. The template description SHALL state the URI form and that a
`text/markdown` content sidecar is returned, so it is self-describing in
`resources/templates/list`.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with
  `uriTemplate = "forgejo://repo/{owner}/{repo}/wiki/{pageName}"`

### Requirement: Wiki resource read returns decoded content and bounded revisions

`resources/read` for a wiki URI SHALL return a primary `application/json` content block
with `owner`, `repo`, `page_name`, `title`, `commit_sha`, and a `recent_revisions` array
of up to 30 most-recent revisions (each `sha`, `author`, `message`), plus a
`text/markdown` sidecar containing the base64-decoded page content. When the page has
more than 30 revisions the response SHALL include a truncation sentinel naming the
`get_wiki_revisions` tool and the total count.

#### Scenario: Page with few revisions includes all
- **WHEN** a client reads a wiki URI for a page with `M ≤ 30` revisions
- **THEN** `recent_revisions` SHALL contain all `M` revisions
- **AND** no truncation sentinel SHALL appear
- **AND** a `text/markdown` sidecar SHALL contain the decoded page content

#### Scenario: Page with many revisions truncates
- **WHEN** a client reads a wiki URI for a page with `M > 30` revisions
- **THEN** `recent_revisions` SHALL contain exactly 30 revisions
- **AND** the response SHALL include a sentinel naming the `get_wiki_revisions` tool and
  the total count `M`

#### Scenario: Missing page returns -32003
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/Nonexistent`
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Wiki URI page name is decoded and must be non-empty

`ParseWiki` SHALL accept `forgejo://repo/{owner}/{repo}/wiki/{pageName}`, requiring the
path to be exactly `[owner, repo, "wiki", pageName]`, URL-decoding `pageName`, and
rejecting an empty or whitespace-only page name with `-32602` before any upstream call.

#### Scenario: Empty page name rejected
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued

#### Scenario: URL-encoded page name is decoded
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/Getting%20Started`
- **THEN** the handler SHALL request the page named `Getting Started`

### Requirement: Wiki resource is documented for discovery

The README Resources table SHALL include a row for the
`forgejo://repo/{owner}/{repo}/wiki/{pageName}` template, and `AGENTS.md` SHALL note the
wiki resource alongside the wiki tools.

#### Scenario: README documents the wiki resource
- **WHEN** a reader views the README Resources table
- **THEN** it SHALL include a row for the wiki page resource template
