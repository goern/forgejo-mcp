## ADDED Requirements

### Requirement: Wiki page resource template

The server SHALL register a resource template with URI
`forgejo://repo/{owner}/{repo}/wiki/{pageName}` and MIME type `application/json`
describing a single Forgejo wiki page, registered from `operation/operation.go` via
`RegisterWikiResource`. The template description SHALL state the URI form, that a
`text/markdown` content sidecar is returned, and that `pageName` is the server-normalized
page name (percent-encode characters such as `/` and spaces), so it is self-describing in
`resources/templates/list`.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with
  `uriTemplate = "forgejo://repo/{owner}/{repo}/wiki/{pageName}"`

### Requirement: Wiki resource read returns decoded content and bounded revisions via two calls

`resources/read` for a wiki URI SHALL fetch the page and its revisions in two upstream
calls: the page via `GET …/wiki/page/{pageName}` (primary) and the revisions via
`GET …/wiki/revisions/{pageName}` (secondary). It SHALL return a primary
`application/json` block with `owner`, `repo`, `page_name`, `title`, `commit_sha`, and a
`recent_revisions` array of up to 30 most-recent revisions (each `sha`, `author`,
`message`), plus a `text/markdown` sidecar containing the base64-decoded page content.

`commit_sha` SHALL come from the page payload, not from `recent_revisions[0]`. Error
policy follows the existing two-call precedent in `operation/issue/resources.go`: only
the **primary** page call's status maps the read result (`403`→`-32002`, `404`→`-32003`);
a failure of the **secondary** revisions call SHALL degrade `recent_revisions` to an
empty array and still succeed the read. (The page GET having already succeeded, the page
is known to exist, so an empty `recent_revisions` here is a **degraded** read, not a
missing-page signal — distinct from the `get_wiki_revisions` tool, where a `404` is the
sole signal and correctly means page-not-found. The resource's page-first ordering is what
keeps the two consistent.)

When the page has more than 30 revisions the response SHALL include a truncation sentinel
naming the `get_wiki_revisions` tool and the total count.

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

#### Scenario: Revisions sub-call failure degrades to empty
- **WHEN** the primary page call succeeds but the secondary revisions call fails
- **THEN** the read SHALL still succeed with the page content and metadata
- **AND** `recent_revisions` SHALL be an empty array

#### Scenario: Missing page returns -32003
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/Nonexistent`
- **AND** the primary page call returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Wiki resource content is bounded, never unbounded

A wiki page body can be arbitrarily large, so the resource SHALL NOT embed it unbounded.
The `text/markdown` content sidecar SHALL be capped at `MaxInlineDownloadBytes` (the
existing 1 MiB cap in `pkg/forgejo`). When the decoded body exceeds the cap, the sidecar
SHALL be truncated at the cap boundary and SHALL carry a truncation marker that names the
`get_wiki_page` tool (which exposes `start_line`/`end_line`) as the way to retrieve the
remainder. No bytes SHALL be dropped silently. This satisfies `output-bounding.md`
sub-rules 1 (no silent truncation) and 3 (resumable) without a per-read knob the
`resources/read` protocol cannot carry.

#### Scenario: Oversized body is capped with a marker
- **WHEN** a client reads a wiki URI whose decoded body exceeds `MaxInlineDownloadBytes`
- **THEN** the `text/markdown` sidecar SHALL be truncated at the cap
- **AND** SHALL include a marker naming the `get_wiki_page` tool for the remainder

### Requirement: Wiki URI page name is parsed from the escaped path and must be non-empty

`ParseWiki` SHALL accept `forgejo://repo/{owner}/{repo}/wiki/{pageName}`, requiring the
path to be exactly `[owner, repo, "wiki", pageName]` (a strict 4-segment parser — it does
NOT greedily join trailing segments, so future `…/wiki/{page}/<subresource>` URIs cannot
be mis-routed). Because Go's `url.Parse` pre-splits a decoded `/`, `ParseWiki` SHALL read
the page-name segment from the **escaped** path (`u.EscapedPath()` / `RawPath`) and
`PathUnescape` only that segment — so a percent-encoded `%2F` in a sub-page name survives
into a single decoded page name. `ParseWiki` MUST NOT reuse the shared
`splitPath(u.Path)` for the page name: `u.Path` is already decoded, so it would re-split
`%2F`. An empty or whitespace-only page name SHALL be rejected with `-32602` before any
upstream call. A literal (unencoded) `/` that produces extra segments SHALL return a
guided `-32602` error telling the caller to percent-encode `/` as `%2F`.

Note: the parser keeping `%2F` as one segment is unconditional Go behavior, but whether
the **upstream** resolves an encoded slash is server-dependent — some stacks (Apache
`AllowEncodedSlashes` off, proxies that decode-then-resplit) reject `%2F` in a path. Task
5.7 verifies this live; where the upstream does not resolve `%2F`, resource-URI access to
slash-bearing sub-pages is documented as unsupported and the fallback is the
`get_wiki_page` tool (`page_name="Guides/Setup"`), with the resource returning a normal
not-found rather than silently misleading.

#### Scenario: Empty page name rejected
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued

#### Scenario: URL-encoded space is decoded
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/Getting%20Started`
- **THEN** the handler SHALL request the page named `Getting Started`

#### Scenario: Encoded slash yields a single sub-page name
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/Guides%2FSetup`
- **THEN** `ParseWiki` SHALL decode the page name to `Guides/Setup` (one segment)
- **AND** SHALL NOT split it into separate path segments

#### Scenario: Literal slash returns a guided error
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/Guides/Setup` with a
  literal slash
- **THEN** the server SHALL return `-32602` with a message instructing the caller to
  percent-encode `/` as `%2F`

### Requirement: Wiki resource is documented for discovery

The README Resources table SHALL include a row for the
`forgejo://repo/{owner}/{repo}/wiki/{pageName}` template (noting the percent-encoding rule
for `/` and spaces in sub-page names), and `AGENTS.md` SHALL note the wiki resource
alongside the wiki tools.

#### Scenario: README documents the wiki resource
- **WHEN** a reader views the README Resources table
- **THEN** it SHALL include a row for the wiki page resource template
