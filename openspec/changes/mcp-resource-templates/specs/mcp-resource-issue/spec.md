## ADDED Requirements

### Requirement: Issue resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/issue/{index}` and MIME type `application/json` describing a single Forgejo issue.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/issue/{index}"`

### Requirement: Issue resource read returns metadata and recent comments

`resources/read` for an issue URI SHALL return a primary `application/json` content block with issue metadata (`index`, `title`, `state`, `body`, `labels`, `assignees`, `milestone`, `author`, `created_at`, `updated_at`, `closed_at`, `comments_count`, `html_url`) and a `text/markdown` sidecar containing the rendered body. The JSON SHALL include a `recent_comments` array of up to 30 most-recent comments, each with `id`, `author`, `created_at`, `body_excerpt` (first 280 characters).

#### Scenario: Issue with few comments includes all
- **WHEN** a client reads an issue URI for an issue with `M ≤ 30` comments
- **THEN** the `recent_comments` array SHALL contain all `M` comments
- **AND** no truncation sentinel SHALL appear

#### Scenario: Issue with many comments truncates
- **WHEN** a client reads an issue URI for an issue with `M > 30` comments
- **THEN** the `recent_comments` array SHALL contain exactly 30 comments
- **AND** the response SHALL include a sentinel that names the `list_issue_comments` tool and includes the total count `M`

#### Scenario: Missing issue returns -32003
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/issue/999999`
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Issue index parameter must be numeric

The `{index}` URI parameter SHALL be parsed as a positive integer. Non-numeric values SHALL fail with `-32602` before any upstream call is made.

#### Scenario: Non-numeric index rejected
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/issue/abc`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued
