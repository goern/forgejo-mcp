# Demo: releases — list, fetch, mutate, attach

*2026-05-12T16:42:00Z*
<!-- showboat-id: 8c2d4a1f-release-management-demo-2026 -->

## What these tools do

Fourteen MCP tools cover the full release surface: tag-anchored release records and their binary attachments (`.tar.gz`, `.mcpb`, signatures, etc.). They close the gap reported in [#127](https://codeberg.org/goern/forgejo-mcp/issues/127), where agents could read commits and PRs but had no way to inspect or author releases through the MCP server.

Release tools:

- `list_releases` — Paginated listing with a client-side `state` filter (`all` | `draft` | `prerelease` | `published`).
- `get_release_by_id` — Fetch one release by numeric ID.
- `get_release_by_tag` — Fetch one release by tag name.
- `get_latest_release` — Latest non-draft, non-prerelease release.
- `create_release` — New release; pass `target_commitish` to create the tag at a SHA or branch.
- `edit_release` — Partial update; only fields the caller supplies are sent.
- `delete_release` — Delete by ID. Destructive.
- `delete_release_by_tag` — Delete by tag. Destructive — verify tag before calling.

Release-attachment tools (keyed by `release_id`, mirror the issue/comment attachment shape):

- `list_release_attachments` — Paginated list of release assets.
- `get_release_attachment` — Single asset metadata.
- `download_release_attachment` — Inline bytes below the 1 MiB cap; metadata + `browser_download_url` otherwise.
- `create_release_attachment` — Upload an asset from base64 content.
- `edit_release_attachment` — Rename an asset.
- `delete_release_attachment` — Remove an asset.

All `list_*` endpoints satisfy [`docs/design/output-bounding.md`](../docs/design/output-bounding.md): client-controlled `page` + `limit`, no silent truncation.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=...
make build
```

The `FORGEJO_ACCESS_TOKEN` only needs `repo` scope for read tools; the mutating tools (`create_*`, `edit_*`, `delete_*`) require `write:repository` on the target repo.

## Read-only walkthrough

Every example below runs against the public `goern/forgejo-mcp` repository. Read tools are safe to run as many times as you like.

### 1. List releases (default pagination)

```bash
./forgejo-mcp --cli list_releases \
  --args '{"owner":"goern","repo":"forgejo-mcp","limit":3}'
```

Compacted to the fields that matter:

```output
[
  {"id":9279378,"tag_name":"v2.22.0","draft":false,"prerelease":false,"asset_count":16},
  {"id":9162902,"tag_name":"v2.21.0","draft":false,"prerelease":false,"asset_count":16},
  {"id":9113435,"tag_name":"v2.20.0","draft":false,"prerelease":false,"asset_count":8}
]
```

The raw JSON envelope is the SDK's `Release` struct passed through unchanged: `id`, `tag_name`, `target_commitish`, `name`, `body` (release notes), `draft`, `prerelease`, `created_at`, `published_at`, `author`, and the `assets` array.

### 2. Latest published release

```bash
./forgejo-mcp --cli get_latest_release \
  --args '{"owner":"goern","repo":"forgejo-mcp"}'
```

```output
{
  "id": 9279378,
  "tag_name": "v2.22.0",
  "name": "v2.22.0",
  "draft": false,
  "prerelease": false,
  "html_url": "https://codeberg.org/goern/forgejo-mcp/releases/tag/v2.22.0",
  "asset_count": 16
}
```

Server-side filter — drafts and prereleases are not eligible. If the repository has no published release, the SDK returns 404 and the tool surfaces an error result.

### 3. Fetch a release by tag

```bash
./forgejo-mcp --cli get_release_by_tag \
  --args '{"owner":"goern","repo":"forgejo-mcp","tag":"v2.21.0"}'
```

```output
{
  "id": 9162902,
  "tag_name": "v2.21.0",
  "name": "v2.21.0",
  "html_url": "https://codeberg.org/goern/forgejo-mcp/releases/tag/v2.21.0",
  "created_at": "2026-05-07T23:03:37+02:00"
}
```

`get_release_by_id` is the symmetric variant for when you already have the numeric ID from a list call.

### 4. State filter (`published` excludes drafts and prereleases)

```bash
./forgejo-mcp --cli list_releases \
  --args '{"owner":"goern","repo":"forgejo-mcp","limit":2,"state":"published"}'
```

```output
[
  {"tag_name":"v2.22.0","draft":false,"prerelease":false},
  {"tag_name":"v2.21.0","draft":false,"prerelease":false}
]
```

Behaviour summary:

| `state` | Returned releases |
|---------|-------------------|
| `all` (default) | Every release on the page |
| `draft` | `draft=true` only |
| `prerelease` | `draft=false` and `prerelease=true` |
| `published` | `draft=false` and `prerelease=false` |

The filter runs *client-side after the SDK call*, so a single page may return fewer items than `limit` even when more matches exist on later pages. Bump `page` to walk the rest. Pass an unknown state and the tool fails fast without touching the SDK:

```bash
./forgejo-mcp --cli list_releases \
  --args '{"owner":"goern","repo":"forgejo-mcp","state":"foo"}'
```

```output
Error: tool execution failed: invalid state "foo": must be one of all|draft|prerelease|published
```

### 5. List release attachments (client-side slicing)

```bash
./forgejo-mcp --cli list_release_attachments \
  --args '{"owner":"goern","repo":"forgejo-mcp","release_id":9279378,"page":1,"limit":3}'
```

```output
[
  {"id":1291122,"name":"forgejo-mcp_2.22.0_darwin_amd64.mcpb","size":4440907,"browser_download_url":"https://codeberg.org/attachments/b7441902-81d4-4c92-96dc-1e67aa919200"},
  {"id":1291683,"name":"forgejo-mcp_2.22.0_darwin_amd64.mcpb","size":4440898,"browser_download_url":"https://codeberg.org/attachments/7dbdbf67-d16e-4657-85a7-dc59b0ac396a"},
  {"id":1291107,"name":"forgejo-mcp_2.22.0_darwin_amd64.tar.gz","size":4414324,"browser_download_url":"https://codeberg.org/attachments/a094e680-bb64-463f-afdb-ff86bafeccab"}
]
```

The Forgejo API does not paginate release attachments server-side. The tool fetches the full slice and then applies `[offset : offset+limit]` client-side. For releases with dozens of assets this is acceptable; the trade-off is documented in the tool description.

### 6. Download — over the 1 MiB inline cap

Release binaries are typically multi-megabyte, so the over-cap branch is the common path:

```bash
./forgejo-mcp --cli download_release_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","release_id":9279378,"attachment_id":1291107}'
```

```output
{
  "attachment": {
    "id": 1291107,
    "name": "forgejo-mcp_2.22.0_darwin_amd64.tar.gz",
    "size": 4414324,
    "browser_download_url": "https://codeberg.org/attachments/a094e680-bb64-463f-afdb-ff86bafeccab"
  },
  "inline": false,
  "reason": "size 4414324 bytes >= inline cap 1048576; fetch browser_download_url with Authorization: token <TOKEN>"
}
```

The agent never sees the bytes in the MCP envelope; it fetches them directly with the same token:

```bash
curl -H "Authorization: token $FORGEJO_ACCESS_TOKEN" \
  https://codeberg.org/attachments/a094e680-bb64-463f-afdb-ff86bafeccab \
  -o forgejo-mcp_2.22.0_darwin_amd64.tar.gz
```

For an attachment smaller than 1 MiB (e.g. a `SHA256SUMS` file, a signature, a short README), the response carries the bytes inline as an MCP `BlobResourceContents` alongside the metadata, identical to `download_issue_attachment`. See the [issue-attachments demo](issue-attachments.md#4-download--inline-file-is-under-1-mib) for the inline shape.

## Write lifecycle

Every write tool requires a token with `write:repository` scope on the target repo. The commands below show the parameter surface; run them only against a repo you intend to mutate.

### 7. Create a release (at an existing tag)

```bash
./forgejo-mcp --cli create_release \
  --args '{
    "owner":"goern","repo":"forgejo-mcp",
    "tag_name":"v0.0.1-demo",
    "name":"v0.0.1 demo",
    "body":"Demo release for the release-management walkthrough.",
    "draft":true
  }'
