## Context

`forgejo-mcp` already wraps issues, pull requests, repo content, orgs, search, tracking, and issue/PR attachments. Releases are a separate Forgejo concept (tag-anchored artifacts with notes + binary assets) that the MCP server does not yet surface. Codeberg issue #127 asks for tools that let an LLM read release history and (optionally) author release notes.

Current state per repo inspection:

- Existing pattern: one Go package per domain under `operation/`, each with its own `RegisterTool(s *server.MCPServer)` entry point, wired into `operation/operation.go`.
- Tool naming is snake_case verb-noun (`list_repo_issues`, `get_issue_by_index`, `create_issue_comment`, ...). Shared parameter descriptions live in `operation/params/params.go`.
- The Forgejo SDK v3 (`codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3 v3.0.0`) already exposes the full release + release-attachment surface — no fallback to raw HTTP needed (unlike the existing `operation/attachment/` package, which exists precisely because the issue-attachment SDK gap forced raw HTTP).
- `docs/design/output-bounding.md` is the architectural invariant for any data-proportional response.

Stakeholders: end users running `forgejo-mcp` against Codeberg or any Forgejo instance; future tool authors who will read this domain as a template.

## Goals / Non-Goals

**Goals:**

- Cover all release CRUD plus release-attachment CRUD as MCP tools, with one consistent naming convention.
- Reuse existing patterns (params package, response helpers in `pkg/to`, the `pkg/forgejo` singleton client) so the new domain reads like the others.
- Satisfy `docs/design/output-bounding.md` for both list endpoints.
- Land the proposal without breaking any existing tool name or behavior.

**Non-Goals:**

- Generating release notes from commit ranges or PR titles. That is downstream LLM work that consumes these tools, not part of the MCP surface itself.
- A separate "release notes" tool that auto-summarizes. Keep the MCP layer thin.
- Migrating any existing tool. This is purely additive.
- Supporting non-Forgejo APIs.
- Bumping the SDK version. v3.0.0 already has everything.

## Decisions

### D1. New package `operation/release/`

Mirror the per-domain convention. One package, one `RegisterTool` entry point, wired into `operation/operation.go` between `RegisterAttachmentTool` and end of list.

Alternatives considered:

- Fold release tools into `operation/repo/`. Rejected: releases are a distinct domain with their own SDK surface; users (and the CLI domain grouping) benefit from a separate group, the same way `issue` and `pull` are split despite both being repo-scoped.
- Single-file vs multi-file package. Start single-file `release.go`. If the asset sub-surface grows past ~300 lines, split into `release.go` + `attachment.go` as a later refactor. The `operation/attachment/` package shows the single-file form scales fine to ~12 tools.

### D2. Tool naming

Match existing repo conventions exactly:

| MCP tool | SDK method |
|----------|------------|
| `list_releases` | `ListReleases` |
| `get_release_by_id` | `GetRelease` |
| `get_release_by_tag` | `GetReleaseByTag` |
| `get_latest_release` | `GetLatestRelease` |
| `create_release` | `CreateRelease` |
| `edit_release` | `EditRelease` |
| `delete_release` | `DeleteRelease` |
| `delete_release_by_tag` | `DeleteReleaseByTag` |
| `list_release_attachments` | `ListReleaseAttachments` |
| `get_release_attachment` | `GetReleaseAttachment` |
| `create_release_attachment` | `CreateReleaseAttachment` |
| `edit_release_attachment` | `EditReleaseAttachment` |
| `delete_release_attachment` | `DeleteReleaseAttachment` |

`get_release_by_id` is named explicitly (not `get_release`) because `get_release_by_tag` exists as a peer — the symmetric pair avoids ambiguity for the LLM. Same rationale for `delete_release` vs `delete_release_by_tag`: keep the by-tag variants as their own tools rather than overloading one tool with two ID modes; this keeps each tool's parameter schema unambiguous.

Alternatives considered:

- A single `get_release` tool with mutually-exclusive `id` and `tag` params. Rejected: makes the schema fuzzy and forces conditional validation in the handler. Separate tools are simpler and match how the SDK splits them.

### D3. Identifier types

Release IDs are `int64` per SDK (`func (c *Client) GetRelease(owner, repo string, id int64)`). Expose to MCP as `mcp.WithNumber("release_id", ...)` — MCP-Go normalizes JSON numbers to `float64`, so the handler must cast through `int64(...)` with a range guard, same as existing `index`/`attachment_id` handlers do.

