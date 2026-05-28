## ADDED Requirements

### Requirement: Commit resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/commit/{sha}` and MIME type `application/json` describing a single Forgejo commit. The template description SHALL state that commits are immutable per sha.

#### Scenario: Template appears in templates list with immutability note
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}"`
- **AND** the description SHALL contain the word "immutable"

### Requirement: Commit resource read returns JSON plus markdown sidecar

`resources/read` for a `forgejo://repo/{owner}/{repo}/commit/{sha}` URI SHALL return two content blocks: a primary `application/json` block with commit metadata (`sha`, `author`, `committer`, `parents` array of parent shas, `tree_sha`, `message_subject`, `created_at`, `html_url`), and a secondary `text/markdown` block containing the full commit message body.

#### Scenario: Existing commit resolves with both blocks
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/commit/df15877777b177d9d237af26fab3c4a829ba61de`
- **AND** that sha exists on the repo
- **THEN** the response SHALL contain exactly two content blocks
- **AND** one block SHALL have `mimeType = "application/json"`
- **AND** the other SHALL have `mimeType = "text/markdown"`
- **AND** the JSON SHALL include the requested `sha` and a non-empty `tree_sha`

#### Scenario: Short sha rejected
- **WHEN** a client reads a URI with a `sha` shorter than 40 hex characters
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** the error message SHALL indicate that full 40-character shas are required

#### Scenario: Unknown sha returns -32003
- **WHEN** a client reads a commit URI whose sha is not present in the repo
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Commit resource carries no embedded list

The commit resource SHALL NOT embed file diffs, file lists, status entries, or related items. Diffs are served via existing tools; statuses are served via the separate status resource template.

#### Scenario: Response omits file diffs
- **WHEN** a client reads a commit URI for a commit touching many files
- **THEN** the JSON content SHALL NOT contain an array of file diffs or file paths
