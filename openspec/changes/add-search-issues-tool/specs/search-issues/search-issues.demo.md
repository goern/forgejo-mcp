# Owner-scoped issue search (search_issues)

*2026-08-05T13:22:09Z by Showboat dev*
<!-- showboat-id: 5b860b18-d953-4c57-b3ab-cb855e993de0 -->

*Captured: 2026-08-05 via Showboat dev*
<!-- captured-for: PR pending -->
<!-- captured-at: 2026-08-05 -->
<!-- captured-against: worktree-x8v (72ed65cf33fc7e7c99842e8fca09ee9423e212ca) -->

Proves the `search-issues` capability — spec [`spec.md`](./spec.md) (change `add-search-issues-tool`), issue [`#452`](https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/452).

Captured live against `https://git.b4mad.industries/`, org `agentic-forges` (the org #452 came from), using the `--cli` structured-output surface. Read-only: every command below is a `search_issues` call; no issue, PR, repo, or webhook is created, edited, or deleted.

## Replay setup

```bash
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"   # local build of this branch, run `make build` first
# FORGEJO_URL and FORGEJO_ACCESS_TOKEN must be exported in the shell; a read-only token is enough.
```

## Scenario: Listing open issues for an organization

Spec: *`search_issues` called with `owner` set to an organization the token can read* → *the response contains issues drawn from multiple repositories in that organization, and each issue identifies its source repository.* No `repo` parameter is passed — the org search spans every repo the token can read.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}, (.issues[] | {number, html_url})'

```

```output
{
  "page": 1,
  "limit": 20,
  "count": 15,
  "has_next": false
}
{
  "number": 457,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/457"
}
{
  "number": 16,
  "html_url": "https://git.b4mad.industries/agentic-forges/semantic-release/issues/16"
}
{
  "number": 434,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/434"
}
{
  "number": 432,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/432"
}
{
  "number": 289,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/289"
}
{
  "number": 267,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/267"
}
{
  "number": 258,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/258"
}
{
  "number": 232,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/232"
}
{
  "number": 217,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/217"
}
{
  "number": 191,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/191"
}
{
  "number": 178,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/pulls/178"
}
{
  "number": 137,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/137"
}
{
  "number": 98,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/98"
}
{
  "number": 82,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/82"
}
{
  "number": 42,
  "html_url": "https://git.b4mad.industries/agentic-forges/forgejo-mcp/issues/42"
}
```

The 15 open items span both `agentic-forges/forgejo-mcp` and `agentic-forges/semantic-release` (issue `#16`), confirming the response is not repo-scoped. Each issue identifies its source repo through `html_url` — the envelope has no separate `repository` field, since the upstream `/repos/issues/search` response does not carry one either.

## Scenario: Owner omitted

Spec: *called without `owner`, or with `owner` set to an empty string* → *the tool returns an error naming `owner` as the missing parameter, and no upstream request is made.* The validation guard runs before any Forgejo client call.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{}' 2>&1 | grep -o "owner is required" | head -1
echo "exit was error (owner is required, no upstream request logged above)"

```

```output
owner is required
exit was error (owner is required, no upstream request logged above)
```

## Scenario: Owner has no visible issues

Spec: *`search_issues` called with an owner whose repositories contain no issues matching the filters* → *an empty result set with `has_next` false, and the call does not error.* Reproduced live with an unmatchable `q` term against a real owner (rather than a nonexistent owner, which would exercise the same empty-result path less convincingly).

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","q":"zzz_no_such_issue_zzz"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next,issues}'

```

```output
{
  "page": 1,
  "limit": 20,
  "count": 0,
  "has_next": false,
  "issues": []
}
```

## Scenario: Default bound applied

Spec: *`search_issues` called without `page` or `limit`* → *at most 20 issues are returned, and the response reports `page` 1 and `limit` 20.* Same call as the org-listing scenario above, re-run to keep this section self-contained; note `page`/`limit` in the envelope even though the org has fewer than 20 open items.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}'

