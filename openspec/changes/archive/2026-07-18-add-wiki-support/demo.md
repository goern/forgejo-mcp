# Showboat Demo — Forgejo wiki via forgejo-mcp

End-to-end proof of the whole wiki surface in one sitting: six tools + the
`forgejo://…/wiki/…` resource template, including the agent-discovery payoff (a bare URI
in a prompt resolves to page content with no tool call).

## Prerequisites

- `forgejo-mcp` built from this change and configured against a Forgejo/Codeberg instance.
- A token with **repo write** scope; the target repo has **Wiki enabled** in settings.
- A throwaway repo, referred to below as `<owner>/<repo>`.

> Captured on 2026-07-18 against Forgejo `16.0.0+gitea-1.22.0`, using
> `Codemancer/forgejo-mcp-wiki-test` and the locally built binary from this change. The
> temporary demo pages were deleted after the run. Replace the repository and page names
> when replaying.

---

## 1. Create a page

```jsonc
// tool: create_wiki_page
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test", "title": "PR Ready Demo 20260718",
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n" }
```
→ Page created. Content is sent base64-encoded under the hood; the caller only ever sees
plain markdown.

```jsonc
{ "title": "PR Ready Demo 20260718", "page_name": "PR-Ready-Demo-20260718",
  "commit_sha": "11435b1508fdd4fbdea5e18da527cb7b181339b8" }
```

## 2. List pages (bounded)

```jsonc
// tool: list_wiki_pages
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test", "page": 1, "limit": 30 }
```
```jsonc
{ "pages": [ { "title": "Home", "page_name": "Home", "sub_url": "Home" },
             { "title": "PR Ready Demo 20260718",
               "page_name": "PR-Ready-Demo-20260718",
               "sub_url": "PR-Ready-Demo-20260718" } ],
  "page": 1, "has_next": false }
```

## 3. Read via the tool (decoded + total_lines)

```jsonc
// tool: get_wiki_page
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test",
  "page_name": "PR-Ready-Demo-20260718" }
```
```jsonc
{ "title": "PR Ready Demo 20260718", "page_name": "PR-Ready-Demo-20260718",
  "commit_sha": "11435b1508fdd4fbdea5e18da527cb7b181339b8", "total_lines": 4,
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n" }
```
> `total_lines` is 4, not 3: the body ends in `\n`, and the shared `sliceLines` split
> (`strings.Split(content, "\n")`) counts the trailing newline as a final empty line.

## 4. Read via the resource URI — the money shot

Hand an agent a prompt containing only the bare URI:

```
Summarize forgejo://repo/Codemancer/forgejo-mcp-wiki-test/wiki/PR-Ready-Demo-20260718
```

The result is **one `resources/read` call instead of a `get_wiki_page` tool call**,
returning:

- `application/json`: `{"owner":"Codemancer","repo":"forgejo-mcp-wiki-test","page_name":"PR-Ready-Demo-20260718","title":"PR Ready Demo 20260718","commit_sha":"11435b1508fdd4fbdea5e18da527cb7b181339b8","recent_revisions":[{"sha":"11435b1508fdd4fbdea5e18da527cb7b181339b8","author":"Codemancer","message":"Create wiki page 'PR Ready Demo 20260718'\n"}]}`
- `text/markdown`: `# Welcome\n\nThis wiki is driven by forgejo-mcp.\n`

A client that auto-resolves resource URIs issues that `resources/read` transparently;
clients that do not (the common case today — auto-resolution of a bare URI embedded in
chat text is not yet universal across Claude Code/Desktop, Codex, Cursor) issue it
explicitly. Either way the wiki page becomes first-class, URI-addressable context — the
discoverability win — without over-claiming that every client resolves it with zero
prompting.

## 5. Edit the page

```jsonc
// tool: update_wiki_page
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test",
  "page_name": "PR-Ready-Demo-20260718",
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n\n## Links\n\n- See `list_wiki_pages`.\n" }
```
```jsonc
{ "title": "PR Ready Demo 20260718", "page_name": "PR-Ready-Demo-20260718",
  "commit_sha": "be7f5ee2d66fdd0253eab9a870330c6744266084" }
```
`title` was omitted and the page kept its name.

## 6. Revision history (bounded)

```jsonc
// tool: get_wiki_revisions
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test",
  "page_name": "PR-Ready-Demo-20260718", "page": 1, "limit": 30 }
```
```jsonc
{ "revisions": [ { "sha": "be7f5ee2d66fdd0253eab9a870330c6744266084",
                   "author": "Codemancer", "message": "Update wiki page 'PR-Ready-Demo-20260718'\n" },
                 { "sha": "11435b1508fdd4fbdea5e18da527cb7b181339b8",
                   "author": "Codemancer", "message": "Create wiki page 'PR Ready Demo 20260718'\n" } ],
  "page": 1, "has_next": false }
```

## 7. Bounded read of a long page (resumability)

```jsonc
// tool: get_wiki_page
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test",
  "page_name": "PR-Ready-Demo-20260718",
  "start_line": 1, "end_line": 5 }
```
```jsonc
{ "title": "PR Ready Demo 20260718", "page_name": "PR-Ready-Demo-20260718",
  "commit_sha": "be7f5ee2d66fdd0253eab9a870330c6744266084",
  "total_lines": 8, "start_line": 1, "end_line": 5,
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n\n## Links" }
```
`total_lines: 8` > `end_line: 5` ⇒ the caller knows to request lines 6–8 next. (The
edited body ends in `\n`, so the split counts a trailing empty 8th line; the same
`sliceLines` routine and count as `get_file_content`.)

## 8. Cleanup

```jsonc
// tool: delete_wiki_page
{ "owner": "Codemancer", "repo": "forgejo-mcp-wiki-test",
  "page_name": "PR-Ready-Demo-20260718" }
```
```jsonc
{ "deleted": true, "page_name": "PR-Ready-Demo-20260718" }
```
The following list call contained only the pre-existing `Home` page, proving that the
temporary page was removed.

## Flat slash-name check

The same live run also created `PR Ready Resource/Slash 20260718`, received the normalized
name `PR-Ready-Resource%2FSlash-20260718`, and successfully read it through
`resources/read` at the `%2F` URI. Both metadata and the Markdown sidecar were returned;
the probe page was then deleted. Forgejo treats
the slash as a flat naming convention, not a hierarchy: it does not automatically create
or link a `Guides` parent page. Reuse the returned normalized `page_name` for tools; in a
resource URI, a literal slash must be represented as `%2F` without double-encoding an
already normalized name. Delete both probe pages after the check.

---

## What a successful replay proves

- All six tools work against a stock instance with **no `forgejo-sdk` wiki dependency**.
- Content round-trips correctly through base64 (caller never sees encoding).
- Every data-proportional response is **bounded and resumable** (`page`/`limit`/`has_next`,
  `start_line`/`end_line`/`total_lines`).
- A wiki page is **discoverable by URI** for agents and humans alike.
