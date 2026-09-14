<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## ADDED Requirements

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
