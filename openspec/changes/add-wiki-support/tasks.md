## 1. Raw-HTTP wiki layer (`pkg/forgejo/wiki.go`)

- [ ] 1.1 Add `WikiPage`, `WikiPageMeta`, `WikiCommit` types with `content_base64` and `sub_url` JSON tags
- [ ] 1.2 Implement `ListWikiPages(ctx, owner, repo, page, limit)` via `DoJSONList` on `GET /repos/{o}/{r}/wiki/pages` (404 → empty); **over-fetch `limit+1`** so the handler can compute `has_next`
- [ ] 1.3 Implement `GetWikiPage(ctx, owner, repo, pageName)` via `DoJSON`, returning the raw struct (decode happens in handler)
- [ ] 1.4 Implement `GetWikiPageRevisions(ctx, owner, repo, pageName, page, limit)` via `DoJSON` (**not** `DoJSONList` — a 404 here is page-not-found, not empty); over-fetch `limit+1` for `has_next`
- [ ] 1.5 Implement `CreateWikiPage` (POST `/wiki/new`) and `EditWikiPage` (PATCH `/wiki/page/{pageName}`), both setting `content_base64`
- [ ] 1.6 Implement `DeleteWikiPage` (DELETE `/wiki/page/{pageName}`); treat **any `2xx`** as success (do not hard-require `204`)
- [ ] 1.7 Escape each path segment; for `pageName` use the round-trip-correct form confirmed in tasks 5.1–5.2 (do not assume `url.PathEscape`/`%20` works — a dash-normalized page would 404)
- [ ] 1.8 Unit tests with an `httptest` server: each method, 404-empty for list, 404-error for revisions, 403 → `ErrUnauthorized`, 404 → `ErrNotFound`

## 2. Wiki tools (`operation/wiki/wiki.go`)