```

Omit `name` and the tool defaults it to `tag_name` so the SDK's "title is empty" validator does not trip.

### 8. Create a release at a specific commit (tag does not yet exist)

```bash
./forgejo-mcp --cli create_release \
  --args '{
    "owner":"goern","repo":"forgejo-mcp",
    "tag_name":"v0.0.2-demo",
    "target_commitish":"main",
    "name":"v0.0.2 demo",
    "prerelease":true
  }'
```

Forgejo creates the tag at the commit `target_commitish` resolves to (a branch name, tag, or SHA).

### 9. Promote a draft to published

```bash
./forgejo-mcp --cli edit_release \
  --args '{
    "owner":"goern","repo":"forgejo-mcp",
    "release_id":12345,
    "draft":false
  }'
```

`edit_release` is partial — only fields you pass are sent. The SDK's `EditReleaseOption` uses `*bool` for `draft`/`prerelease`, so omitting them leaves the existing values intact. Passing `draft:false` flips the flag; passing nothing leaves it untouched.

### 10. Upload an asset (base64-encoded content)

```bash
B64=$(base64 -w0 dist/forgejo-mcp.tar.gz)
./forgejo-mcp --cli create_release_attachment \
  --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\",\"release_id\":12345,\"content\":\"$B64\",\"filename\":\"forgejo-mcp.tar.gz\",\"mime_type\":\"application/gzip\"}"
