<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# attachment-upload-sources Specification

## Purpose

The attachment-upload-sources capability defines how the three attachment
creation tools — `create_issue_attachment`, `create_comment_attachment`, and
`create_release_attachment` — obtain the bytes they upload. A caller supplies
either base64 `content` or a `file_path` on the host running `forgejo-mcp`.
Host file reads are a file-read primitive handed to whatever drives the MCP
client, so they are opt-in and optionally confined to one directory. Uploads
stream to Forgejo rather than buffering the whole body in memory.

## Requirements
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

### Requirement: Upload safety limits

The `create_issue_attachment`, `create_comment_attachment`, and `create_release_attachment` tools SHALL reject a base64 `content` argument larger than 90MiB before decoding it. `create_issue_attachment` and `create_comment_attachment` SHALL additionally bound their multipart upload call with a 45-second timeout reported as an "upload timed out after 45s" error rather than a bare context-deadline error or an unbounded hang; this timeout SHALL NOT apply to `create_release_attachment`, whose uploads may legitimately need more headroom than an issue/comment attachment.

A base64 decode failure on `content` SHALL report how many bytes this server received alongside the underlying decode error, so a caller can tell whether truncation happened before or after this process. This applies to all three tools, since all three accept base64 `content` and resolve it through the same shared code path.

These limits exist as defense in depth regardless of where a runaway or stuck request originates: they bound worst-case behavior for a huge or malformed `content` argument, or a stuck network path during upload, so a broken request always fails fast with a clear, actionable error.

#### Scenario: Oversized content is rejected before decoding

- **WHEN** a caller supplies `content` whose base64 length exceeds 90MiB, on any of the three attachment-creating tools
- **THEN** the tool SHALL reject the request with a size-limit error
- **AND** SHALL NOT attempt to decode the content first

#### Scenario: Decode failure reports the received length

- **WHEN** a caller supplies `content` that is not valid base64, on any of the three attachment-creating tools
- **THEN** the tool SHALL reject the request
- **AND** the error SHALL report the number of bytes this server received

#### Scenario: Upload call is bounded by a timeout

- **WHEN** the multipart upload to Forgejo for `create_issue_attachment` or `create_comment_attachment` does not complete within 45 seconds
- **THEN** the tool SHALL abort the call and return an "upload timed out after 45s" error
- **AND** SHALL NOT hang the caller past that bound

#### Scenario: Release attachment uploads are not subject to the 45s timeout

- **WHEN** a caller uploads a release attachment via `create_release_attachment`
- **THEN** the tool SHALL NOT apply the 45-second upload timeout
- **AND** SHALL still apply the 90MiB base64 `content` size cap and the received-length decode diagnostic

#### Scenario: Uploads under the limits are unaffected

- **WHEN** a caller supplies `content` under the size limit and, for `create_issue_attachment` / `create_comment_attachment`, Forgejo responds within the timeout
- **THEN** the tool SHALL upload and return normally, unaffected by either limit
