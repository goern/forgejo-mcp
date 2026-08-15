# Demo: issue due dates — read, write, clear, and sort

*2026-08-15T23:06:46Z by Showboat 0.6.1*
<!-- showboat-id: f0f97075-4cc1-47c9-ac6d-42b661c0c4f9 -->
<!-- captured-for: PR #483 -->
<!-- captured-at: 2026-08-15 -->
<!-- captured-against: 6681329 (feat/issue-due-date) -->

## Background

Forgejo carries a deadline on every issue, but `forgejo-mcp` could neither
write it nor sort by it, and one of the three read surfaces silently dropped
it. An agent doing earliest-deadline-first work selection therefore had no way
to ask "what is due next?" without pulling every issue and sorting client-side.

This change closes all three gaps:

- **`update_issue`** gains `due_date` (RFC3339, sets the deadline) and
  `clear_due_date` (bool, unsets it). They map to the Forgejo API's
  `due_date` / `unset_due_date` fields on `EditIssueOption`. The two are
  **mutually exclusive — passing both is an error**, and omitting both leaves
  the deadline untouched.
- **`list_repo_issues`** gains `sort`, exposing the API's server-side sort
  enum. The vendored SDK (`forgejo-sdk` v3.0.0) has no `Sort` field on
  `ListIssueOption`, so this one parameter goes around the SDK client through
  the raw-HTTP helper — the same pattern `fetchOrgLabels` uses.
- **The issue resource template** (`forgejo://repo/{owner}/{repo}/issue/{index}`)
  now carries `due_date`. It builds its own payload struct rather than
  marshaling the raw SDK `Issue`, unlike `get_issue_by_index` and
  `list_repo_issues`, so the field was being dropped on that surface only.

### The nine `sort` values

`sort` is passed through to Forgejo untouched; the full enum is:

| Value | Orders by |
|-------|-----------|
| `relevance` | Search relevance |
| `latest` | Newest first |
| `oldest` | Oldest first |
| `recentupdate` | Most recently updated first |
| `leastupdate` | Least recently updated first |
| `mostcomment` | Most comments first |
| `leastcomment` | Fewest comments first |
| `nearduedate` | Nearest deadline first |
| `farduedate` | Furthest deadline first |

The last two are the due-date directions this change exists for.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
make build
```

## Replay setup

Evidence commands below reference the binary through an environment variable so
the demo replays against any build:

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-forgejo-mcp}"
# Point at a local build: export FORGEJO_MCP_BIN=./forgejo-mcp
```

Everything below was captured against a live `codeberg.org`. The write flow runs
in `synath-labs/pedlar`, a scratch repository, and the walkthrough closes every
issue it creates at the end (§7).

## 1. A fresh issue starts with no deadline

The walkthrough needs one issue to act on, so it makes one. `create_issue` has
no due-date parameter — the deadline is set afterwards with `update_issue`,
which is the point of the next section.

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli create_issue \
  --args '{"owner":"synath-labs","repo":"pedlar","title":"[demo scratch] due-date walkthrough — safe to close","body":"Scratch issue for demos/issue-due-dates.md. Safe to close."}' 2>/dev/null | python3 -c "
import sys, json
iss = json.loads(json.load(sys.stdin)[0]['text'])['Result']
print('index    :', iss['number'])
print('title    :', iss['title'])
print('state    :', iss['state'])
print('due_date :', iss['due_date'])
print('url      :', iss['html_url'])
"
```

```output
index    : 2
title    : [demo scratch] due-date walkthrough — safe to close
state    : open
due_date : None
url      : https://codeberg.org/synath-labs/pedlar/issues/2
```

## 2. `update_issue` sets the deadline

`due_date` takes an RFC3339 timestamp and maps to `EditIssueOption.due_date`.

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli update_issue \
  --args '{"owner":"synath-labs","repo":"pedlar","index":2,"due_date":"2026-08-20T17:00:00Z"}' 2>/dev/null | python3 -c "
import sys, json
iss = json.loads(json.load(sys.stdin)[0]['text'])['Result']
print('index    :', iss['number'])
print('due_date :', iss['due_date'])
"
```

```output
index    : 2
due_date : 2026-08-21T01:59:59+02:00
```

Note what came back: `2026-08-20T17:00:00Z` was sent, `2026-08-21T01:59:59+02:00`
was stored. Forgejo treats a deadline as a *day*, not an instant — it snaps the
value to the end of that UTC day and renders it in the instance's offset. The
tool passes the timestamp through unaltered; the normalisation is upstream's.
Agents should compare due dates by day, not by exact instant.

