# Showboat Demo — Forgejo wiki via forgejo-mcp

End-to-end proof of the whole wiki surface in one sitting: six tools + the
`forgejo://…/wiki/…` resource template, including the agent-discovery payoff (a bare URI
in a prompt resolves to page content with no tool call).

## Prerequisites

- `forgejo-mcp` built from this change and configured against a Forgejo/Codeberg instance.
- A token with **repo write** scope; the target repo has **Wiki enabled** in settings.
- A throwaway repo, referred to below as `<owner>/<repo>`.

> Replace `<owner>/<repo>` throughout. Outputs shown are the *expected shape*; task 6.2
> replaces them with real captured output.

---

## 1. Create a page

```jsonc
// tool: create_wiki_page
{ "owner": "<owner>", "repo": "<repo>", "title": "Home",
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n" }
```
→ Page created. Content is sent base64-encoded under the hood; the caller only ever sees
plain markdown.

```jsonc
{ "title": "Home", "page_name": "Home", "commit_sha": "a1b2c3…" }
```

## 2. List pages (bounded)

```jsonc
// tool: list_wiki_pages
{ "owner": "<owner>", "repo": "<repo>", "page": 1, "limit": 50 }
```
```jsonc
{ "pages": [ { "title": "Home", "page_name": "Home", "sub_url": "Home" } ],
  "page": 1, "has_next": false }
```

## 3. Read via the tool (decoded + total_lines)

```jsonc
// tool: get_wiki_page
{ "owner": "<owner>", "repo": "<repo>", "page_name": "Home" }
```
```jsonc
{ "title": "Home", "commit_sha": "a1b2c3…", "total_lines": 3,
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n" }
```

## 4. Read via the resource URI — the money shot

Hand an agent a prompt containing only the bare URI:

```
Summarize forgejo://repo/<owner>/<repo>/wiki/Home
```

A resource-template-aware client (Claude Code, Claude Desktop, Codex, Cursor) calls
`resources/read` on the URI **with no explicit tool call** and gets:

- `application/json`: `{ owner, repo, page_name, title, commit_sha, recent_revisions: [...] }`
- `text/markdown` sidecar: the decoded page body.

This is the discoverability win: a wiki page is now first-class, URI-addressable context.

## 5. Edit the page

```jsonc
// tool: update_wiki_page
{ "owner": "<owner>", "repo": "<repo>", "page_name": "Home",
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n\n## Links\n\n- See `list_wiki_pages`.\n" }
```
→ New commit. `title` omitted ⇒ page keeps its name.

## 6. Revision history (bounded)

```jsonc
// tool: get_wiki_revisions
{ "owner": "<owner>", "repo": "<repo>", "page_name": "Home", "page": 1, "limit": 30 }
```
```jsonc
{ "revisions": [ { "sha": "d4e5f6…", "author": "you", "message": "Update wiki page 'Home'" },
                 { "sha": "a1b2c3…", "author": "you", "message": "Create wiki page 'Home'" } ],
  "page": 1, "has_next": false }
```

## 7. Bounded read of a long page (resumability)

```jsonc
// tool: get_wiki_page
{ "owner": "<owner>", "repo": "<repo>", "page_name": "Home",
  "start_line": 1, "end_line": 5 }
```
```jsonc
{ "title": "Home", "total_lines": 7, "start_line": 1, "end_line": 5,
  "content": "# Welcome\n\nThis wiki is driven by forgejo-mcp.\n\n## Links" }
```
`total_lines: 7` > `end_line: 5` ⇒ the caller knows to request lines 6–7 next.

## 8. Cleanup

```jsonc
// tool: delete_wiki_page
{ "owner": "<owner>", "repo": "<repo>", "page_name": "Home" }
```
→ `204 No Content` reported as success. `list_wiki_pages` now returns an empty list.

---

## What this proves

- All six tools work against a stock instance with **no `forgejo-sdk` wiki dependency**.
- Content round-trips correctly through base64 (caller never sees encoding).
- Every data-proportional response is **bounded and resumable** (`page`/`limit`/`has_next`,
  `start_line`/`end_line`/`total_lines`).
- A wiki page is **discoverable by URI** for agents and humans alike.
