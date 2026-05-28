## ADDED Requirements

### Requirement: Pull request resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/pr/{index}` and MIME type `application/json` describing a single Forgejo pull request.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/pr/{index}"`

### Requirement: PR resource read returns metadata, refs, mergeability, and recent activity

`resources/read` for a PR URI SHALL return a primary `application/json` content block containing: `index`, `title`, `state`, `body`, `labels`, `assignees`, `requested_reviewers`, `author`, `created_at`, `updated_at`, `closed_at`, `merged_at`, `merge_commit_sha`, `mergeable`, `head` (`ref`, `sha`, `repo_full_name`), `base` (`ref`, `sha`, `repo_full_name`), `comments_count`, `review_count`, `html_url`. A `text/markdown` sidecar SHALL carry the rendered PR body. The JSON SHALL include `recent_comments` and `recent_reviews` arrays, each bounded at 30 most-recent entries.

#### Scenario: Open PR resolves with head and base refs
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/pr/123` for an open PR
- **THEN** the response SHALL include a primary `application/json` block
- **AND** the JSON SHALL include `head.sha` and `base.sha` as non-empty 40-character hex strings
- **AND** the JSON SHALL include a boolean `mergeable` field

#### Scenario: PR with many comments truncates with sentinel naming list_issue_comments
- **WHEN** a client reads a PR URI for a PR with `M > 30` comments
- **THEN** the `recent_comments` array SHALL contain exactly 30 entries
- **AND** the response SHALL include a sentinel that names the `list_issue_comments` tool and includes total count `M`

#### Scenario: PR with many reviews truncates with sentinel naming list_pull_reviews
- **WHEN** a client reads a PR URI for a PR with `R > 30` reviews
- **THEN** the `recent_reviews` array SHALL contain exactly 30 entries
- **AND** the response SHALL include a sentinel that names the `list_pull_reviews` tool and includes total count `R`

#### Scenario: Missing PR returns -32003
- **WHEN** a client reads a PR URI whose index does not exist on the repo
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: PR index parameter must be numeric

The `{index}` URI parameter SHALL be parsed as a positive integer. Non-numeric values SHALL fail with `-32602` before any upstream call is made.

#### Scenario: Non-numeric index rejected
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/pr/foo`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** no upstream HTTP request SHALL be issued
