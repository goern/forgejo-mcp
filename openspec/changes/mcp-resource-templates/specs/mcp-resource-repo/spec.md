## ADDED Requirements

### Requirement: Repo resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}` and MIME type `application/json` describing a Forgejo repository.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}"`
- **AND** the template's `mimeType` SHALL be `application/json`

### Requirement: Repo resource read

`resources/read` for a `forgejo://repo/{owner}/{repo}` URI SHALL return a JSON content block containing repository metadata: `full_name`, `owner_login`, `description`, `default_branch`, `visibility` (`public`/`private`/`internal`), `archived`, `fork`, `created_at`, `updated_at`, and counts (`open_issues_count`, `open_pr_count`, `stars_count`).

#### Scenario: Existing repo resolves
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp`
- **THEN** the response SHALL contain one `application/json` content block
- **AND** the JSON SHALL include `full_name = "goern/forgejo-mcp"` and a non-empty `default_branch`

#### Scenario: Missing repo returns -32003
- **WHEN** a client reads `forgejo://repo/goern/does-not-exist`
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

#### Scenario: Private repo without access returns -32002
- **WHEN** a client reads a `forgejo://repo/{owner}/{repo}` URI for a private repo
- **AND** the authenticated token lacks read permission
- **THEN** the server SHALL return MCP error code `-32002`

### Requirement: Repo resource does not embed lists

The repo resource SHALL NOT embed lists of issues, PRs, branches, files, or any other variable-size collection. Counts SHALL be returned as numeric fields only; enumeration is the job of existing list tools.

#### Scenario: Response carries counts but not items
- **WHEN** a client reads a repo URI for a repo with many open issues
- **THEN** the response JSON SHALL contain `open_issues_count` as a number
- **AND** the response SHALL NOT contain an array of issue objects
