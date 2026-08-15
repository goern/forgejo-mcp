# Demo: bounded issue-list and comment-thread resources

*2026-08-15T23:33:59Z by Showboat 0.6.1*
<!-- showboat-id: 8e857e23-19d2-411a-86e9-09bf9c9a64a3 -->
<!-- captured-for: PR #487 -->
<!-- captured-at: 2026-08-15 -->
<!-- captured-against: d879ca1 (feat/issue-list-resources) -->

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
§4 walks a page boundary to show that it holds.

Truncation is detected from the `Link … rel="next"` header and the sentinel's
total comes from `X-Total-Count`. One endpoint needs more than that, and §7
shows why.

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

The sentinel said "30 of 1560". Neither number is inferred from the rows in
hand. `30` is the bound the resource applied; `1560` is what upstream
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

## 3. An explicit `limit`

The bound is the caller's to set. `limit=5` returns five rows and the sentinel
carries the same authoritative total — the total describes the query, not the
page:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://repo/forgejo/forgejo/issues?limit=5"}}' \
  | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq -r 'select(.id==2) | .result.contents[0].text | fromjson
      | "page/limit: \(.page)/\(.limit)",
        "rows      : \(.issues|length)",
        "sentinel  : \(.sentinel)",
        "",
        (.issues[] | "  #\(.index)  \(.title[:56])")'
```

```output
page/limit: 1/5
rows      : 5
sentinel  : [truncated: 5 of 1560 items shown. Use list_repo_issues tool to fetch more.]

  #13937  problem: Forgejo Actions "on push tags" doesn't trigger 
  #13936  bug: Branch selector uncaught TypeError with Rocket Load
  #13934  fix(secutify): prevent unauthorized access to draft rele
  #13933  Update dependency globals to v17.11.0 (forgejo)
  #13932  problem: the `action` table is huge
```

## 4. The page boundary — nothing lost, nothing repeated

This is the property the paging rule exists to protect. The earlier
implementation asked upstream for `limit + 1` rows to detect truncation while
showing only `limit`, which moved the offset of the next page one row past the
end of the current one — so exactly one row per boundary was returned by no
page at all.

The check below is self-verifying rather than a matter of reading two lists
carefully: it walks pages 1 and 2 at `limit=5`, then reads the same query once
at `limit=10`. If the boundary is sound the two pages concatenated must equal
the single ten-row read, in order, with no duplicates.

```bash
read_uri() {
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"resources/read\",\"params\":{\"uri\":\"$1\"}}" \
    | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
    | jq -r 'select(.id==2) | .result.contents[0].text | fromjson | .issues[].index'
}

p1=$(read_uri 'forgejo://repo/forgejo/forgejo/issues?page=1&limit=5')
p2=$(read_uri 'forgejo://repo/forgejo/forgejo/issues?page=2&limit=5')
ten=$(read_uri 'forgejo://repo/forgejo/forgejo/issues?page=1&limit=10')

python3 - "$p1" "$p2" "$ten" <<'PY'
import sys
p1, p2, ten = (s.split() for s in sys.argv[1:4])
print('page 1 (limit=5) :', ' '.join(p1))
print('page 2 (limit=5) :', ' '.join(p2))
print()
print('last id of page 1 :', p1[-1])
print('first id of page 2:', p2[0])
print()
walked = p1 + p2
print('pages 1+2 walked  :', ' '.join(walked))
print('single limit=10   :', ' '.join(ten))
print()
print('duplicates across pages :', sorted(set(p1) & set(p2)) or 'none')
print('walk == single read     :', walked == ten)
PY
```

```output
page 1 (limit=5) : 13937 13936 13934 13933 13932
page 2 (limit=5) : 13931 13930 13929 13924 13922

last id of page 1 : 13932
first id of page 2: 13931

pages 1+2 walked  : 13937 13936 13934 13933 13932 13931 13930 13929 13924 13922
single limit=10   : 13937 13936 13934 13933 13932 13931 13930 13929 13924 13922

duplicates across pages : none
walk == single read     : True
```

## 5. `state` and `labels` reach the API

`state` accepts `open`, `closed` or `all` and defaults to `open`; `labels` is a
comma-separated list, trimmed around each entry. Both are applied upstream
rather than by filtering a page after the fact, which the sentinel's total
makes visible — narrowing the query moves the total, not just the rows.

Label names containing `/` must be percent-encoded as `%2F`: the URI is parsed
as a URI, so a bare slash in a query value does not match the template.

```bash
for q in 'state=closed' \
         'state=closed&labels=bug' \
         'state=closed&labels=bug,%20forgejo%2Fui'; do
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"resources/read\",\"params\":{\"uri\":\"forgejo://repo/forgejo/forgejo/issues?$q&limit=3\"}}" \
    | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
    | jq -r --arg q "$q" 'select(.id==2) | .result.contents[0].text | fromjson
        | "\($q)\n    state=\(.state)  labels=\(.labels // [] | tostring)\n    \(.sentinel)"'
done
```

```output
state=closed
    state=closed  labels=[]
    [truncated: 3 of 12209 items shown. Use list_repo_issues tool to fetch more.]
