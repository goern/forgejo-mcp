## ADDED Requirements

### Requirement: Per-file slicing of pull-request diff

The `get_pull_request_diff` MCP tool SHALL accept an optional string parameter `file_path`. When `file_path` is omitted or empty, the tool SHALL return the full unified diff (current behavior). When `file_path` is set, the tool SHALL return only the diff hunks belonging to that file, identified by the `diff --git a/<file_path> b/<file_path>` boundary in the unified diff. Match SHALL succeed on either the pre-rename path (`a/`) or post-rename path (`b/`).

If `file_path` is set and no matching file is found in the diff, the tool SHALL return an MCP error result identifying the missing path; the tool MUST NOT silently return the full diff or an empty string.

#### Scenario: Omitting file_path returns full diff

- **WHEN** the tool is called with `owner`, `repo`, `index`, and no `file_path`
- **THEN** the system SHALL return the unified diff for the entire pull request

#### Scenario: file_path selects a single file's hunks

- **WHEN** the tool is called with `file_path="cmd/main.go"` against a PR that touches `cmd/main.go` and other files
- **THEN** the system SHALL return only the `diff --git a/cmd/main.go b/cmd/main.go` section and its hunks, terminating at the next `diff --git` line or end of input

#### Scenario: Renamed file matches either side

- **WHEN** the diff contains `diff --git a/old.go b/new.go`
- **AND** the tool is called with either `file_path="old.go"` or `file_path="new.go"`
- **THEN** the system SHALL return the rename section in both cases

#### Scenario: Binary-file diff section

- **WHEN** `file_path` points at a binary file whose section is `Binary files a/img.png and b/img.png differ`
- **THEN** the system SHALL return that section unchanged

#### Scenario: file_path not in diff

- **WHEN** `file_path` does not appear in any `diff --git` header in the response
- **THEN** the system SHALL return an MCP error naming the missing `file_path`
- **AND** the system SHALL NOT return the full diff as a fallback
