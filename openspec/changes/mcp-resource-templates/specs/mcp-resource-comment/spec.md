## ADDED Requirements

### Requirement: Comment resource template discriminates on parent kind

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` and MIME type `application/json`, where `{kind}` is constrained to the literal values `issue` or `pr`.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}"`

#### Scenario: Unknown kind rejected before upstream call
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/wiki/1/comment/42` (kind `wiki`)
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued

### Requirement: Comment resource read

`resources/read` for a comment URI SHALL return a primary `application/json` content block with: `id`, `kind`, `parent_index`, `author`, `body`, `created_at`, `updated_at`, `html_url`. A `text/markdown` sidecar SHALL carry the rendered comment body.

#### Scenario: Existing issue comment resolves
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/issue/42/comment/1234` for an existing comment
- **THEN** the response SHALL include a primary `application/json` block
- **AND** the JSON `kind` field SHALL equal `"issue"`
- **AND** the JSON `parent_index` field SHALL equal `42`
- **AND** the JSON `id` field SHALL equal `1234`

#### Scenario: Missing comment returns -32003
- **WHEN** a client reads a comment URI whose id does not exist
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Comment index and id parameters must be numeric

Both the `{index}` parent identifier and the `{id}` comment identifier SHALL be parsed as positive integers. Non-numeric values SHALL fail with `-32602` before any upstream call is made.

#### Scenario: Non-numeric id rejected
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/pr/1/comment/xyz`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued
