# release-management Specification

## Purpose

The release-management capability exposes Forgejo's release and release-attachment endpoints through MCP tools, so that LLMs and CLI callers can list, read, author, edit, and delete tag-anchored releases and their binary assets without leaving the MCP surface. All list endpoints are bounded per `docs/design/output-bounding.md`; the download tool reuses the 1 MiB inline cap and `BlobResourceContents` envelope shared with the issue/comment attachment domains.

## Requirements
### Requirement: List releases for a repository

The system SHALL expose an MCP tool `list_releases` that returns a paginated list of releases for a given repository. The tool SHALL accept `owner` (string, required), `repo` (string, required), `page` (number, default 1), `limit` (number, default 20), and `state` (string, default `all`, one of `all` | `draft` | `prerelease` | `published`). The tool SHALL satisfy `docs/design/output-bounding.md`: client-controlled bound, no silent truncation. The `state` filter SHALL be applied client-side after the SDK call: `draft` matches releases with `Draft=true`; `prerelease` matches `Draft=false && Prerelease=true`; `published` matches `Draft=false && Prerelease=false`; `all` returns every release. The tool description SHALL note that the filter runs after pagination, so result size may be smaller than `limit` even when more matches exist on later pages.

#### Scenario: Default pagination returns first page

- **WHEN** the caller invokes `list_releases` with only `owner` and `repo`
- **THEN** the system SHALL return page 1 with up to 20 releases as a JSON array of `Release` objects
- **AND** the system SHALL NOT cap the response with an undocumented byte limit

#### Scenario: Caller controls page size

- **WHEN** the caller invokes `list_releases` with `page=2` and `limit=5`
- **THEN** the system SHALL forward those values to `ListReleases` via `ListReleasesOptions.ListOptions{Page: 2, PageSize: 5}`
- **AND** the system SHALL return up to 5 releases from the second page

#### Scenario: Repository has no releases

- **WHEN** the caller invokes `list_releases` against a repo with zero releases
- **THEN** the system SHALL return an empty JSON array
- **AND** the system SHALL NOT return an error

#### Scenario: State filter excludes drafts

- **WHEN** the caller invokes `list_releases` with `state=published` against a repo containing a mix of drafts, prereleases, and published releases
- **THEN** the system SHALL return only releases where both `Draft=false` and `Prerelease=false`

#### Scenario: Invalid state value

- **WHEN** the caller invokes `list_releases` with `state=foo`
- **THEN** the system SHALL return an MCP error result identifying the invalid state value
- **AND** the system SHALL NOT call the SDK

### Requirement: Fetch a release by numeric ID

The system SHALL expose an MCP tool `get_release_by_id` that returns a single release by its numeric ID. The tool SHALL accept `owner` (string, required), `repo` (string, required), and `release_id` (number, required).

#### Scenario: Release exists

- **WHEN** the caller invokes `get_release_by_id` with a valid `release_id`
- **THEN** the system SHALL return the `Release` object JSON-encoded

#### Scenario: Release not found

- **WHEN** the caller invokes `get_release_by_id` with an unknown `release_id`
- **THEN** the system SHALL return an MCP error result wrapping the SDK's not-found error

### Requirement: Fetch a release by tag name

The system SHALL expose an MCP tool `get_release_by_tag` that returns a single release identified by its tag name. The tool SHALL accept `owner` (string, required), `repo` (string, required), and `tag` (string, required).

#### Scenario: Tag exists and has a release

- **WHEN** the caller invokes `get_release_by_tag` with an existing tag that has a release attached
- **THEN** the system SHALL return the `Release` object

#### Scenario: Tag has no release

- **WHEN** the caller invokes `get_release_by_tag` with a tag that has no release attached
- **THEN** the system SHALL return an MCP error result wrapping the SDK error

### Requirement: Fetch the latest non-draft, non-prerelease release

The system SHALL expose an MCP tool `get_latest_release` that returns the most recent published release. The tool SHALL accept `owner` (string, required) and `repo` (string, required).

#### Scenario: Repository has published releases

- **WHEN** the caller invokes `get_latest_release` against a repo with at least one published release
- **THEN** the system SHALL return the most recent non-draft, non-prerelease `Release`

#### Scenario: Repository has only drafts or prereleases

- **WHEN** the caller invokes `get_latest_release` against a repo whose only releases are drafts or prereleases
- **THEN** the system SHALL return an MCP error result wrapping the SDK's not-found error

### Requirement: Create a release

The system SHALL expose an MCP tool `create_release` that creates a new release. The tool SHALL accept `owner` (string, required), `repo` (string, required), `tag_name` (string, required), `target_commitish` (string, optional), `name` (string, optional), `body` (string, optional), `draft` (boolean, default false), and `prerelease` (boolean, default false). When the tag does not yet exist, `target_commitish` SHALL be passed to the SDK so Forgejo creates the tag.

#### Scenario: Create against an existing tag

- **WHEN** the caller invokes `create_release` with `tag_name` referencing an existing tag and no `target_commitish`
- **THEN** the system SHALL call `CreateRelease` with the provided fields and return the created `Release`

#### Scenario: Create a new tag at a specific commit

- **WHEN** the caller invokes `create_release` with a `tag_name` that does not yet exist and a `target_commitish` set to a commit SHA or branch
- **THEN** the system SHALL include `target_commitish` in `CreateReleaseOption` so Forgejo creates the tag
- **AND** the system SHALL return the created `Release`

#### Scenario: Create a draft release

- **WHEN** the caller invokes `create_release` with `draft=true`
- **THEN** the created release SHALL have `Draft=true` in the returned `Release` object

### Requirement: Edit an existing release

The system SHALL expose an MCP tool `edit_release` that updates fields of an existing release. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), and any subset of `tag_name`, `target_commitish`, `name`, `body`, `draft`, `prerelease` as optional fields.

#### Scenario: Update release body only

- **WHEN** the caller invokes `edit_release` with only `release_id` and a new `body`
- **THEN** the system SHALL call `EditRelease` with an `EditReleaseOption` whose `Note` is set and other fields left zero-valued
- **AND** the system SHALL return the updated `Release`

#### Scenario: Promote a draft to published

- **WHEN** the caller invokes `edit_release` with `draft=false` on a release that was previously `draft=true`
- **THEN** the returned `Release` SHALL have `Draft=false`

### Requirement: Delete a release by numeric ID

The system SHALL expose an MCP tool `delete_release` that deletes a release by its numeric ID. The tool SHALL accept `owner` (string, required), `repo` (string, required), and `release_id` (number, required). The tool description SHALL warn that the operation is destructive.

#### Scenario: Delete succeeds

- **WHEN** the caller invokes `delete_release` with a valid `release_id`
- **THEN** the system SHALL call `DeleteRelease` and return a success result

#### Scenario: Release not found

- **WHEN** the caller invokes `delete_release` with an unknown `release_id`
- **THEN** the system SHALL return an MCP error result wrapping the SDK error

### Requirement: Delete a release by tag name

The system SHALL expose an MCP tool `delete_release_by_tag` that deletes a release identified by its tag name. The tool SHALL accept `owner` (string, required), `repo` (string, required), and `tag` (string, required). The tool description SHALL warn that the operation is destructive and that callers must verify the tag.

#### Scenario: Delete by tag succeeds

- **WHEN** the caller invokes `delete_release_by_tag` with an existing tag
- **THEN** the system SHALL call `DeleteReleaseByTag` and return a success result

#### Scenario: Tag has no release

- **WHEN** the caller invokes `delete_release_by_tag` with a tag that has no release
- **THEN** the system SHALL return an MCP error result wrapping the SDK error

### Requirement: List attachments of a release

The system SHALL expose an MCP tool `list_release_attachments` that returns attachments for a given release. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), `page` (number, default 1), and `limit` (number, default 20). Because the SDK does not paginate server-side for this endpoint, the system SHALL slice the full response client-side; the tool description SHALL state this trade-off.

#### Scenario: Release has attachments

- **WHEN** the caller invokes `list_release_attachments` for a release with attachments
- **THEN** the system SHALL return up to `limit` attachments starting at offset `(page-1)*limit`

