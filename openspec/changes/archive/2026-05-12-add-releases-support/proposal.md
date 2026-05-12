## Why

Forgejo repositories expose releases (tag-based artifacts with notes, optional pre-release / draft flags, and binary attachments), but `forgejo-mcp` has no tools that read or mutate them. Codeberg issue [#127](https://codeberg.org/goern/forgejo-mcp/issues/127) asks for releases support so an LLM can draft release notes, summarize changes between releases, and (optionally) publish them through the same MCP server it already uses for issues and pull requests.

The upstream Forgejo SDK already exposes the full surface (`ListReleases`, `GetRelease`, `GetLatestRelease`, `GetReleaseByTag`, `CreateRelease`, `EditRelease`, `DeleteRelease`, `DeleteReleaseByTag`, plus `ListReleaseAttachments`, `GetReleaseAttachment`, `CreateReleaseAttachment`, `EditReleaseAttachment`, `DeleteReleaseAttachment`), so this is purely an MCP-side wrapping job.

## What Changes

- Add a new MCP tool domain `operation/release/` covering:
  - `list_releases` — paginated list of releases for a repo (supports `page`, `limit` per output-bounding rules)
  - `get_release_by_id` — single release by numeric ID
  - `get_release_by_tag` — single release by tag name
  - `get_latest_release` — latest non-draft, non-prerelease
  - `create_release` — new release from a tag (with `name`, `body`, `draft`, `prerelease`, `target_commitish`)
  - `edit_release` — update fields of an existing release
  - `delete_release` — delete by numeric ID
  - `delete_release_by_tag` — delete by tag name
- Add release-attachment tools (kept under the same domain because they hang off a release ID):
  - `list_release_attachments` — paginated list of assets for a release
  - `get_release_attachment` — single asset metadata
  - `download_release_attachment` — inline bytes below a size cap, else metadata + `browser_download_url`
  - `create_release_attachment` — upload an asset to a release
  - `edit_release_attachment` — rename / re-describe an asset
  - `delete_release_attachment` — remove an asset
- Register the new domain in `operation/operation.go`.
- Document the new tools in `DEVELOPER.md` and the README's tools table.
- Add a `Kind/Feature` label entry once the PR lands.

All list endpoints satisfy `docs/design/output-bounding.md`: client-controlled `page` + `limit`, no silent truncation, paging cursor surfaced in response.

## Capabilities

### New Capabilities

- `release-management`: CRUD over Forgejo releases and their attachments, exposed as MCP tools. Covers tag-anchored releases (draft / prerelease / published), associated notes, target commitish, and binary attachments uploaded against a release ID.

### Modified Capabilities

_None._ This is a greenfield domain; no existing spec changes shape or requirements.

## Impact

- **New code:** `operation/release/release.go` (and possibly `operation/release/attachment.go` for the asset sub-surface).
- **Touched code:** `operation/operation.go` (import + register call), `README.md` (tools table), `DEVELOPER.md` (mention release domain in examples list).
- **Dependencies:** none new — `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3 v3.0.0` already provides every method we need.
- **APIs surfaced:** 14 new MCP tools (8 release + 6 attachment).
- **Output bounding:** `list_releases` and `list_release_attachments` are the only data-proportional outputs; both use `page` + `limit`.
- **Permissions:** mutating tools (`create_*`, `edit_*`, `delete_*`) require a token with write access to the target repo; documented per-tool.
- **No breaking changes.**