### D4. List pagination

Both list endpoints (`list_releases`, `list_release_attachments`) accept `page` (default 1) and `limit` (default 20), matching `list_repo_issues` and similar. SDK's `ListReleasesOptions` carries `ListOptions{Page, PageSize}`; pass them through. No server-side truncation, no envelope cap — caller bounds the output per `docs/design/output-bounding.md`.

`list_release_attachments` has no SDK list options struct (it returns the full slice), so we slice client-side after fetching. Document this in the tool description: "limited by Forgejo's API response shape; large attachment sets are still fully fetched server-side before slicing."

In addition, `list_releases` exposes a `state` filter (`all` | `draft` | `prerelease` | `published`, default `all`). The SDK's `ListReleasesOptions` does not carry this field, so the filter is applied client-side after the SDK call by inspecting each release's `Draft` and `Prerelease` flags. Because pagination is server-side and filtering is client-side, `limit` is interpreted as "max items returned for this page after filtering" — the tool description documents that callers may see fewer items than `limit` even when more matches exist on later pages.

### D5. Asset upload + download payload shape

Reuse the base64 inline shape from `operation/attachment/`:

- `create_release_attachment` takes `content` (base64) + `filename` + optional `mime_type`. Handler decodes, builds a multipart body, and POSTs via the SDK's `CreateReleaseAttachment(... io.Reader, filename string)` method — which fortunately *does* exist in the SDK (unlike the issue/comment attachment endpoints).
- `download_release_attachment` mirrors `download_issue_attachment`: handler fetches metadata first, then either inlines the bytes (when the size is below the inline cap) as a base64-encoded embedded resource, or returns metadata-only plus `browser_download_url` so the caller fetches the file themselves with the same auth token. The inline cap matches the existing constant used by `operation/attachment/`. Returned shape reuses the `downloadResult` schema (or an equivalent struct in the release package) so the response is consistent across attachment domains.

### D6. Output formatting

Use `pkg/to` helpers exactly as other domains do (`to.SuccessResult`, `to.ErrorResult`). Return raw SDK structs (`*forgejo_sdk.Release`, `*forgejo_sdk.Attachment`) JSON-encoded — no custom shape — so the contract evolves with the SDK and stays consistent with the rest of the server.

### D7. Permissions and destructive operations

Mutating tools (`create_*`, `edit_*`, `delete_*`) require a Forgejo token with `write:repository` scope on the target repo. Document this in each tool's description. The MCP layer adds *no* extra confirm prompt for deletes — callers (LLM + harness) own that gate, same as everywhere else in this server.

## Risks / Trade-offs

- **[Asset upload size]** Base64-inline upload buffers the whole payload in memory. → Mitigation: matches the existing issue-attachment path's profile; if it becomes a problem, switch to a streaming param (file-path or URL) in a follow-up — symmetric with whatever fix issue-attachments adopt.
- **[`list_release_attachments` has no SDK pagination]** Server slices after a full fetch. → Mitigation: document the trade-off in the tool description; most releases have <50 assets, so the wasted work is negligible. Revisit if real-world releases prove otherwise.
- **[Tag-based delete is destructive and easy to mis-fire]** A wrong `tag` parameter wipes a release without confirmation. → Mitigation: tool description warns "destructive; verify tag before calling"; matches the existing `delete_issue_*` posture.
- **[Forgejo version skew]** The SDK targets a specific Forgejo API; older instances may reject newer fields. → Mitigation: standard SDK-passthrough error handling; surface the SDK error to the caller unchanged via `to.ErrorResult`.

## Migration Plan

This change is purely additive — no migration. Rollback is a single revert of the registration line in `operation/operation.go`. No data migration, no config flag, no version-gated behavior.

## Resolved Decisions

- **R1 (was Q1):** `list_releases` includes a client-side `state` filter (`all` | `draft` | `prerelease` | `published`, default `all`). Applied after the SDK call. Tool description documents that the filter runs after pagination so result size may be smaller than `limit`.
- **R2 (was Q2):** `download_release_attachment` is included in this change. Same inline-cap + metadata-only fallback as `download_issue_attachment`.
- **R3 (was Q3):** `create_release` and `edit_release` accept `target_commitish` and pass it through to the SDK so Forgejo can create or move the tag.