state=closed&labels=bug
    state=closed  labels=["bug"]
    [truncated: 3 of 743 items shown. Use list_repo_issues tool to fetch more.]
state=closed&labels=bug,%20forgejo%2Fui
    state=closed  labels=["bug","forgejo/ui"]
    [truncated: 3 of 172 items shown. Use list_repo_issues tool to fetch more.]
```

## 6. The comment thread, with full bodies

`forgejo/forgejo` issue [#1024](https://codeberg.org/forgejo/forgejo/issues/1024)
is a long-running proposal thread — 69 comments at capture time. The
single-issue resource excerpts each comment at 200 characters; this resource
carries them whole, and reports the same kind of sentinel.

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://repo/forgejo/forgejo/issue/1024/comments"}}' \
  | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq -r 'select(.id==2) | .result.contents[0].text | fromjson
      | "kind/index  : \(.kind)/\(.index)",
        "page/limit  : \(.page)/\(.limit)",
        "comments    : \(.comments|length)",
        "sentinel    : \(.sentinel)",
        "body chars  : min \([.comments[].body|length]|min), max \([.comments[].body|length]|max)",
        "",
        (.comments[:2][] | "  #\(.id) \(.author) \(.created_at)\n    \(.body[:70] | gsub("\n"; " "))…")'
```

```output
kind/index  : issue/1024
page/limit  : 1/30
comments    : 30
sentinel    : [truncated: 30 of 69 items shown. Use list_issue_comments tool to fetch more.]
body chars  : min 75, max 6032

  #980539 caesar 2023-07-12T01:21:09+02:00
    I quite like this idea, and I think Sourcehut does something similar. …
  #981106 n0toose 2023-07-12T12:56:53+02:00
    Abuse-related vector: If a username (e.g. `https://codeberg.org/forgej…
```

## 7. Paging a thread

Walking the thread five at a time. Page 2 is a different window, and the two
pages join without a gap or an overlap, exactly as the issue list does in §4:

```bash
read_ids() {
  printf '%s\n' \
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
    '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"resources/read\",\"params\":{\"uri\":\"$1\"}}" \
    | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
    | jq -r 'select(.id==2) | .result.contents[0].text | fromjson | .comments[].id'
}

p1=$(read_ids 'forgejo://repo/forgejo/forgejo/issue/1024/comments?page=1&limit=5')
p2=$(read_ids 'forgejo://repo/forgejo/forgejo/issue/1024/comments?page=2&limit=5')
ten=$(read_ids 'forgejo://repo/forgejo/forgejo/issue/1024/comments?page=1&limit=10')

python3 - "$p1" "$p2" "$ten" <<'PY'
import sys
p1, p2, ten = (s.split() for s in sys.argv[1:4])
print('page 1 (limit=5) :', ' '.join(p1))
print('page 2 (limit=5) :', ' '.join(p2))
print()
print('last id of page 1 :', p1[-1])
print('first id of page 2:', p2[0])
print()
print('duplicates across pages :', sorted(set(p1) & set(p2)) or 'none')
print('walk == single limit=10 :', p1 + p2 == ten)
PY
```

```output
page 1 (limit=5) : 980539 981106 1059048 1059052 1059586
page 2 (limit=5) : 2023383 2105550 2105950 2106024 2109638

last id of page 1 : 1059586
first id of page 2: 2023383

duplicates across pages : none
walk == single limit=10 : True
```

One difference from the issue list is worth knowing about, because it is
invisible in the payload: **Forgejo's issue-comments endpoint ignores `page`
and `limit`** — it returns the whole thread whatever is asked for, and sends
`X-Total-Count` but no `Link` header. (The `/issues` endpoint honours both and
sends both, which is why only this resource needs the following.) The window
above is therefore applied client-side, and only when the server hands back
more rows than were requested, so the resource stops slicing by itself if
Forgejo ever starts honouring the bounds. The response is bounded either way;
the fetch behind it is not.

## 8. Autonomous workflow: triage without reading the repository

The two resources are the cheap halves of a loop that used to be expensive:

1. **Survey.** Read `forgejo://repo/{owner}/{repo}/issues` — one payload, rows
   only, 16.7× smaller than the same issues as objects. The agent sees index,
   title, labels, assignee, comment count and due date: enough to rank.
2. **Narrow.** Re-read with `state` and `labels` when the first pass is too
   broad, or walk pages when it is too deep. The bound is the caller's, and
   the sentinel says plainly how much of the repository is still unseen rather
   than letting a silent cut look like the whole set.
3. **Open one.** `forgejo://repo/{owner}/{repo}/issue/{index}` for the body and
   excerpted recent comments — the deciding read.
4. **Read the argument.** `forgejo://repo/{owner}/{repo}/issue/{index}/comments`
   only for the thread that turned out to matter, with full bodies, paged if
   the thread is long.

The cost of the survey scales with the number of rows; the cost of the deep
read is paid once, for the one thread the agent chose. That is the property
`docs/design/output-bounding.md` asks for, and steps 1 and 4 are the two pieces
that were missing.
