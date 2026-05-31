## 1. Raw-HTTP wiki layer (`pkg/forgejo/wiki.go`)

- [ ] 1.1 Add `WikiPage`, `WikiPageMeta`, `WikiCommit` types with `content_base64` and `sub_url` JSON tags
- [ ] 1.2 Implement `ListWikiPages(ctx, owner, repo, page, limit)` via `DoJSONList` on `GET /repos/{o}/{r}/wiki/pages` (404 → empty)
- [ ] 1.3 Implement `GetWikiPage(ctx, owner, repo, pageName)` via `DoJSON`, returning the raw struct (decode happens in handler)
- [ ] 1.4 Implement `GetWikiPageRevisions(ctx, owner, repo, pageName, page, limit)` via `DoJSON`
- [ ] 1.5 Implement `CreateWikiPage` (POST `/wiki/new`) and `EditWikiPage` (PATCH `/wiki/page/{pageName}`), both setting `content_base64`
- [ ] 1.6 Implement `DeleteWikiPage` (DELETE `/wiki/page/{pageName}`), 204 → success
- [ ] 1.7 `url.PathEscape` every path segment, including `pageName`
- [ ] 1.8 Unit tests with an `httptest` server: each method, 404-empty for list, 403 → `ErrUnauthorized`, 404 → `ErrNotFound`

## 2. Wiki tools (`operation/wiki/wiki.go`)

- [ ] 2.1 Remove the `//go:build wiki` tag and all `forgejo-sdk` wiki imports
- [ ] 2.2 `list_wiki_pages`: add `page`/`limit` params; return pages + `page` echo + `has_next`
- [ ] 2.3 `get_wiki_page`: base64-decode content; add `start_line`/`end_line` (reuse `get_file_content` semantics) + `total_lines`; explicit error on undecodable content
- [ ] 2.4 `get_wiki_revisions`: new tool; `page`/`limit` + `has_next`
- [ ] 2.5 `create_wiki_page` / `update_wiki_page`: base64-encode `content`; default commit message; retain page name when `title` omitted
- [ ] 2.6 `delete_wiki_page`: new tool
- [ ] 2.7 Rich `mcp.WithDescription` per tool naming bound params; reuse `operation/params` descriptions, add missing ones
- [ ] 2.8 `RegisterTool(s)` registers all six; unit tests per handler (happy path + bound + error)

## 3. Wiki resource template (`operation/wiki/resources.go`)

- [ ] 3.1 Add `WikiParams` + `ParseWiki` to `operation/resource/parse.go`; URL-decode `pageName`; reject empty (`-32602`)
- [ ] 3.2 Register `forgejo://repo/{owner}/{repo}/wiki/{pageName}` with `RegisterWikiResource` and a self-describing template description
- [ ] 3.3 Handler returns JSON metadata + `text/markdown` decoded-content sidecar + bounded `recent_revisions` via `resource.Bounded(..., "get_wiki_revisions")`
- [ ] 3.4 Map upstream `403` → `-32002`, `404` → `-32003` via `resource.MapForgejoError`
- [ ] 3.5 Unit tests: parse (happy, empty, URL-encoded), read (< cap, > cap with sentinel, 404)

## 4. Wiring & discovery

- [ ] 4.1 Wire `RegisterWikiTool` into `RegisterTool` and `RegisterWikiResource` into `RegisterCoreResources` in `operation/operation.go`
- [ ] 4.2 README: add a `**Wiki**` group to the tool table (six tools, bound params named) and a wiki row to the Resources table
- [ ] 4.3 `AGENTS.md`: note `operation/wiki/` tools + resource
- [ ] 4.4 `docs/plans/wiki-support.md`: header noting the SDK-contribution path is superseded by direct API calls (this change)
- [ ] 4.5 CHANGELOG: note the additive wiki surface

## 5. Live verification (load-bearing)

- [ ] 5.1 Against a live Forgejo/Codeberg repo with wiki enabled: confirm the `content_base64` field name on read and write
- [ ] 5.2 Confirm page-name URL rule (spaces → dashes / encoding) for `get`/`update`/`delete` paths
- [ ] 5.3 Confirm list/revisions paging params and the effective `limit` ceiling; document if `limit` is advisory
- [ ] 5.4 If any of 5.1–5.3 differ from the spec, correct the spec deltas before sync/archive

## 6. Showboat demo

- [ ] 6.1 Finalize `demo.md` with real tool calls and expected response shapes for all eight steps (incl. the resource-URI auto-resolution step)
- [ ] 6.2 Run the demo end-to-end against a throwaway repo; paste actual outputs back into `demo.md`

## 7. Wrap-up

- [ ] 7.1 `make build` + full test suite pass
- [ ] 7.2 Remove `Status/Blocked` label from Codeberg issue #32 and post a closing comment linking the change
- [ ] 7.3 `openspec validate add-wiki-support --strict` passes; archive the change
