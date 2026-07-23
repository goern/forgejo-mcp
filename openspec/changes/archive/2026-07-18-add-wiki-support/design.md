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

Handlers call `forgejo.DoJSON(ctx, method, path, body, &out)`. The `404`→empty mapping
(`DoJSONList`) is used for **`list_wiki_pages` only** — there a `404` means "wiki has no
pages." `get_wiki_revisions` uses plain `DoJSON`: an existing page always has ≥1 revision,
so a `404` there means the page does not exist and MUST surface as a not-found error, not
an empty list. Path building escapes each segment, but the wiki page-name segment is the
one wire detail the live-verification tasks pin down (see Decision 2): the upstream may
expect the dashed/normalized form rather than a `%20`-escaped human title, so the spec
states a **round-trip-correctness requirement** (a name returned by `create`/`list` must
be reusable verbatim on `get`/`update`/`delete`) rather than prescribing the escape
mechanism up front.

**Paging without response headers.** `DoJSON` returns only `error` and discards
`resp.Header`, so `X-Total-Count` / `Link` are unreachable. A one-row over-fetch is
**incorrect** for Forgejo's page-number pagination: changing `limit` also changes the
offset of every later page. Live testing with 32 pages proved that requesting page 1
with upstream `limit=31`, returning 30 rows, and then requesting page 2 with
`limit=31` skips item 31 because page 2 starts at item 32.

The handler therefore requests the current page with exactly the caller's `limit`. If
the response is short, `has_next=false`. If it contains exactly `limit` rows, the
handler makes a second request for `page+1` with the **same** `limit` and sets
`has_next` according to whether that probe is non-empty. This preserves page boundaries
and works when the total is an exact multiple of `limit`; the same algorithm applies to
wiki revisions.

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

Live verification against Forgejo `15.0.4+gitea-1.22.0` confirmed this field name,
the page-name URL rule, and `commit_sha` in the page payload. It also established that
a content-only PATCH renames a page to `unnamed`; `update_wiki_page` therefore reads the
current page and echoes its title when the caller omits `title`.

## Decision 3 — output-bounding per response shape

| Tool | Data shape | Bound (per `output-bounding.md`) | Resume signal |
|------|-----------|----------------------------------|---------------|
| `list_wiki_pages` | list of pages | `page`, `limit` | `has_next` + echoed `page` |
| `get_wiki_revisions` | list of commits | `page`, `limit` | `has_next` + echoed `page` |
| `get_wiki_page` | one page, unbounded body | `start_line`, `end_line` (same param names as `get_file_content`) | `total_lines` + echoed range |
| `create`/`update`/`delete` | single fixed-shape result | exempt (semantics-bounded) | n/a |

`get_wiki_page` reuses the **same line-splitting routine** `get_file_content` uses
(`sliceLines` in `operation/repo/file.go`), not merely its parameter names — so the two
tools agree on what a "line" is by construction. Important: `get_file_content` does **not**
itself expose a `total_lines` field; `total_lines` is **new to `get_wiki_page`**, defined
as `len(strings.Split(decoded, "\n"))` (the count produced by that same split). The split
is on `"\n"`, so a trailing newline counts as a final empty line and CRLF bodies keep
their `"\r"` — the count is identical for `"a\r\nb\r\n"` and `"a\nb\n"`. A unit test
asserts CRLF/LF parity and trailing-newline counting.

For the **resource**, embedded `recent_revisions` use `operation/resource.Bounded(...,
"get_wiki_revisions")` (cap = `EmbeddedListCap` = 30). The page *content* sidecar is
**bounded too** — a wiki page is a document and can be megabytes, which is exactly the
unbounded-body trap `output-bounding.md` exists to prevent (the issue/commit precedent
does not apply: issue bodies are bounded by comment norms, wiki pages are not). The
sidecar is capped at `MaxInlineDownloadBytes` (the existing 1 MiB cap in `pkg/forgejo`)
with a truncation marker naming `get_wiki_page` (which has `start_line`/`end_line`) for
the remainder. This satisfies sub-rules 1 and 3 without a per-read knob the
`resources/read` protocol cannot carry.

The resource performs **two upstream calls** (page + revisions). Per the
`operation/issue/resources.go` precedent, only the primary page call maps the read
result (`403`→`-32002`, `404`→`-32003`); a secondary revisions-call failure degrades
`recent_revisions` to empty and still succeeds the read. `commit_sha` comes from the page
payload, not `recent_revisions[0]`.

## Decision 4 — URI parsing for wiki page names

New `ParseWiki(uri) (WikiParams, error)` in `operation/resource/parse.go`:
- Strict 4-segment path = `[owner, repo, "wiki", pageName]` (`parts[2] == "wiki"`). It does
  **not** greedily join trailing segments — a greedy join would mis-route any future
  `…/wiki/{page}/<subresource>` URI.