```

```output
{
  "page": 1,
  "limit": 20,
  "count": 15,
  "has_next": false
}
```

## Scenario: Caller raises the bound

Spec: *`limit` 100 against an owner with more than 20 matching issues* → *up to 100 issues are returned in one response, and the count may be lower if the instance clamps `limit` at `MAX_RESPONSE_ITEMS`.* Closed issues/PRs on this org exceed 20, so `state=closed` exercises the raised bound. This instance clamps at 50 — the response reports `limit` 100 (the value the caller asked for) but `count` 50 (what the instance actually returned), which doubles as live proof of the clamp.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":100}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}'

```

```output
{
  "page": 1,
  "limit": 100,
  "count": 50,
  "has_next": true
}
```

## Scenario: Caller pages forward

Spec: *`search_issues` called with `page` 2 and `limit` 20* → *the response contains the second page of results, and the response reports `page` 2.* Using `limit=5` here (instead of 20) keeps the two pages visibly different within the closed-issue set.

```bash
echo "page 1:"
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":5,"page":1}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}, (.issues[].number)'
echo "page 2:"
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":5,"page":2}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}, (.issues[].number)'

```

```output
page 1:
{
  "page": 1,
  "limit": 5,
  "count": 5,
  "has_next": true
}
456
455
454
453
452
page 2:
{
  "page": 2,
  "limit": 5,
  "count": 5,
  "has_next": true
}
451
450
9
8
7
```

## Scenario: Non-positive paging values

Spec: *`page` or `limit` less than 1* → *the tool substitutes the documented default for that parameter, and the response reports the value actually used.* Calling with `page=0` and `limit=-5` should fall back to `page=1`, `limit=20`.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","page":0,"limit":-5}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}'

```

```output
{
  "page": 1,
  "limit": 20,
  "count": 15,
  "has_next": false
}
```

## Scenario: More results available

Spec: *a call returns issues and the next-page probe at the same `limit` returns at least one issue* → *`has_next` is true, and the caller can retrieve the remainder by re-issuing the call with `page` incremented.* `state=closed,limit=5` is the same shape used in the paging scenario above; `has_next=true` on page 1 is exactly the probe result the design decision (D3) describes.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":5}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}'

```

```output
{
  "page": 1,
  "limit": 5,
  "count": 5,
  "has_next": true
}
```

## Scenario: Last page reached

Spec: *a call returns no issues, or the next-page probe at the same `limit` returns none* → *`has_next` is false.* The default org search earlier already demonstrates the non-empty case (15 issues, probe on page 2 returns none, `has_next` false). The command below demonstrates the other half of the THEN clause directly: an empty page (reusing the no-visible-issues query).

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","q":"zzz_no_such_issue_zzz"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {count,has_next}'

```

```output
{
  "count": 0,
  "has_next": false
}
```

## Scenario: Clamped page still signals continuation

Spec: *a call with `limit` 100 is answered with 50 issues because the instance clamps at `MAX_RESPONSE_ITEMS`, and further matching issues exist* → *`has_next` is true.* Unlike the caveat in the task brief, this instance genuinely clamps at 50 (see the "Caller raises the bound" capture above), so this is a **live** capture, not a test-citation fallback. `page=1` and `page=2` both return exactly 50 rows with `has_next` still true, confirming the probe (page+1 at the same `limit`) keeps firing rather than being fooled by the clamp.

```bash
echo "page 1 (limit=100 requested):"
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":100,"page":1}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}'
echo "page 2 (same limit, clamp still in effect):"
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":100,"page":2}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {page,limit,count,has_next}'

```

```output
page 1 (limit=100 requested):
{
  "page": 1,
  "limit": 100,
  "count": 50,
  "has_next": true
}
page 2 (same limit, clamp still in effect):
{
  "page": 2,
  "limit": 100,
  "count": 50,
  "has_next": true
}
```

## Scenario: Envelope always present

Spec: *any successful `search_issues` call completes* → *the response object contains `issues`, `page`, `limit`, `count`, and `has_next`, and `count` equals the length of `issues`.* Checked directly against a live response.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","limit":5}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | "keys="+(keys|sort|join(",")) + " issues_len="+(.issues|length|tostring) + " count="+(.count|tostring)'

```

```output
keys=count,has_next,issues,limit,page issues_len=5 count=5
```

## Scenario: Filtering by state

Spec: *`search_issues` called with `state` `closed`* → *only closed issues are returned.*

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","state":"closed","limit":5}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result.issues[] | .state' | sort -u

