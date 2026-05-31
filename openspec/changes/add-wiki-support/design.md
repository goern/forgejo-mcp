# Design — add-wiki-support

## Context

`operation/wiki/wiki.go` is the only domain package gated behind a build tag
(`//go:build wiki`) and the only one never registered in `operation/operation.go`. It
was written against an imagined `forgejo-sdk` wiki API that was never merged upstream.
`forgejo-sdk/forgejo/v3@v3.0.0` contains no `wiki*.go`, no `WikiPage` type, and no
`*Client` wiki methods (grep-verified). So the package cannot compile, which is the
real meaning of issue #32's `Status/Blocked`.

Meanwhile the repo already solved "SDK lacks a method" once, for issue attachments
(Codeberg #106): `pkg/forgejo/rawhttp.go` exposes `DoJSON`, `DoJSONList`, `DoMultipart`,
`DoRaw` — authenticated REST helpers that share the SDK's base URL, token resolution
(context token → `flag.Token`), user-agent, structured logging (`LogAPICall`), and an
`HTTPError` type with `ErrUnauthorized` / `ErrNotFound` sentinels. Wiki is the same
shape of problem, so it gets the same solution.

## Goals / Non-Goals

**Goals**
- All six wiki operations working against a stock Forgejo instance, no SDK changes.
- A resource template so `forgejo://repo/{o}/{r}/wiki/{pageName}` resolves to content.
- Strict output-bounding on every data-proportional response.
- High discoverability for humans (README/AGENTS) and agents (tool/param descriptions,
  resource template list).
- A reproducible showboat demo.

**Non-Goals**
- Upstream `forgejo-sdk` contribution (explicitly superseded — note left in the plan doc
  for whoever picks it up later).
- Wiki index resource, wiki attachments, server-rendered HTML.
- `resources/subscribe` (consistent with `mcp-resources-core`).

## Decision 1 — Direct REST via `DoJSON`, not the SDK

Handlers call `forgejo.DoJSON(ctx, method, path, body, &out)` (and `DoJSONList` for the
list endpoint, so a `404` on an empty wiki maps to "no pages" rather than an error).
Path building mirrors `rawhttp.go`'s own convention: `url.PathEscape` each path segment.

A thin typed layer lives in `pkg/forgejo/wiki.go` (`WikiPage`, `WikiPageMeta`,
`WikiCommit`, `ListWikiPages`, `GetWikiPage`, `GetWikiPageRevisions`, `CreateWikiPage`,
`EditWikiPage`, `DeleteWikiPage`) so the operation handlers stay declarative and the raw
paths/encoding live in one tested place. This is the same separation the SDK gave us —
we just own it now.

## Decision 2 — base64 content is load-bearing

The Forgejo/Gitea wiki API carries page bodies as base64 in a `content_base64` JSON
field on both read and write. Therefore:
- **Read** (`get_wiki_page`, resource): decode `content_base64` → UTF-8 markdown before
  returning. If decode fails, surface an explicit error (do not return raw base64).
- **Write** (`create_wiki_page`, `update_wiki_page`): the caller passes plain markdown in
  `content`; the handler base64-encodes it into `content_base64`.

This field name and the spaces→dashes page-name URL rule are asserted from the
documented API but **MUST be confirmed against a live instance** (task 5.x) before the
change archives. If the live API differs, the spec deltas are corrected before sync.

## Decision 3 — output-bounding per response shape

| Tool | Data shape | Bound (per `output-bounding.md`) | Resume signal |
|------|-----------|----------------------------------|---------------|
| `list_wiki_pages` | list of pages | `page`, `limit` | `has_next` + echoed `page` |
| `get_wiki_revisions` | list of commits | `page`, `limit` | `has_next` + echoed `page` |
| `get_wiki_page` | one page, unbounded body | `start_line`, `end_line` (reuse `get_file_content` vocab) | `total_lines` + echoed range |
| `create`/`update`/`delete` | single fixed-shape result | exempt (semantics-bounded) | n/a |

`get_wiki_page` reuses the **exact** parameter names `get_file_content` already exposes
so agents carry one line-range dialect across both tools.

For the **resource**, embedded `recent_revisions` use `operation/resource.Bounded(...,
"get_wiki_revisions")` (cap = `EmbeddedListCap` = 30) so the truncation sentinel is
identical to every other embedded list. The page *content* in the resource is returned
in full as a `text/markdown` sidecar — matching the issue/commit precedent where the
entity body is embedded whole (resource reads take no params, so range-slicing belongs
on the tool, not the resource). A size note is documented; callers needing slices use
the `get_wiki_page` tool.

## Decision 4 — URI parsing for wiki page names

New `ParseWiki(uri) (WikiParams, error)` in `operation/resource/parse.go`:
- path = `[owner, repo, "wiki", pageName]`, so `len(parts) == 4 && parts[2] == "wiki"`.
- `pageName` is URL-decoded (`url.PathUnescape`) and must be non-empty after decode.
- `WikiParams{Owner, Repo, PageName string}`.

The existing `parseForgejoURI` already rejects empty/whitespace path segments, so
`forgejo://repo/o/r/wiki/` fails with `-32602` for free. Page names containing `/`
(sub-paths) are out of scope for v1 — a `/`-bearing name would parse as extra segments
and be rejected; documented as a known limitation.

## Decision 5 — discoverability is a first-class requirement

The issue explicitly asks that the feature "surface the user and [be] easy to discover
by humans and agents." Concretely:
- **Humans**: a `**Wiki**` group in the README tool table (all six tools, with the bound
  params named in the description column), a `wiki` row in the README Resources table,
  and a one-line Wiki note in `AGENTS.md` pointing at `operation/wiki/`.
- **Agents**: every tool gets a `mcp.WithDescription` that states what it does *and* its
  bound params; the resource template gets a `mcp.WithTemplateDescription` spelling out
  the URI form and the decoded-markdown sidecar, so it shows up usefully in
  `resources/templates/list`.
- **Trail-of-record**: `docs/plans/wiki-support.md` gets a header noting the SDK path is
  superseded by this change; the `Status/Blocked` label is removed from #32 on archive.

## Showboat demo

`openspec/changes/add-wiki-support/demo.md` is a copy-pasteable transcript that proves
the whole surface in one sitting against a throwaway repo. Outline:

1. **Create** — `create_wiki_page` with title `Home`, a markdown body. Show the returned
   metadata.
2. **List** — `list_wiki_pages` (page 1, limit 50) → `Home` appears; show `has_next:false`.
3. **Read (tool)** — `get_wiki_page` for `Home` → decoded markdown + `total_lines`.
4. **Read (resource, the money shot)** — an agent is handed the bare URI
   `forgejo://repo/<you>/<repo>/wiki/Home` in a prompt and resolves it with **no tool
   call**, returning the rendered page. This is the discoverability payoff.
5. **Edit** — `update_wiki_page` adds a section; show new commit.
6. **History** — `get_wiki_revisions` for `Home` → two commits; show bounded paging.
7. **Bounded read** — `get_wiki_page` with `start_line=1`, `end_line=5` on a long page →
   slice + `total_lines` proving resumability.
8. **Cleanup** — `delete_wiki_page`.

Each step lists the exact tool call / URI and the shape of the expected response so a
reviewer can replay it and a future reader sees the intended UX end to end.

## Risks / Open Questions

- **base64 field name / page-name encoding** — verify live (task 5.x). Highest-impact
  unknown; everything else is mechanical.
- **`limit` ceiling** — Forgejo caps `limit`; we echo the server's effective page size
  and document that `limit` is advisory, mirroring other paged tools.
- **Write auth** — wiki writes need a token with repo write + wiki enabled on the repo;
  `403` maps to `ErrUnauthorized` → MCP error, same as every other write tool. The demo
  notes the required token scope.