- The page-name segment is read from the **escaped** path (`u.EscapedPath()` / `RawPath`),
  then `url.PathUnescape`d on its own. This matters: `parseForgejoURI` splits on the
  already-decoded `u.Path`, so a `%2F` in a sub-page name would be pre-split into two
  segments and lost. Reading the escaped form keeps `Guides%2FSetup` as one segment that
  decodes to `Guides/Setup`.
- Empty/whitespace page name → `-32602` (handled by `parseForgejoURI` today).
- A **literal** unencoded `/` (extra segments) → a guided `-32602` telling the caller to
  percent-encode `/` as `%2F`, rather than an opaque "invalid params."
- `WikiParams{Owner, Repo, PageName string}`.

Sub-pages are therefore reachable (via `%2F`), not silently lossy. The README/AGENTS
entries document the percent-encoding rule for `/` and spaces.
Forgejo does not model these names as a hierarchy: `Parent/Child` is a flat naming
convention, and creating it neither creates nor links a `Parent` page.

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

- **base64 field name / page-name encoding** — resolved by live verification against
  Forgejo `15.0.4+gitea-1.22.0`: `content_base64` round-trips, spaces normalize to
  dashes, and the returned `%2F` form round-trips for slash-bearing names.
- **Paging / `limit` ceiling** — paging was verified with 30 temporary pages and requests
  up to `limit=50`, with no observed clamp or `Link` header. A subsequent 32-page test
  falsified `limit+1` over-fetch because it changes later page offsets. The adopted
  same-limit next-page probe preserves continuity. The exact server ceiling remains
  unknown, so `limit` is advisory if an instance clamps it.
- **Write auth** — wiki writes need a token with repo write + wiki enabled on the repo;
  `403` maps to `ErrUnauthorized` → MCP error, same as every other write tool. The demo
  notes the required token scope.