#### Scenario: Release has no attachments

- **WHEN** the caller invokes `list_release_attachments` for a release with zero attachments
- **THEN** the system SHALL return an empty JSON array

### Requirement: Fetch metadata for a single release attachment

The system SHALL expose an MCP tool `get_release_attachment` that returns metadata (including `browser_download_url`) for one attachment. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), and `attachment_id` (number, required).

#### Scenario: Attachment exists

- **WHEN** the caller invokes `get_release_attachment` with valid IDs
- **THEN** the system SHALL return the `Attachment` object including `browser_download_url`

#### Scenario: Attachment not found

- **WHEN** the caller invokes `get_release_attachment` with an unknown `attachment_id`
- **THEN** the system SHALL return an MCP error result wrapping the SDK error

### Requirement: Upload an attachment to a release

The system SHALL expose an MCP tool `create_release_attachment` that uploads a new attachment to a release. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), `content` (string, required, base64-encoded), `filename` (string, required), and `mime_type` (string, optional).

#### Scenario: Successful upload

- **WHEN** the caller invokes `create_release_attachment` with valid IDs, a valid base64 `content`, and a `filename`
- **THEN** the system SHALL decode `content` and pass an `io.Reader` plus `filename` to `CreateReleaseAttachment`
- **AND** the system SHALL return the new `Attachment`

#### Scenario: Invalid base64 content

- **WHEN** the caller invokes `create_release_attachment` with `content` that is not valid base64
- **THEN** the system SHALL return an MCP error result identifying the decoding failure
- **AND** the system SHALL NOT call the SDK

### Requirement: Download a release attachment

The system SHALL expose an MCP tool `download_release_attachment` that returns either the inline bytes of an attachment (base64-encoded as an MCP embedded resource) or the metadata plus `browser_download_url` when the file size meets or exceeds the inline cap. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), and `attachment_id` (number, required). The inline cap SHALL match the constant used by `download_issue_attachment` so behavior is consistent across attachment domains.

#### Scenario: File below inline cap

- **WHEN** the caller invokes `download_release_attachment` for an attachment whose size is below the inline cap
- **THEN** the system SHALL fetch the bytes from `browser_download_url` using the same Forgejo auth token and return the result with `Inline=true`, `BytesIncluded` set to the file size, and the bytes embedded as a base64 resource

#### Scenario: File at or above inline cap

- **WHEN** the caller invokes `download_release_attachment` for an attachment whose size meets or exceeds the inline cap
- **THEN** the system SHALL return the attachment metadata with `Inline=false`, `Reason` explaining the cap, and `BytesIncluded=0`
- **AND** the response SHALL include `browser_download_url` so the caller can fetch the file directly

#### Scenario: Attachment not found

- **WHEN** the caller invokes `download_release_attachment` with an unknown `attachment_id`
- **THEN** the system SHALL return an MCP error result wrapping the SDK error

### Requirement: Edit a release attachment

The system SHALL expose an MCP tool `edit_release_attachment` that renames an attachment. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), `attachment_id` (number, required), and `name` (string, required).

#### Scenario: Rename succeeds

- **WHEN** the caller invokes `edit_release_attachment` with a new `name`
- **THEN** the system SHALL call `EditReleaseAttachment` with `EditAttachmentOptions{Name: name}`
- **AND** the system SHALL return the updated `Attachment`

### Requirement: Delete a release attachment

The system SHALL expose an MCP tool `delete_release_attachment` that removes an attachment from a release. The tool SHALL accept `owner` (string, required), `repo` (string, required), `release_id` (number, required), and `attachment_id` (number, required). The tool description SHALL warn that the operation is destructive.

#### Scenario: Delete succeeds

- **WHEN** the caller invokes `delete_release_attachment` with valid IDs
- **THEN** the system SHALL call `DeleteReleaseAttachment` and return a success result

#### Scenario: Attachment not found

- **WHEN** the caller invokes `delete_release_attachment` with an unknown `attachment_id`
- **THEN** the system SHALL return an MCP error result wrapping the SDK error