```

The base64 step happens client-side; the tool decodes and streams the bytes through `multipart/form-data` to Forgejo. Invalid base64 is rejected before any SDK call:

```output
Error: tool execution failed: content must be base64-encoded: ...
```

### 11. Rename an asset

```bash
./forgejo-mcp --cli edit_release_attachment \
  --args '{
    "owner":"goern","repo":"forgejo-mcp",
    "release_id":12345,"attachment_id":67890,
    "name":"forgejo-mcp-amd64.tar.gz"
  }'
```

### 12. Delete a release (destructive)

```bash
./forgejo-mcp --cli delete_release \
  --args '{"owner":"goern","repo":"forgejo-mcp","release_id":12345}'
```

```output
{"Result":{"status":"deleted"}}
```

The tag itself is **kept**. To wipe the release and the tag, use `delete_release_by_tag`:

```bash
./forgejo-mcp --cli delete_release_by_tag \
  --args '{"owner":"goern","repo":"forgejo-mcp","tag":"v0.0.1-demo"}'
```

The tool description warns "destructive — verify tag before calling". Match twice, cut once.

## Autonomous workflow: draft release notes for the next tag

A typical end-to-end use case stitches the read tools together with the rest of the server:

1. `get_latest_release` → learn the current published version (e.g. `v2.22.0`) and grab its `published_at` timestamp.
2. `list_repo_commits` → fetch commits since that timestamp (covered in the bounded-responses demo).
3. Have the model summarise the commit list into Markdown release notes.
4. `create_release` with `tag_name=v2.23.0`, `target_commitish=main`, `draft=true`, `body=<generated notes>`.
5. Optionally `create_release_attachment` with built binaries (CI usually owns this step).
6. `edit_release` with `draft=false` once a human approves.

No file paths cross the MCP boundary at any step, so the same flow works identically over stdio, SSE, and streamable-HTTP transports.

## Why the cap, why the slice

- **1 MiB inline cap.** Identical to `download_issue_attachment` — keeps small assets ergonomic in-band, prevents context-window blowout for big ones, and the always-present `browser_download_url` lets agents fall through cleanly. The cap is the constant `MaxInlineDownloadBytes` in `pkg/forgejo/rawhttp.go`; one source of truth across all attachment domains.
- **Client-side state filter on `list_releases`.** Forgejo's REST API has no `state` query param. Filtering client-side means a page may return fewer items than `limit`; the tool description documents this so callers know to paginate, not retry.
- **Client-side slicing on `list_release_attachments`.** No SDK pagination is available. The full asset slice is fetched, then sliced. Acceptable while real-world releases have <50 assets; revisit if that changes.