## 3. The deadline is readable on the single-issue surfaces

An issue object comes back from three places: `get_issue_by_index`, the issue
resource template, and `list_repo_issues` (§4). The first two are here.

`get_issue_by_index` marshals the raw SDK `Issue`, so `due_date` was already
present here before this change — this is the control.

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli get_issue_by_index \
  --args '{"owner":"synath-labs","repo":"pedlar","index":2}' 2>/dev/null | python3 -c "
import sys, json
iss = json.loads(json.load(sys.stdin)[0]['text'])['Result']
print('index    :', iss['number'])
print('title    :', iss['title'])
print('state    :', iss['state'])
print('due_date :', iss['due_date'])
"
```

```output
index    : 2
title    : [demo scratch] due-date walkthrough — safe to close
state    : open
due_date : 2026-08-21T01:59:59+02:00
```

The issue **resource template** is the surface that was dropping the field.
It builds its own payload struct instead of marshaling the SDK `Issue`, so
`due_date` never reached the client even though `Issue.Deadline` carried it.
`--cli` mode covers tools only, so resources are read over the MCP stdio
transport:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"0"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"forgejo://repo/synath-labs/pedlar/issue/2"}}' \
  | "${FORGEJO_MCP_BIN:-forgejo-mcp}" -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==5) | .result.contents[0].text | fromjson
        | {owner, repo, index, title, state, due_date}'
```

```output
{
  "owner": "synath-labs",
  "repo": "pedlar",
  "index": 2,
  "title": "[demo scratch] due-date walkthrough — safe to close",
  "state": "open",
  "due_date": "2026-08-21T01:59:59+02:00"
}
```

## 4. `list_repo_issues` sorts by deadline, in both directions

Sorting needs something to sort, so the walkthrough adds two more scratch
issues with deadlines either side of the first one. All three are closed at
the end of this demo.

```bash
for spec in "sort near|2026-08-18T12:00:00Z" "sort far|2026-12-01T12:00:00Z"; do
  title="[demo scratch] due-date ${spec%%|*}"
  due="${spec##*|}"
  idx=$("${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli create_issue \
    --args "{\"owner\":\"synath-labs\",\"repo\":\"pedlar\",\"title\":\"$title\",\"body\":\"Scratch issue for demos/issue-due-dates.md. Safe to close.\"}" 2>/dev/null \
    | python3 -c "import sys,json; print(json.loads(json.load(sys.stdin)[0]['text'])['Result']['number'])")
  "${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli update_issue \
    --args "{\"owner\":\"synath-labs\",\"repo\":\"pedlar\",\"index\":$idx,\"due_date\":\"$due\"}" 2>/dev/null \
    | python3 -c "
import sys, json
iss = json.loads(json.load(sys.stdin)[0]['text'])['Result']
print('created #%s  due_date=%s  %s' % (iss['number'], iss['due_date'], iss['title']))
"
done
```

```output
created #3  due_date=2026-08-19T01:59:59+02:00  [demo scratch] due-date sort near
created #4  due_date=2026-12-02T00:59:59+01:00  [demo scratch] due-date sort far
```

`sort=nearduedate` — earliest deadline first. This is the ordering an
earliest-deadline-first scheduler wants, and it now costs one call instead of
"fetch everything, sort locally":

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli list_repo_issues \
  --args '{"owner":"synath-labs","repo":"pedlar","state":"all","sort":"nearduedate"}' 2>/dev/null | python3 -c "
import sys, json
rows = json.loads(json.load(sys.stdin)[0]['text'])['Result'] or []
for i in rows:
    print('#%-3s %-7s due=%-30s %s' % (i['number'], i['state'], i['due_date'], i['title']))
"
```

```output
#3   open    due=2026-08-19T01:59:59+02:00      [demo scratch] due-date sort near
#2   open    due=2026-08-21T01:59:59+02:00      [demo scratch] due-date walkthrough — safe to close
#4   open    due=2026-12-02T00:59:59+01:00      [demo scratch] due-date sort far
#1   closed  due=None                           chore(toolchain): pin pnpm 11.8.0
```

`sort=farduedate` — the same query, the opposite direction. The three dated
issues reverse; the undated pull request stays last either way, because
Forgejo orders rows with no deadline after the dated ones in both directions.

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli list_repo_issues \
  --args '{"owner":"synath-labs","repo":"pedlar","state":"all","sort":"farduedate"}' 2>/dev/null | python3 -c "
import sys, json
rows = json.loads(json.load(sys.stdin)[0]['text'])['Result'] or []
for i in rows:
    print('#%-3s %-7s due=%-30s %s' % (i['number'], i['state'], i['due_date'], i['title']))
"
```

```output
#4   open    due=2026-12-02T00:59:59+01:00      [demo scratch] due-date sort far
#2   open    due=2026-08-21T01:59:59+02:00      [demo scratch] due-date walkthrough — safe to close
#3   open    due=2026-08-19T01:59:59+02:00      [demo scratch] due-date sort near
#1   closed  due=None                           chore(toolchain): pin pnpm 11.8.0
```

## 5. `clear_due_date` removes the deadline

Unsetting is a separate boolean rather than an empty `due_date`, because the
upstream API models it that way: `EditIssueOption.unset_due_date`. An omitted
`due_date` means "leave it alone", which is why "clear" needs its own word.

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli update_issue \
  --args '{"owner":"synath-labs","repo":"pedlar","index":2,"clear_due_date":true}' 2>/dev/null | python3 -c "
import sys, json
iss = json.loads(json.load(sys.stdin)[0]['text'])['Result']
print('index    :', iss['number'])
print('due_date :', iss['due_date'])
"
```

```output
index    : 2
due_date : None
```

## 6. Setting both at once is an error, not a precedence rule

The two parameters are mutually exclusive and the handler rejects the
combination rather than silently preferring one. Silent precedence is how
agents learn a wrong model of a tool:

```bash
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli update_issue \
  --args '{"owner":"synath-labs","repo":"pedlar","index":2,"due_date":"2026-08-20T17:00:00Z","clear_due_date":true}' 2>&1 | tail -1
```

```output
Error: tool execution failed: cannot set 'due_date' and 'clear_due_date' at the same time
```

## 7. Cleanup — the scratch issues are closed

A demo that writes to a real repository should leave it as it found it:

```bash
for idx in 2 3 4; do
  "${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli issue_state_change \
    --args "{\"owner\":\"synath-labs\",\"repo\":\"pedlar\",\"index\":$idx,\"state\":\"closed\"}" 2>/dev/null \
    | python3 -c "
import sys, json
iss = json.loads(json.load(sys.stdin)[0]['text'])['Result']
print('#%s -> %s   %s' % (iss['number'], iss['state'], iss['html_url']))
"
done

echo
echo 'Still open in synath-labs/pedlar:'
"${FORGEJO_MCP_BIN:-forgejo-mcp}" --cli list_repo_issues \
  --args '{"owner":"synath-labs","repo":"pedlar","state":"open"}' 2>/dev/null | python3 -c "
import sys, json
rows = json.loads(json.load(sys.stdin)[0]['text'])['Result'] or []
print('  (none)' if not rows else '\n'.join('  #%s %s' % (i['number'], i['title']) for i in rows))
"
```

```output
#2 -> closed   https://codeberg.org/synath-labs/pedlar/issues/2
#3 -> closed   https://codeberg.org/synath-labs/pedlar/issues/3
#4 -> closed   https://codeberg.org/synath-labs/pedlar/issues/4

Still open in synath-labs/pedlar:
  (none)
```

## 8. Autonomous workflow: earliest-deadline-first work selection

The three pieces compose into the scheduling loop this change was built for:

1. **Select.** `list_repo_issues` with `sort=nearduedate` — the next issue to
   work on is the first row. No client-side sort, and no need to pull every
   issue to find the urgent one.
2. **Read.** `forgejo://repo/{owner}/{repo}/issue/{index}` for the body and
   recent comments, with `due_date` now on the payload so the agent can
   re-check the deadline it selected on without a second call.
3. **Re-plan.** `update_issue` with `due_date` when work slips, or
   `clear_due_date` when an issue turns out not to be time-bound. Omitting
   both leaves the deadline untouched, so an agent editing a title cannot
   accidentally erase a deadline it never looked at.
4. **Audit.** `sort=farduedate` answers the opposite question — what has been
   parked the longest — which is the input to a "should this still have a
   deadline?" sweep.

Two properties make this safe to run unattended: setting and clearing are
distinct operations that cannot be confused for one another, and asking for
both at once fails loudly instead of picking one.