- [ ] 2.1 Remove the `//go:build wiki` tag and all `forgejo-sdk` wiki imports
- [ ] 2.2 `list_wiki_pages`: add `page`/`limit` params; return pages + `page` echo + `has_next`
- [ ] 2.3 `get_wiki_page`: base64-decode content; reuse the shared line-slicer (see 2.3a) for `start_line`/`end_line`; define `total_lines = len(strings.Split(decoded, "\n"))`; `total_lines` always returned (with or without a range) so an agent can size-then-window; explicit error on undecodable content
- [ ] 2.3a Make the line-slicer callable across packages: export it in place as `repo.SliceLines` (rename `sliceLines`, `operation/repo/file.go:134`) OR lift it to a shared package; `get_file_content` (file.go:123) MUST call the SAME post-rename routine with existing tests still green. REQUIRED before 2.3 (cross-package call to the unexported symbol does not compile); copy-paste is forbidden — it would re-introduce the C8 line-dialect divergence
- [ ] 2.4 `get_wiki_revisions`: new tool; `page`/`limit` + `has_next`; 404 → not-found error
- [ ] 2.5 `create_wiki_page` / `update_wiki_page`: base64-encode `content`; default commit message; surface server-assigned `page_name`; `update` MUST NOT silently rename when `title` omitted (mechanism per task 5.5); if task 5.6a finds a precondition field, accept optional `last_commit_sha` and forward it (lost-update guard, mirroring `update_file`'s required `sha`); create on an existing title follows task 5.6b (guided "use update" error on reject, or documented-overwrite warning)
- [ ] 2.6 `delete_wiki_page`: new tool
- [ ] 2.7 Rich `mcp.WithDescription` per tool naming bound params; reuse `operation/params` descriptions, add missing ones
- [ ] 2.8 `RegisterTool(s)` registers all six; unit tests per handler (happy path + bound + error), **including a CRLF/trailing-newline `total_lines` parity test** (`"a\r\nb\r\n"` and `"a\nb\n"` → same count) and an inverted-range error test

## 3. Wiki resource template (`operation/wiki/resources.go`)

- [ ] 3.1 Add `WikiParams` + `ParseWiki` to `operation/resource/parse.go`; strict 4-segment parser; read the page-name segment from the **escaped** path (`u.EscapedPath()`/`RawPath`) then `PathUnescape` only that segment (so `%2F` survives as one segment) — MUST NOT reuse `splitPath(u.Path)` for the page name (decoded → would re-split `%2F`); reject empty (`-32602`); literal `/` → guided `-32602` ("percent-encode as `%2F`")
- [ ] 3.2 Register `forgejo://repo/{owner}/{repo}/wiki/{pageName}` with `RegisterWikiResource` and a self-describing template description (note the `%2F`/space encoding rule)
- [ ] 3.3 Handler: **two calls** (page + revisions). JSON metadata + `text/markdown` decoded-content sidecar **capped at `MaxInlineDownloadBytes` with a `get_wiki_page` marker** + bounded `recent_revisions` via `resource.Bounded(..., "get_wiki_revisions")`. `commit_sha` from the page payload. Secondary revisions-call failure degrades `recent_revisions` to empty and still succeeds the read (per `issue/resources.go`)
- [ ] 3.4 Map the **primary page call's** `403` → `-32002`, `404` → `-32003` via `resource.MapForgejoError`
- [ ] 3.5 Unit tests: parse (happy, empty, `%20`, `%2F` single-segment **and assert `u.EscapedPath()` retains `%2F`** — proves `RawPath` was populated, not recomputed from decoded `Path`, literal-slash guided error), read (< cap, > cap with sentinel, oversized-body capped, revisions-subcall-failure-degrades-to-empty, 404), **and a dispatch test that registers the wiki template alongside the issue/pr/commit templates and asserts `resources/read` on `…/wiki/Home` routes to the wiki handler** (guards literal-segment disambiguation across an mcp-go upgrade — N2)

## 4. Wiring & discovery

- [ ] 4.1 Wire `RegisterWikiTool` into `RegisterTool` and `RegisterWikiResource` into `RegisterCoreResources` in `operation/operation.go`
- [ ] 4.2 README: add a `**Wiki**` group to the tool table (six tools, bound params named) and a wiki row to the Resources table
- [ ] 4.3 `AGENTS.md`: note `operation/wiki/` tools + resource (incl. the sub-page `%2F` encoding rule and the naming convention `list_*` enumerates / `get_*` fetches one entity by name)
- [ ] 4.4 `docs/plans/wiki-support.md`: header noting the SDK-contribution path is superseded by direct API calls (this change)
- [ ] 4.5 CHANGELOG: note the additive wiki surface
- [ ] 4.6 (Referee doc-policy, endorsed by both debate sides — non-blocking) Add one line to `docs/design/output-bounding.md` extending the invariant to MCP **resource content blocks** (a data-proportional `resources/read` sidecar MUST be capped with a marker naming a range-bound tool, since `resources/read` carries no caller knob) — so the next resource author does not re-ship the unbounded-body bug C6 caught

## 5. Live verification (load-bearing)

- [ ] 5.1 Against a live Forgejo/Codeberg repo with wiki enabled: confirm the `content_base64` field name on read and write
- [ ] 5.2 Confirm page-name URL rule (spaces → dashes / encoding) for `get`/`update`/`delete` paths, and that a name from `create`/`list` round-trips verbatim
- [ ] 5.3 Confirm list/revisions paging params and the effective `limit` ceiling; document if `limit` is advisory. **Also confirm the over-fetch interaction**: that requesting `limit+1` is honored below the ceiling, that at the ceiling `has_next` is computed from the effective returned row count, and pin max `limit` to `ceiling-1` if the server clamps (so a full page at the cap never falsely reports `has_next:false`)
- [ ] 5.4 If any of 5.1–5.7 differ from the spec, correct the spec deltas before sync/archive
- [ ] 5.5 Confirm how `update_wiki_page` preserves the page name when `title` is omitted (reusing the `Getting Started` page from 5.2: PATCH content-only, GET, read the resulting title — server-side retention vs. echoing the existing title); adopt whichever keeps the page reachable, never silently renaming
- [ ] 5.6 Confirm the page `GET` payload carries `commit_sha`; if not, document the fallback (derive from `recent_revisions[0]`)
- [ ] 5.7 Confirm the upstream round-trips an **encoded slash**: create `Guides/Setup`, read via resource URI `…/wiki/Guides%2FSetup` and via `get_wiki_page` `page_name=Guides/Setup`. If the server rejects/re-splits `%2F` (proxy or `AllowEncodedSlashes` off), document that resource-URI sub-page access is unsupported there (fallback: `get_wiki_page` tool) and correct the `mcp-resource-wiki` note before sync
- [ ] 5.6a Confirm whether `PATCH …/wiki/page/{pageName}` accepts an optimistic-concurrency precondition (base `commit_sha` / `If-Match`). If yes: add optional `last_commit_sha` to `update_wiki_page` and forward it; add a stale-write conflict test. If no: document the lost-update (last-writer-wins) window in the tool description and add a missing-falsification note to design.md Risks (N3)
- [ ] 5.6b Confirm `POST …/wiki/new` behavior when the title already exists (overwrite vs `409`/`422`). If reject: map to a guided "page exists, use `update_wiki_page`" error + test. If overwrite: document the destructive behavior in the create tool description. Correct the spec scenario to the observed branch before sync (N4)

## 5b. Lens follow-ups (api-contract-drift — operational hardening, may land as separate beads)

- [ ] 5b.1 Post-decode guard: if `content_base64` decodes empty while metadata indicates a non-empty page, surface an explicit error (guards a future Forgejo field rename instead of returning a silent blank body)
- [ ] 5b.2 Distinguish "wiki disabled" from "page not found": on a wiki `404`, probe `GET /repos/{o}/{r}` `has_wiki`; if false return a distinct "wiki is not enabled on this repository" error
- [ ] 5b.3 Document a tested Forgejo/Gitea version range (support matrix) in the README wiki section

## 6. Showboat demo

- [ ] 6.1 Finalize `demo.md` with real tool calls and expected response shapes for all eight steps (incl. the resource-URI auto-resolution step)
- [ ] 6.2 Run the demo end-to-end against a throwaway repo; paste actual outputs back into `demo.md`

## 7. Wrap-up

- [ ] 7.1 `make build` + full test suite pass
- [ ] 7.2 Remove `Status/Blocked` label from Codeberg issue #32 and post a closing comment linking the change
- [ ] 7.3 `openspec validate add-wiki-support --strict` passes; archive the change
