## 1. Scaffolding

- [x] 1.1 Create `operation/release/` directory with `release.go` (package `release`).
- [x] 1.2 Add release-specific shared param descriptions to `operation/params/params.go` (`ReleaseID`, `ReleaseTag`, `ReleaseTagName`, `ReleaseTargetCommitish`, `ReleaseDraft`, `ReleasePrerelease`). Reuse existing `Owner`, `Repo`, `Page`, `Limit`, `AttachmentID`, `AttachmentName`, `AttachmentContent`, `AttachmentFilename`, `AttachmentMIME` where applicable.
- [x] 1.3 Define a single `RegisterTool(s *server.MCPServer)` entry point in `operation/release/release.go`.

## 2. Release read tools

- [x] 2.1 Define + register `list_releases` (params: `owner`, `repo`, `page`, `limit`, `state`). Handler calls `client.ListReleases` with `ListReleasesOptions{ListOptions: ListOptions{Page: page, PageSize: limit}}`, then applies the client-side `state` filter (`all`/`draft`/`prerelease`/`published`). Reject unknown `state` values before calling the SDK.
- [x] 2.2 Define + register `get_release_by_id` (params: `owner`, `repo`, `release_id`). Handler casts `release_id` to `int64` with a range guard.
- [x] 2.3 Define + register `get_release_by_tag` (params: `owner`, `repo`, `tag`).
- [x] 2.4 Define + register `get_latest_release` (params: `owner`, `repo`).

## 3. Release write tools

- [x] 3.1 Define + register `create_release` (params: `owner`, `repo`, `tag_name`, `target_commitish`, `name`, `body`, `draft`, `prerelease`). Build `CreateReleaseOption` from inputs; pass through to SDK.
- [x] 3.2 Define + register `edit_release` (params: `owner`, `repo`, `release_id`, `tag_name`, `target_commitish`, `name`, `body`, `draft`, `prerelease`). Only set `EditReleaseOption` fields that the caller provided.
- [x] 3.3 Define + register `delete_release` (params: `owner`, `repo`, `release_id`). Tool description warns "destructive".
- [x] 3.4 Define + register `delete_release_by_tag` (params: `owner`, `repo`, `tag`). Tool description warns "destructive — verify tag before calling".

## 4. Release-attachment tools

- [x] 4.1 Define + register `list_release_attachments` (params: `owner`, `repo`, `release_id`, `page`, `limit`). Fetch full slice from SDK, then slice client-side `[offset : offset+limit]`. Tool description documents the slicing trade-off.
- [x] 4.2 Define + register `get_release_attachment` (params: `owner`, `repo`, `release_id`, `attachment_id`).
- [x] 4.3 Define + register `download_release_attachment` (params: `owner`, `repo`, `release_id`, `attachment_id`). Fetch metadata via `GetReleaseAttachment`; if `Size < inlineCap`, fetch `browser_download_url` with auth header and return base64-inline embedded resource; otherwise return metadata-only result with `Inline=false`, `Reason`, and `BytesIncluded=0`. Reuse the inline-cap constant from `operation/attachment/` (or move it to a shared location if it currently lives package-private).
- [x] 4.4 Define + register `create_release_attachment` (params: `owner`, `repo`, `release_id`, `content`, `filename`, `mime_type`). Decode `content` from base64 into `bytes.NewReader`; pass to `client.CreateReleaseAttachment`. Surface base64 decode errors via `to.ErrorResult` without calling the SDK.
- [x] 4.5 Define + register `edit_release_attachment` (params: `owner`, `repo`, `release_id`, `attachment_id`, `name`). Build `EditAttachmentOptions{Name: name}`.
- [x] 4.6 Define + register `delete_release_attachment` (params: `owner`, `repo`, `release_id`, `attachment_id`). Tool description warns "destructive".

## 5. Wiring

- [x] 5.1 Import `codeberg.org/goern/forgejo-mcp/v2/operation/release` in `operation/operation.go`.
- [x] 5.2 Add `RegisterReleaseTool(s)` function in `operation/operation.go` that delegates to `release.RegisterTool(s)` and logs `Registered release tools`.
- [x] 5.3 Call `RegisterReleaseTool(s)` from `RegisterTool(s *server.MCPServer)`, placed after `RegisterAttachmentTool`. (Also wired the CLI domain grouping in `cmd/cli.go` so `--cli list` shows a `RELEASE:` group.)

## 6. Tests

- [x] 6.1 Add `operation/release/release_test.go` with handler-level unit tests that stub the SDK via a small interface, covering at minimum: `list_releases` default + custom pagination, `list_releases` state filter (`published` excludes drafts and prereleases), `list_releases` invalid `state` rejected before SDK call, `get_release_by_id` not-found path, `create_release` with new tag (via `target_commitish`), `edit_release` partial update (body only), `delete_release` + `delete_release_by_tag` success paths.
- [x] 6.2 Add base64 decode failure test for `create_release_attachment` that asserts the SDK is not called.
- [x] 6.3 Add client-side slicing test for `list_release_attachments` (page boundary + empty page).
- [x] 6.4 Add `download_release_attachment` tests: below-cap returns inline base64; at-cap returns metadata-only with `Inline=false` and a populated `Reason`; unknown attachment ID returns error result.

## 7. Documentation

- [x] 7.1 Add a "Releases" row group to the tools table in `README.md`, listing all 14 new tools with one-line descriptions.
- [x] 7.2 Mention the `release` domain in `DEVELOPER.md` wherever existing domains are enumerated as examples.
- [x] 7.3 Cross-link the new tools from the issue #127 close comment (the PR body already does this). _Done: PR #134 body `Closes #127`; merge auto-closed the issue with the cross-reference._

## 8. Verification

- [x] 8.1 `make build` succeeds.
- [x] 8.2 `make vendor` is a no-op (no go.mod changes).
- [x] 8.3 `openspec validate add-releases-support` passes (already does after specs phase; re-confirm after tasks complete).
- [x] 8.4 Manual smoke against `goern/forgejo-mcp` on Codeberg using the built binary: list_releases → get_latest_release → get_release_by_tag → list_release_attachments. Read-only sequence is safe to run against the real repo. _Done: captured in `demos/release-management.md` against live Codeberg data; also covered download_release_attachment over-cap path._
- [x] 8.5 Update bd issue `forgejo-mcp-0ep` notes with smoke-test outcome; close once PR merged. _Done: closed via `bd close forgejo-mcp-0ep` after PR #134 merge._
