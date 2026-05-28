## ADDED Requirements

### Requirement: Commit status resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/commit/{sha}/status` and MIME type `application/json` describing the combined CI status for a commit sha. The template description SHALL state that the status is pinned to the sha and is therefore cacheable for any given sha.

#### Scenario: Template appears in templates list with cacheability note
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}/status"`
- **AND** the description SHALL contain the substring "cacheable"

### Requirement: Status resource read returns aggregate state plus bounded per-context list

`resources/read` for a commit status URI SHALL return a primary `application/json` content block containing: `sha`, `state` (the aggregate state: `success`, `pending`, `failure`, `error`, or `unknown`), `total_count`, and a `statuses` array of per-context entries. Each entry SHALL include `context`, `state`, `target_url`, `description`, `creator_login`, `created_at`, `updated_at`. The `statuses` array SHALL be bounded at 30 entries; when truncated the response SHALL append a sentinel naming the `get_commit_statuses` tool and including the total count.

#### Scenario: Commit with few statuses returns aggregate plus all entries
- **WHEN** a client reads a status URI for a sha with `M ≤ 30` status contexts
- **THEN** the JSON `statuses` array SHALL contain all `M` entries
- **AND** no truncation sentinel SHALL appear
- **AND** the JSON `state` field SHALL reflect the aggregate

#### Scenario: Commit with many statuses truncates with sentinel
- **WHEN** a client reads a status URI for a sha with `M > 30` status contexts
- **THEN** the JSON `statuses` array SHALL contain exactly 30 entries
- **AND** the response SHALL include a sentinel that names the `get_commit_statuses` tool and includes the total count `M`

#### Scenario: Commit with no statuses returns unknown aggregate
- **WHEN** a client reads a status URI for a sha that has never been reported on
- **THEN** the JSON `state` field SHALL equal `"unknown"`
- **AND** the `statuses` array SHALL be empty

#### Scenario: Missing commit returns -32003
- **WHEN** a client reads a status URI for a sha that does not exist on the repo
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Status URI sha must be full 40-character hex

The `{sha}` URI parameter SHALL be validated as 40 hexadecimal characters. Shorter values SHALL fail with `-32602` before any upstream call is made.

#### Scenario: Short sha rejected
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/commit/df15877/status`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued
