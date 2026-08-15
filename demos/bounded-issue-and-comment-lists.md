# Demo: bounded issue-list and comment-thread resources

*2026-08-15T23:15:35Z by Showboat 0.6.1*
<!-- showboat-id: 8e857e23-19d2-411a-86e9-09bf9c9a64a3 -->
<!-- captured-for: PR #487 -->
<!-- captured-at: 2026-08-15 -->
<!-- captured-against: be9e4b4 (feat/issue-list-resources) -->

## Background

Two everyday reads had no cheap shape.

**Triage** — "what is open, and which of it is labelled X?" — went through
`list_repo_issues`, which returns whole issue objects: every body, plus a full
user object per issue. The payload therefore scaled with how much prose a
repository contains rather than with the number of rows asked for.

**Reading a thread** had the opposite gap. The single-issue resource
(`forgejo://repo/{owner}/{repo}/issue/{index}`) embeds recent comments but
excerpts each at 200 characters — right for deciding whether a thread is worth
reading, unusable for actually reading it. The alternatives were one resource
read per comment id, or the list tool with its per-comment user objects.

This change adds the two bounded collection resources that answer those
questions directly, following the pattern the merged `…/labels{?page,limit}`
resource established:

| URI | Returns |
|-----|---------|
| `forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}` | Issue **rows** — index, title, state, author, labels, assignees, milestone, comment count, timestamps, due date, `is_pull` — and **no bodies** |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}` | Comments with **full bodies**; `kind` ∈ {`issue`, `pr`} |

### The paging rule

Both resources request **exactly** the caller's limit from upstream, and take
"more exists" from the response rather than from an over-fetched extra row.
That is not a style preference. Forgejo computes the offset as
`(page - 1) * PageSize`, so a `PageSize` of `limit + 1` that hands back only
`limit` rows makes page N+1 begin one row past the last row page N showed —
and the row in between is unreachable from any page a client can ask for.
Truncation is therefore detected from the `Link … rel="next"` header, and the
sentinel's total comes from `X-Total-Count` when the server sends one.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
make build
```

## Replay setup

Evidence commands reference the binary through an environment variable so the
demo replays against any build:

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-forgejo-mcp}"
# Point at a local build: export FORGEJO_MCP_BIN=./forgejo-mcp
```

`--cli` mode covers tools only, so resources are read over the MCP stdio
transport — `printf` pipes the JSON-RPC handshake plus one `resources/read`
into the binary, and `jq` selects the response by `id`. This demo is
**read-only**; it runs against the public `forgejo/forgejo` repository on
codeberg.org, which at capture time had 1,560 open issues and pull requests.

## 1. The issue list at the default cap

Reading the collection with no parameters answers "what is open right now?"
The default bound is `EmbeddedListCap` = 30 rows. The payload reports the
bound it applied, whether it truncated, and the sentinel naming the tool that
enumerates the remainder.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://repo/forgejo/forgejo/issues"}}' \
  | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq -r 'select(.id==2) | .result.contents[0].text | fromjson
      | "state     : \(.state)",
        "page/limit: \(.page)/\(.limit)",
        "rows      : \(.issues|length)",
        "truncated : \(.truncated)",
        "sentinel  : \(.sentinel)",
        "",
        "row keys  : \(.issues[0]|keys|join(", "))",
        "",
        (.issues[:3][] | "  #\(.index) [\(.state)] c=\(.comment_count) pull=\(.is_pull // false)  \(.title[:52])")'
```

```output
state     : open
page/limit: 1/30
rows      : 30
truncated : true
sentinel  : [truncated: 30 of 1560 items shown. Use list_repo_issues tool to fetch more.]

row keys  : author, comment_count, created_at, index, labels, state, title, updated_at

  #13937 [open] c=0 pull=false  problem: Forgejo Actions "on push tags" doesn't trig
  #13936 [open] c=1 pull=false  bug: Branch selector uncaught TypeError with Rocket 
  #13934 [open] c=2 pull=true  fix(secutify): prevent unauthorized access to draft 
```

No `body` key on a row — that is the point of the resource, not an omission.
The same 30 issues fetched as whole objects through `list_repo_issues` cost
this much more:

```bash
rows=$(printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://repo/forgejo/forgejo/issues"}}' \
  | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq -r 'select(.id==2) | .result.contents[0].text' | wc -c)

objs=$("${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli list_repo_issues \
  --args '{"owner":"forgejo","repo":"forgejo","limit":30}' 2>/dev/null \
  | jq -r '.[0].text' | wc -c)

python3 -c "
import sys
rows, objs = int(sys.argv[1]), int(sys.argv[2])
print('resource rows      : %8d bytes' % rows)
print('list_repo_issues   : %8d bytes' % objs)
print('reduction          : %8.1fx' % (objs / rows))
" "$rows" "$objs"
```

```output
resource rows      :     8764 bytes
list_repo_issues   :   145922 bytes
reduction          :     16.7x
```

## 2. `N of M shown` — where the two numbers come from

The sentinel above said "30 of 1560". Neither number is inferred from the rows
in hand. `30` is the bound the resource applied; `1560` is what upstream
reported. Both headers the resource relies on, straight from the API:

```bash
curl -sS -D - -o /dev/null \
  -H "Authorization: token $FORGEJO_ACCESS_TOKEN" \
  "$FORGEJO_URL/api/v1/repos/forgejo/forgejo/issues?state=open&page=1&limit=30" \
  | grep -iE '^(link|x-total-count):' \
  | sed 's/^/  /'
```

```output
  link: <https://codeberg.org/api/v1/repos/forgejo/forgejo/issues?limit=30&page=2&state=open>; rel="next",<https://codeberg.org/api/v1/repos/forgejo/forgejo/issues?limit=30&page=52&state=open>; rel="last"
  x-total-count: 1560
```

## 3. The comment thread, with full bodies

`forgejo/forgejo` issue [#1024](https://codeberg.org/forgejo/forgejo/issues/1024)
is a long-running proposal thread — 69 comments at capture time. The
single-issue resource excerpts each comment at 200 characters; this resource
carries them whole.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"forgejo://repo/forgejo/forgejo/issue/1024/comments"}}' \
  | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq -r 'select(.id==3) | .result.contents[0].text | fromjson
      | "kind/index  : \(.kind)/\(.index)",
        "page/limit  : \(.page)/\(.limit)",
        "comments    : \(.comments|length)",
        "truncated   : \(.truncated)",
        "sentinel    : \(.sentinel)",
        "body chars  : min \([.comments[].body|length]|min), max \([.comments[].body|length]|max)",
        "",
        (.comments[:2][] | "  #\(.id) \(.author) \(.created_at)\n    \(.body[:70] | gsub("\n"; " "))…")'
```

```output
kind/index  : issue/1024
page/limit  : 1/30
comments    : 30
truncated   : true
sentinel    : [truncated: 30 of 69 items shown. Use list_issue_comments tool to fetch more.]
body chars  : min 75, max 6032

  #980539 caesar 2023-07-12T01:21:09+02:00
    I quite like this idea, and I think Sourcehut does something similar. …
  #981106 n0toose 2023-07-12T12:56:53+02:00
    Abuse-related vector: If a username (e.g. `https://codeberg.org/forgej…
```

## 4. Not yet demonstrable: `state`, `labels`, `page` and `limit`

Everything above used default bounds, because **no URI carrying a query string
resolves**. Passing any documented parameter fails the read:

```bash
read_uri() {
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"resources/read\",\"params\":{\"uri\":\"$1\"}}" \
    | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
    | jq -r --arg u "$1" 'select(.id==4)
        | if .error then "FAIL  \($u)" else "ok    \($u)" end'
}

# added by this change
read_uri 'forgejo://repo/forgejo/forgejo/issues'
read_uri 'forgejo://repo/forgejo/forgejo/issues?limit=5'
read_uri 'forgejo://repo/forgejo/forgejo/issues?state=all'
read_uri 'forgejo://repo/forgejo/forgejo/issue/1024/comments?page=2&limit=5'

# already merged on main — same failure, so this is inherited, not introduced
read_uri 'forgejo://repo/forgejo/forgejo/labels?page=1&limit=5'
read_uri 'forgejo://org/forgejo/labels?limit=5'
```

```output
ok    forgejo://repo/forgejo/forgejo/issues
FAIL  forgejo://repo/forgejo/forgejo/issues?limit=5
FAIL  forgejo://repo/forgejo/forgejo/issues?state=all
FAIL  forgejo://repo/forgejo/forgejo/issue/1024/comments?page=2&limit=5
FAIL  forgejo://repo/forgejo/forgejo/labels?page=1&limit=5
FAIL  forgejo://org/forgejo/labels?limit=5
```

The last two lines are the already-merged label resources on `main`, which fail
identically — so this is inherited, not introduced here.

**Cause.** The templates are registered without their RFC 6570 query
expansion:

```go
resource.RegisterTemplate(s, "forgejo://repo/{owner}/{repo}/issues", …)
```

`mcp-go` matches a read against `template.Regexp()`, which is anchored, so a
URI with a `?` matches nothing. The handlers are not at fault — `pageLimit()`
and `issuesQuery()` already parse the parameters out of `req.Params.URI`. Only
the registered string is short. Registering the form the spec, `AGENTS.md` and
the template's own description all use makes both shapes match:

```go
resource.RegisterTemplate(s, "forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}", …)
```

**Why the tests pass.** Every resource test builds an `mcp.ReadResourceRequest`
and calls the handler function directly, so none of them cross the matcher that
rejects these URIs. `operation/operation_wiki_test.go` already has the shape
that would catch it — `server.NewMCPServer` plus `HandleMessage` with a real
`resources/read` envelope.

Until that lands, these sections cannot be captured honestly and are therefore
absent rather than faked: the issue list at an explicit `limit`, the `state`
and `labels` filters, and the walk across a page boundary showing no row lost
or repeated.

## 5. A second, upstream limit on comment threads

Independent of the above: Forgejo's issue-comments endpoint ignores both
`page` and `limit`.

```bash
for q in 'page=1&limit=5' 'page=1&limit=30' 'page=2&limit=30'; do
  curl -s -H "Authorization: token $FORGEJO_ACCESS_TOKEN" \
    "$FORGEJO_URL/api/v1/repos/forgejo/forgejo/issues/1024/comments?$q" \
    | python3 -c "
import sys, json
d = json.load(sys.stdin)
print('%-16s -> %d rows, first id %s' % (sys.argv[1], len(d), d[0]['id'] if d else None))
" "$q"
done
```

```output
page=1&limit=5   -> 69 rows, first id 980539
page=1&limit=30  -> 69 rows, first id 980539
page=2&limit=30  -> 69 rows, first id 980539
```

The whole 69-comment thread arrives whatever is asked for, and `page=2` starts
at the same comment as `page=1`. Three consequences for the comment-thread
resource, none of which apply to the issue list:

1. **The response is bounded; the fetch is not.** The resource trims to `limit`
   after the entire thread has crossed the wire, so a very long thread is fully
   materialised before 30 comments of it are returned.
2. **`page` is inert here.** Comments past the first `limit` are not reachable
   through this resource at all — the sentinel correctly says more remain and
   correctly points at `list_issue_comments`, which shares the same upstream
   constraint.
3. **This thread's sentinel total is not the `X-Total-Count` number.** With no
   `Link` header the SDK leaves `Response.NextPage` at zero, so `hasMore()` is
   false and `WithMoreRemaining()` is never reached; the `69` comes from
   `resource.Bounded` counting the over-fetched rows. The issue list behaves
   the way the spec describes — it honours the bound, upstream sends both
   headers, and its `1560` really is `X-Total-Count`.

The `/issues` endpoint honours `page` and `limit` correctly, so the paging fix
this change makes — page size equal to the caller's limit, "more exists" from
the response — is right where it can be exercised.

## 6. Autonomous workflow: triage without reading the repository

The two resources are the cheap halves of a loop that used to be expensive:

1. **Survey.** Read `forgejo://repo/{owner}/{repo}/issues` — one payload, rows
   only, 16.7× smaller than the same issues as objects. The agent sees index,
   title, labels, assignee, comment count and due date: enough to rank.
2. **Decide.** Rank on the rows. Nothing has been read that the agent did not
   need, and the truncation sentinel says plainly how much of the repository
   is still unseen rather than letting a silent cut look like the whole set.
3. **Open one.** `forgejo://repo/{owner}/{repo}/issue/{index}` for the body and
   excerpted recent comments — the deciding read.
4. **Read the argument.** `forgejo://repo/{owner}/{repo}/issue/{index}/comments`
   only for the thread that turned out to matter, with full bodies.

The cost of the survey scales with the number of rows; the cost of the deep
read is paid once, for the one thread the agent chose. That is the property
`docs/design/output-bounding.md` asks for, and steps 1 and 4 are the two pieces
that were missing.