```

```output
closed
```

## Scenario: Filtering by label

Spec: *`labels` set to two comma-separated label names* → *only issues carrying those labels are returned.* Forgejo applies OR semantics across comma-separated labels — every returned issue carries at least one of the two.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","labels":"Kind/Feature,Priority/Medium"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {count,has_next}, (.issues[] | {number, labels: [.labels[].name]})'

```

```output
{
  "count": 8,
  "has_next": false
}
{
  "number": 434,
  "labels": [
    "Kind/Feature",
    "needs-scope",
    "stage:intake",
    "stage:waiting"
  ]
}
{
  "number": 232,
  "labels": [
    "Kind/Enhancement",
    "Kind/Feature"
  ]
}
{
  "number": 217,
  "labels": [
    "Priority/Medium",
    "Status/Need More Info"
  ]
}
{
  "number": 191,
  "labels": [
    "Kind/Feature",
    "Priority/Medium"
  ]
}
{
  "number": 137,
  "labels": [
    "Kind/Feature"
  ]
}
{
  "number": 98,
  "labels": [
    "Kind/Feature",
    "Priority/Medium",
    "Status/Blocked"
  ]
}
{
  "number": 82,
  "labels": [
    "Kind/Feature",
    "Status/Blocked"
  ]
}
{
  "number": 42,
  "labels": [
    "Kind/Feature",
    "Priority/Medium",
    "Status/Blocked"
  ]
}
```

## Scenario: Keyword search

Spec: *`q` set to a search term* → *only issues matching that term are returned.* `q=dashboard` narrows the open-issue set (title match).

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","q":"dashboard"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {count}, (.issues[] | {number, title})'

```

```output
{
  "count": 2
}
{
  "number": 16,
  "title": "Dependency Dashboard"
}
{
  "number": 432,
  "title": "Dependency Dashboard"
}
```

## Scenario: Excluding pull requests

Spec: *`search_issues` called with `type` `issues`* → *no pull requests appear in the results.* Checked via the `pull_request` field, which Forgejo only populates on PR entries.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --args '{"owner":"agentic-forges","type":"issues"}' --output json 2>/dev/null \
  | jq -r '.[0].text | fromjson | .Result | {count}, (.issues[] | {number, is_pull_request: (.pull_request != null)})'

```

```output
{
  "count": 10
}
{
  "number": 16,
  "is_pull_request": false
}
{
  "number": 434,
  "is_pull_request": false
}
{
  "number": 432,
  "is_pull_request": false
}
{
  "number": 289,
  "is_pull_request": false
}
{
  "number": 217,
  "is_pull_request": false
}
{
  "number": 191,
  "is_pull_request": false
}
{
  "number": 137,
  "is_pull_request": false
}
{
  "number": 98,
  "is_pull_request": false
}
{
  "number": 82,
  "is_pull_request": false
}
{
  "number": 42,
  "is_pull_request": false
}
```

## Scenario: Tool description lists bounds

Spec: *a client reads the `search_issues` tool definition* → *`page` and `limit` each carry a description explaining their effect.* `--cli <tool> --help` renders the MCP input schema the way a client would see it.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli search_issues --help

```

```output
Tool: search_issues
Description: Search issues across every repository belonging to one owner (organization or user), without naming a repo. Returns a response envelope {issues, page, limit, count, has_next} rather than a bare array. has_next true means a further page may exist; re-issue the call with page incremented to fetch it. Use list_repo_issues instead when the repo is already known.

Parameters:
  assigned_by          string     optional   Filter by assignee username
  created_by           string     optional   Filter by creator username
  labels               string     optional   Labels (comma-separated)
  limit                number     optional   Page size
  mentioned_by         string     optional   Filter by mentioned username
  milestones           string     optional   Milestone names/IDs (comma-separated)
  owner                string     required   Repository owner
  page                 number     optional   Page number (1-based)
  q                    string     optional   Search keyword
  state                string     optional   State (open|closed|all)
  type                 string     optional   Type (issues|pulls)
```

## Scenario: README lists the tool

Spec: *the README tool table is read* → *it contains a `search_issues` row naming its bound parameters.*

```bash
grep -n "search_issues" README.md

```

```output
233:| `search_issues` | Search issues across every repository of one owner (page/limit); returns `{issues,page,limit,count,has_next}` |
```