- **Referee doc-policy item (from C6)** — `docs/design/output-bounding.md` is written for
  *tools*; it is silent on whether the invariant extends to MCP *resource* content blocks.
  This change makes the wiki resource compliant regardless (1 MiB cap + marker), so it is
  not a blocker, but the cross-cutting doc-policy decision ("does the bounding contract
  formally cover resource content?") is logged for the maintainer to settle repo-wide.
- **Concurrent writes** — the tested Forgejo accepted both a stale `last_commit_sha`
  body field and stale `If-Match` header (HTTP 200); updates are last-writer-wins.
- **Duplicate create** — creating an existing title returned HTTP 201 and replaced its
  content; the tool description explicitly warns about this destructive behavior.

## Adversarial Review — 2026-05-31

A three-agent debate team (adversary `devils-advocate`, defender `proponent`, and an
orthogonal `api-contract-drift` lens) reviewed the proposal + design. The adversary culled
to **8 load-bearing critiques**; the defender returned **8 CONCEDE-PATCH, 0 DEFEND, 0
STALEMATE**. The referee (lead) verified the load-bearing code claims against the actual
repo before applying every patch. Direction held (direct REST via `DoJSON`, resource
template, output-bounding intent); the spec under-specified wire details and leaned on a
single-word demo page (`Home`) that hid every multi-word / oversized / multi-call edge.

| # | Load-bearing critique | Verdict | Fix (bound to an existing repo precedent) |
|---|----------------------|---------|-------------------------------------------|
| C1 | Page-name encoding self-contradicted (`url.PathEscape`→`%20` vs spaces→dashes) | CONCEDE-PATCH | Spec states round-trip-correctness; mechanism gated on live task 5.2; normative multi-word scenario added |
| C2 | `has_next` not derivable — `DoJSON` discards `resp.Header` | CONCEDE-PATCH, later live-corrected | Initial `limit+1` over-fetch was falsified because page offsets depend on limit; use an exact-limit current-page request plus a same-limit `page+1` probe when full |
| C3 | `404`→empty wrong for `get_wiki_revisions` (existing page always has ≥1 commit) | CONCEDE-PATCH | `DoJSON` not `DoJSONList`; not-found scenario added; 404 asymmetry documented |
| C4 | `update` title-default silently renames a spaced page | CONCEDE-PATCH | Outcome-based "MUST NOT silently rename"; mechanism gated on new task 5.5 |
| C5 | Sub-page `/` names → opaque `-32602`; `%2F` would pre-split on decoded path | CONCEDE-PATCH | Parse page name from **escaped** path; strict 4-seg parser; `%2F` scenario + guided error |
| C6 | Resource embedded page body unbounded | CONCEDE-PATCH | Cap sidecar at `MaxInlineDownloadBytes` (1 MiB) + marker → `get_wiki_page` (+ referee doc item) |
| C7 | Resource is a 2-call read with no error policy | CONCEDE-PATCH | Primary-call-maps-errors / secondary-degrades-to-empty (`issue/resources.go`); `commit_sha` from page payload; new task 5.6 |
| C8 | `total_lines`/line-range non-deterministic; false claim `get_file_content` returns `total_lines` | CONCEDE-PATCH | Mandate reuse of the shared `sliceLines`; define `total_lines = len(strings.Split(decoded,"\n"))`; CRLF/trailing-newline test; corrected demo counts; dropped the false provenance claim |

**Orthogonal lens (`api-contract-drift`)** surfaced operational/longevity risks the
in-loop adversary did not, recorded as follow-ups rather than blockers: (a) silent empty
body if Forgejo renames `content_base64` in a future release → add a post-decode
non-empty guard; (b) `has_wiki=false` repos return `404` indistinguishable from
"page not found" on writes → probe `has_wiki` and return a distinct "wiki disabled" error;
(c) no declared Forgejo/Gitea support matrix → document a tested version range. These are
captured as lens follow-ups in `tasks.md §5` / future work.

**Self-correction logged:** the defender's first take on C5 ("`%2F` already works") was
false; re-reading `parse.go` (`u.Path` is already decoded) corrected it to the
escaped-path requirement now in Decision 4.

## Adversarial Review — 2026-05-31 (third pass / survival check)

A third team re-ran the change under instruction to surface **only NEW** load-bearing
critiques the first two rounds missed (no re-litigation), plus an orthogonal
`agent-ergonomics` lens (the issue's "discoverable by agents" promise). Result: **3
CONCEDE-PATCH, 1 WITHDRAWN** — the C1–C8 surface held, three genuinely new edges surfaced.

| # | New critique | Verdict | Fix |
|---|--------------|---------|-----|
| N1 | Spec mandates reusing `sliceLines`, but it is **unexported** (`file.go:134`) — `operation/wiki` cannot call it | CONCEDE-PATCH | New task 2.3a: export `repo.SliceLines` (or lift to shared pkg) with `get_file_content` unchanged; copy-paste forbidden |
| N2 | Feared resource-template **dispatch collision** (wiki vs commit/issue/pr 4-seg shapes) | **WITHDRAWN** | Disproven: commit/issue/pr/status templates already ship and coexist; referee verified mcp-go v0.17.0 `matchesTemplate` (server.go:614) matches by **literal-segment regex**, not shape. Added an optional dispatch regression test (task 3.5) as future-proofing |
| N3 | `update_wiki_page` has **no `commit_sha` precondition** (unlike `update_file`'s required `sha`) → silent last-writer-wins clobber | CONCEDE-PATCH | Spec acknowledges the lost-update window; task 5.6a discovers if PATCH accepts a precondition → optional `last_commit_sha` if yes, documented window if no |
| N4 | `create_wiki_page` on an **existing title** — overwrite vs 409 unspecified/unverified | CONCEDE-PATCH | Spec adds the conditional branch (guided "use update" error on reject / documented-overwrite warning); task 5.6b verifies live |

**Referee finding while verifying N2** (a real spec-vs-mcp-go gap, converging with lens
Gap 4): mcp-go matches a `resources/read` URI against each template's compiled regex
*before* any handler runs. A **literal-slash** URI (`…/wiki/Guides/Setup`) matches no
template regex, so mcp-go returns its own "handler not found" and `ParseWiki` is never
reached — the spec's guided `-32602` for literal slashes **cannot fire at the resource
layer**. The encoding guidance and `get_wiki_page` fallback therefore live in the
**template description** (where the agent reads it before building the URI); `ParseWiki`'s
guided error remains a belt-and-suspenders path. The `%2F`-encoded form matches and reaches
`ParseWiki` normally. Captured in the `mcp-resource-wiki` parsing requirement.

**Agent-ergonomics lens** (5 gaps; sharpest were silent-misdirection, not hard errors):
- Gap 1 — naming convention (`list_*` enumerates / `get_*` fetches one) is implicit → state it in `AGENTS.md` (task 4.3).
- Gap 2 — agent derives `page_name` from `title` instead of using the returned canonical name → `create_wiki_page` description must say "use the returned `page_name`, never derive from `title`" (folded into the discovery requirement + create requirement).
- Gap 3 — the demo's "money shot" over-claimed universal client auto-resolution → reframed honestly as "one `resources/read` instead of a tool call; clients that don't auto-resolve issue it explicitly."
- Gap 4 — `%2F`-rejecting servers / literal slashes give a bare 404 dead-end → guidance moved to the template description (see referee finding); fallback to `get_wiki_page` stated.
- Gap 5 — agent can't know to paginate before the first (possibly huge) read → `get_wiki_page` description states `total_lines` is always returned, so size-then-window is possible.

**Net:** the survival check did its job — confirmed the prior surface is sound *and* caught
three real new edges (all mechanical, evidence-gated) plus five ergonomics refinements.
No stalemates, no blockers.
