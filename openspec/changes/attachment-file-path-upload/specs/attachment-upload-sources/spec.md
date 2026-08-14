<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## ADDED Requirements

### Requirement: Attachment upload source

The `create_issue_attachment`, `create_comment_attachment`, and `create_release_attachment` tools SHALL accept optional string arguments `content` and `file_path` and SHALL require exactly one argument key to be present. `content` SHALL contain base64-encoded bytes. `file_path` SHALL refer to a regular file on the host running `forgejo-mcp`.

#### Scenario: Upload base64 content

- **WHEN** a caller supplies `content` and `filename` without `file_path`
- **THEN** the tool SHALL decode and upload those bytes

#### Scenario: Upload a host file

- **WHEN** a caller supplies `file_path` without `content`
- **THEN** the tool SHALL upload that file and use its basename by default
- **AND** an explicit `filename` SHALL override the basename

#### Scenario: Relative host path

- **WHEN** `file_path` is relative
- **THEN** the tool SHALL resolve it from the server process working directory

#### Scenario: Invalid source selection

- **WHEN** both source keys or neither source key are present
- **THEN** the tool SHALL reject the request before contacting Forgejo

#### Scenario: Empty base64 file

- **WHEN** `content` is present with an empty string and `file_path` is absent
- **THEN** the tool SHALL upload a zero-byte file

### Requirement: Host file reads are opt-in

Reading upload content off the host filesystem SHALL be disabled unless the operator sets `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` to a truthy value (`1`, `true`, `yes`, `on`). When the operator also sets `FORGEJO_MCP_UPLOAD_ROOT`, a `file_path` SHALL resolve inside that directory after symlink resolution or be rejected. Base64 `content` uploads SHALL be unaffected by both variables.

#### Scenario: Gate closed by default

- **WHEN** a caller supplies `file_path` and `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` is unset or not truthy
- **THEN** the tool SHALL reject the request before opening the file
- **AND** the error SHALL name the variable that enables the feature

#### Scenario: Path outside the configured root

- **WHEN** `FORGEJO_MCP_UPLOAD_ROOT` is set and `file_path` resolves outside it, whether by an absolute path, `..`, or a symlink
- **THEN** the tool SHALL reject the request before uploading anything

#### Scenario: Base64 upload with the gate closed

- **WHEN** a caller supplies `content` and `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` is unset
- **THEN** the tool SHALL upload those bytes as usual

### Requirement: Streaming multipart upload

Attachment creation SHALL stream multipart bodies to Forgejo without buffering
the complete multipart body in memory.

#### Scenario: Large path upload

- **WHEN** a caller uploads a large regular file through `file_path`
- **THEN** memory use SHALL NOT grow by the complete file size
- **AND** the tool SHALL return the Forgejo attachment response
