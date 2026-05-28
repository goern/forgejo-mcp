# Demo: MCP resource templates on the forgejo:// scheme

*2026-05-28T18:00:00Z by Showboat 0.6.1*
<!-- showboat-id: a3f7e2b1-mcp-resource-templates-demo-2026 -->

## Background

PR [#172](https://codeberg.org/goern/forgejo-mcp/pulls/172) (merged 2026-05-28,
commit `872d4c559868128fedd794327c14e0f74d257a84`) added 7 MCP resource templates
on the `forgejo://` URI scheme. The normative spec lives in the unarchived
change directory at
[`openspec/changes/mcp-resource-templates/specs/mcp-resources-core/spec.md`](../openspec/changes/mcp-resource-templates/specs/mcp-resources-core/spec.md);
on archive it moves to `openspec/specs/mcp-resources-core/spec.md`.

**Why resource templates?**

- **Instance-portable URIs.** `forgejo://repo/goern/forgejo-mcp` resolves against
  whatever Forgejo instance the server is configured with. Same URI works on
  codeberg.org and a self-hosted instance.
- **Additive, not replacing.** All existing tools remain. Clients that do not
  support `resources/templates/list` fall back to tools transparently.
- **JSON primary + markdown sidecar.** Commit, issue, PR, and comment resources
  return two content blocks: `application/json` for structured data and
  `text/markdown` for the human-readable body/message.
- **Embedded-list cap = 30 + truncation sentinel.** Resources that embed lists
  (comments, reviews, CI statuses) cap at 30 items. When truncated, a
  `*_truncated: true` field and a `*_list_tool` escape-hatch name the
  corresponding `list_*` tool to fetch more.
- **v1 scope.** `subscribe=false`, `listChanged=false`. Cache by URI where the
  resource is pinned to an immutable SHA.

**Resource templates shipped:**

| # | URI template | MIME | Sidecar |
|---|-------------|------|---------|
| 1 | `forgejo://owner/{owner}` | json | — |
| 2 | `forgejo://repo/{owner}/{repo}` | json | — |
| 3 | `forgejo://repo/{owner}/{repo}/commit/{sha}` | json | markdown |
| 4 | `forgejo://repo/{owner}/{repo}/commit/{sha}/status` | json | — |
| 5 | `forgejo://repo/{owner}/{repo}/issue/{index}` | json | markdown |
| 6 | `forgejo://repo/{owner}/{repo}/pr/{index}` | json | markdown |
| 7 | `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` | json | markdown |

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
make build
```

## Invocation via stdio JSON-RPC

`--cli` mode covers tools only. Resources require the MCP stdio transport.
Every section below uses `printf` to pipe JSON-RPC messages into the binary,
then `jq` to select the response by `id`.

Handshake used in every example below:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '<YOUR REQUEST>' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==<YOUR ID>)'
```

## 1. Discover all templates — resources/templates/list

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/templates/list"}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==2) | .result.resourceTemplates | map({uriTemplate,name})'
```

```output
[
  {
    "uriTemplate": "forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}",
    "name": "Forgejo Comment"
  },
  {
    "uriTemplate": "forgejo://repo/{owner}/{repo}/commit/{sha}",
    "name": "Forgejo Commit"
  },
  {
    "uriTemplate": "forgejo://repo/{owner}/{repo}/commit/{sha}/status",
    "name": "Forgejo Commit Status"
  },
  {
    "uriTemplate": "forgejo://repo/{owner}/{repo}/issue/{index}",
    "name": "Forgejo Issue"
  },
  {
    "uriTemplate": "forgejo://owner/{owner}",
    "name": "Forgejo Owner"
  },
  {
    "uriTemplate": "forgejo://repo/{owner}/{repo}/pr/{index}",
    "name": "Forgejo Pull Request"
  },
  {
    "uriTemplate": "forgejo://repo/{owner}/{repo}",
    "name": "Forgejo Repository"
  }
]
```

All 7 templates registered.

## 2. Owner — forgejo://owner/{owner}

Resolves user or org by login. Tries user first; falls back to org on 404.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"forgejo://owner/goern"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==3) | .result.contents[0].text | fromjson'
```

```output
{
  "login": "goern",
  "full_name": "Christoph Görn",
  "html_url": "https://codeberg.org/goern",
  "kind": "user",
  "website": "https://görn.name/",
  "created_at": "2022-07-04T06:28:06+02:00",
  "followers_count": 7,
  "following_count": 2
}
```

`kind` is `"user"` or `"org"`. No embedded lists; single JSON block only.

## 3. Repository — forgejo://repo/{owner}/{repo}

Returns identity + mutable counts. No embedded lists.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==4) | .result.contents[0].text | fromjson'
```

```output
{
  "owner": "goern",
  "name": "forgejo-mcp",
  "full_name": "goern/forgejo-mcp",
  "description": "This Model Context Protocol (MCP) server and Command Line Interface (CLI) tool provides tools and resources for interacting with the Forgejo (specifically Codeberg.org) REST API.",
  "html_url": "https://codeberg.org/goern/forgejo-mcp",
  "default_branch": "main",
  "fork": false,
  "archived": false,
  "private": false,
  "stars_count": 86,
  "forks_count": 27,
  "watchers_count": 9,
  "open_issues_count": 9,
  "open_pr_count": 1,
  "size": 9129,
  "has_issues": true,
  "has_wiki": false,
  "has_pull_requests": true
}
```

## 4. Commit — forgejo://repo/{owner}/{repo}/commit/{sha}

SHA must be the full 40-character hex SHA. Returns JSON + markdown sidecar.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/commit/872d4c559868128fedd794327c14e0f74d257a84"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==5) | .result.contents | map({mimeType, text: .text[:120]})'
```

```output
[
  {
    "mimeType": "application/json",
    "text": "{\"url\":\"https://codeberg.org/api/v1/repos/goern/forgejo-mcp/git/commits/872d4c559868128fedd794327c14e0f74d257a84\",\"sha\":"
  },
  {
    "mimeType": "text/markdown",
    "text": "Merge pull request 'feat: MCP resource templates — 7 entities on forgejo:// URI scheme' (#172) from "
  }
]
```

Two content blocks: `application/json` (full commit object from the SDK) and
`text/markdown` (commit message body). Clients that only want the message read
index 1; structured clients parse index 0.

## 5. Commit status — forgejo://repo/{owner}/{repo}/commit/{sha}/status

Aggregates CI contexts into a combined state. Safe to cache (pinned SHA).

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/commit/872d4c559868128fedd794327c14e0f74d257a84/status"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==6) | .result.contents[0].text | fromjson | {sha,state,total_count}'
```

```output
{
  "sha": "872d4c559868128fedd794327c14e0f74d257a84",
  "state": "pending",
  "total_count": 6
}
```

`state` is one of `success`, `failure`, `pending`, `unknown`. `total_count`
reflects all statuses returned. The `truncated` field is omitted when false
(`omitempty`); when over the embedded-list cap it appears as
`"truncated": true` alongside `"list_tool": "get_commit_statuses"` —
use the named tool for paginated enumeration.

## 6. Issue — forgejo://repo/{owner}/{repo}/issue/{index}

Returns JSON metadata + markdown sidecar. Embeds up to 30 recent comments.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":7,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/issue/148"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==7) | .result.contents | map({mimeType, snippet: .text[:200]})'
```

```output
[
  {
    "mimeType": "application/json",
    "snippet": "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\",\"index\":148,\"title\":\"docs: OpenSpec proposal for MCP resource templates\",\"state\":\"closed\",\"author\":\"goern\",\"created_at\":\"2026-05-25T10:11:56+02:00\",\"updated_at\""
  },
  {
    "mimeType": "text/markdown",
    "snippet": "# docs: OpenSpec proposal for MCP resource templates\nState: closed · #148 · goern · 2026-05-25\n\n## Summary\r\n\r\nAdds the OpenSpec change `mcp-resource-templates` — a design-only proposal for exposin"
  }
]
```

The markdown sidecar contains the full issue body rendered in place — useful
for agents that display or summarise issue content without parsing JSON.

## 7. Pull request — forgejo://repo/{owner}/{repo}/pr/{index}

Returns JSON + markdown sidecar. Embeds up to 30 recent comments and 30
recent reviews with truncation sentinels.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":8,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/pr/172"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==8) | .result.contents[0].text | fromjson | {title,state,comment_count,review_count,comments_truncated,comments_list_tool}'
```

```output
{
  "title": "feat: MCP resource templates — 7 entities on forgejo:// URI scheme",
  "state": "merged",
  "comment_count": 107,
  "review_count": 1,
  "comments_truncated": true,
  "comments_list_tool": "list_issue_comments"
}
```

PR #172 has 107 comments — well over the cap of 30. The sentinel fires:
`comments_truncated: true` + `comments_list_tool: "list_issue_comments"`.
An agent reads the 30 embedded excerpts, then calls `list_issue_comments`
with `page=2` for the rest.

The markdown sidecar provides `title · state · #index · author · created_at ·
head · base` followed by the PR description body — enough for a standalone
summary without further API calls.

## 8. Comment — forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}

`kind` is `issue` or `pr`. PR comments share the Forgejo issue-comment API;
`kind` is display context only, not a different fetch path. `id` is the
global comment ID (returned in `recent_comments[].id` above).

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":9,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/pr/172/comment/16020554"}}' \
  | ./forgejo-mcp -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==9) | .result.contents | map({mimeType, snippet: .text[:200]})'
```

```output
[
  {
    "mimeType": "application/json",
    "snippet": "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\",\"kind\":\"pr\",\"index\":172,\"id\":16020554,\"author\":\"op1st-gitops\",\"created_at\":\"2026-05-28T15:20:29+02:00\",\"updated_at\":\"2026-05-28T15:20:29+02:00\",\"body\":\""
  },
  {
    "mimeType": "text/markdown",
    "snippet": "op1st-gitops commented on pr#172:\nop1st Pipelines as Code/openspec-validate-pr-pwkpp is running.\n\nStarting Pipelinerun <b>[openspec-validate-pr-pwkpp](https://console-openshift-console.apps.nostromo."
  }
]
```

## 9. End-to-end: autonomous read-only navigation

An agent navigating a repository without burning tool-call budget:

1. `resources/read forgejo://repo/goern/forgejo-mcp` — get counts (stars,
   open issues, open PRs) at minimal cost. No list embedded; single JSON block.
2. `resources/read forgejo://repo/goern/forgejo-mcp/pr/172` — get PR metadata,
   head/base refs, mergeability, and up to 30 comment excerpts. If
   `comments_truncated: true`, continue with `list_issue_comments` tool.
3. For each `recent_comments[].id` the agent wants to read in full:
   `resources/read forgejo://repo/goern/forgejo-mcp/pr/172/comment/{id}`.
4. `resources/read forgejo://repo/goern/forgejo-mcp/commit/{sha}/status` —
   check CI. `state: "success"` → safe to merge. Response is cache-safe
   because the SHA is immutable.

**Resources vs tools for read-only paths.** Resources return focused
payloads (no pagination params needed, no field projection). Tools are
better for mutations, paginated enumeration, and operations without a URI
anchor. Prefer resources for navigation; fall back to tools when the
embedded list is truncated or when writing.

## Out of scope / future

Two follow-ups filed after PR #172:

- **`forgejo-mcp-7ra`** — `subscribe=true` support: push notifications when
  a resource changes (requires server-sent events on the transport side).
- **`forgejo-mcp-7de`** — cap telemetry: surface `truncated_count` and
  `cap_used` metrics so clients can tune their request patterns.
