## Why

Codeberg issue [#32](https://codeberg.org/goern/forgejo-mcp/issues/32) asks for wiki
support. The repository already ships `operation/wiki/wiki.go`, but it is dead code:
it sits behind a `//go:build wiki` tag and calls `client.ListWikiPages`,
`client.CreateWikiPage`, `client.EditWikiPage`, `CreateWikiPageOption`, and
`EditWikiPageOption` — **none of which exist in `forgejo-sdk/forgejo/v3`** (verified:
the SDK has no `wiki*.go` and defines no wiki types or methods). The build tag exists
solely to stop the broken file from breaking `make build`. That is why the issue
carries `Status/Blocked`.

The prior plan (`docs/plans/wiki-support.md`) proposed unblocking by contributing wiki
methods upstream to `forgejo-sdk` first — a 1–2 week external dependency on someone
else's review cycle. We can unblock today instead: `pkg/forgejo/rawhttp.go` already
provides `DoJSON` / `DoJSONList` helpers (built for issue attachments when the SDK fell
short) that hit the Forgejo REST API directly with the same auth, user-agent, and
error-mapping the SDK path uses. Wiki is the identical situation, so we use the same
escape hatch. No upstream dependency, no new external module.

The Forgejo/Gitea wiki API is stable and documented. Implementing against it directly
gives us all six wiki operations now, plus a `forgejo://` resource template so humans
and agents can reference a wiki page by URI without an explicit tool call.

## What Changes

- **Replace the broken SDK-based wiki package with a direct-API implementation.** Remove
  the `//go:build wiki` tag from `operation/wiki/wiki.go`; reimplement all handlers on
  top of `forgejo.DoJSON` / `forgejo.DoJSONList` (raw REST), dropping the
  `forgejo-sdk` wiki imports entirely. Register the package from
  `operation/operation.go` (it is currently never wired in).
- **Ship six wiki tools** (the create/update tools already exist in name; this change
  makes them compile and adds three read tools + delete):
  | Tool | Method + endpoint | Notes |
  |------|-------------------|-------|
  | `list_wiki_pages` | `GET /repos/{o}/{r}/wiki/pages` | Bounded: `page` + `limit`, `has_next` |
  | `get_wiki_page` | `GET /repos/{o}/{r}/wiki/page/{pageName}` | base64-decoded content; `start_line`/`end_line` bound + `total_lines` |
  | `get_wiki_revisions` | `GET /repos/{o}/{r}/wiki/revisions/{pageName}` | Bounded: `page` + `limit` |
  | `create_wiki_page` | `POST /repos/{o}/{r}/wiki/new` | base64-encodes content |
  | `update_wiki_page` | `PATCH /repos/{o}/{r}/wiki/page/{pageName}` | base64-encodes content |
  | `delete_wiki_page` | `DELETE /repos/{o}/{r}/wiki/page/{pageName}` | 2xx → success (204 canonical) |
- **Add a wiki-page resource template** `forgejo://repo/{owner}/{repo}/wiki/{pageName}`
  returning page metadata + a `text/markdown` content sidecar (decoded) + a bounded
  `recent_revisions` array (cap 30, sentinel names `get_wiki_revisions`). This lets a
  page name dropped in chat resolve to its content with no tool call, consistent with
  the existing issue/pr/commit resources.
- **Honor the output-bounding contract** (`docs/design/output-bounding.md`) on every new
  tool: `list_wiki_pages` and `get_wiki_revisions` use `page`/`limit` paging;
  `get_wiki_page` content is data-proportional so it reuses the `start_line`/`end_line`
  line-range vocabulary from `get_file_content` and returns `total_lines` for
  resumability.
- **Make the feature discoverable to humans and agents** (explicit issue requirement):
  a new **Wiki** section in the README tool table, a wiki row in the README Resources
  table, a Wiki note in `AGENTS.md`, rich per-tool/per-parameter descriptions, and an
  update to `docs/plans/wiki-support.md` recording that the SDK-contribution path is
  superseded by direct API calls. The change also removes the `Status/Blocked` label
  from issue #32 on archive.
- **Ship a showboat demo** (explicit issue requirement): a copy-pasteable end-to-end
  script (`demo.md`) that creates a page, lists pages, reads it through both the tool
  and the `forgejo://…/wiki/…` resource URI, edits it, and shows the revision history —
  proving the full surface and the agent-discovery story in one run.

## Capabilities

### New Capabilities

- `wiki-tools`: Six MCP tools for Forgejo wiki page CRUD + revision history, implemented
  via direct REST calls, each output-bounded per `docs/design/output-bounding.md`.
- `mcp-resource-wiki`: `forgejo://repo/{owner}/{repo}/wiki/{pageName}` resource template
  — single wiki page content + metadata + bounded recent revisions.

### Modified Capabilities

<!-- None. Purely additive. The previously-untagged wiki package never compiled or
registered, so no shipped behavior changes; existing tools and resources are untouched. -->

## Impact

- **Affected code**: `operation/wiki/wiki.go` (rewrite, drop build tag), new
  `operation/wiki/resources.go` (resource template), `operation/resource/parse.go`
  (`ParseWiki` + `WikiParams`), `operation/operation.go` (wire `RegisterWikiTool` and
  `RegisterWikiResource`), `pkg/forgejo` (a small `Wiki*` helper layer or inline
  `DoJSON` calls), `operation/params/` (wiki param descriptions already exist; add
  `page`/`limit`/line-range reuse).
- **No new external dependencies.** Uses the existing raw-HTTP helper and `mcp-go`.
- **No breaking changes.** Additive surface; clients without resource-template support
  use the new tools.
- **Documentation**: README (tools + resources), `AGENTS.md`, `docs/plans/wiki-support.md`,
  CHANGELOG, and the `demo.md` showboat script.
- **Verified wire behavior**: live testing against Forgejo `15.0.4+gitea-1.22.0`
  confirmed the `content_base64` field, title normalization (spaces → dashes), encoded
  slash round-trips, and the revisions response shape. Slash-separated names remain a
  flat naming convention; Forgejo creates no parent page or hierarchy.
- **Out of scope**: a wiki *index* resource (`forgejo://repo/{o}/{r}/wiki` listing all
  pages — enumeration stays the job of `list_wiki_pages`, matching the established
  "resources fetch single entities, tools enumerate" rule); wiki attachments; rendered
  HTML (we return raw markdown).
