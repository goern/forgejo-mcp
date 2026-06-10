# Label CRUD + resource-templates (create / edit / delete / get)

*2026-06-10T06:30:07Z by Showboat dev*
<!-- showboat-id: 88dd08cd-baeb-455b-b2e4-85f1b22e65d6 -->

*Captured: 2026-06-10 via Showboat*
<!-- captured-for: PR #192 / Codeberg #190 -->
<!-- captured-at: 2026-06-10 -->
<!-- captured-against: 00221ef -->

## Background

`forgejo-mcp` previously only offered label *read* and *assignment* tools
(`list_repo_labels`, `list_org_labels`, `add_issue_labels`,
`remove_issue_labels`). Creating, renaming, recoloring, or deleting a label
required falling back to raw `curl`.

This change adds the missing lifecycle half:

**Repo-label CRUD (via `forgejo-sdk/v3`):**
- `create_repo_label` — create a label, get its numeric id back
- `edit_repo_label` — PATCH one or more fields (only supplied fields change)
- `delete_repo_label` — safe-by-default delete: refuses when the label is in
  use and reports the reference count; `delete_mode=force` overrides
- `get_repo_label` — read one label by id

**Org-label CRUD (via raw-HTTP `DoJSON` — no SDK method exists):**
- `create_org_label` / `edit_org_label` / `delete_org_label` / `get_org_label`
  — same shape as the repo tools; org in-use count is best-effort over repos
  the token can see

**Three URI-addressable resource templates:**
- `forgejo://repo/{owner}/{repo}/label/{id}` — single label
- `forgejo://repo/{owner}/{repo}/labels{?page,limit}` — bounded list
- `forgejo://org/{org}/labels{?page,limit}` — bounded org-label list

All eight tools share a `color` normaliser that accepts `rrggbb` or `#rrggbb`
(6-digit hex only) and prepends `#` if absent.

## Replay setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"
make build   # produces ./forgejo-mcp
```

Spec: `openspec/changes/label-crud/specs/label-crud/spec.md`
      `openspec/changes/label-crud/specs/mcp-resource-label/spec.md`
Issue: codeberg.org/goern/forgejo-mcp/issues/190

## 1. Tool surface

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli list 2>/dev/null | grep -E "(create|edit|delete|get)_(repo|org)_label"
```

```output
  create_org_label                         Create an organization-level label. Returns the created label including its numeric id.
  create_repo_label                        Create a repository label. Returns the created label including its numeric id.
  delete_org_label                         Delete an organization-level label. By default refuses if the label is in use; set delete_mode=force to override. Note: in-use count is best-effort over repos visible to the token and may under-count.
  delete_repo_label                        Delete a repository label. By default refuses if the label is in use; set delete_mode=force to override.
  edit_org_label                           Edit an organization-level label (PATCH — only supplied fields change). Providing no fields is an error.
  edit_repo_label                          Edit a repository label (PATCH — only supplied fields change). Providing no fields is an error.
  get_org_label                            Get a single organization-level label by ID.
  get_repo_label                           Get a single repository label by ID.
```

## 2. create_repo_label — full create/read cycle

Create a label, then read it back by id to verify the returned fields.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli create_repo_label \
  --args '{"owner":"goern","repo":"forgejo-mcp","name":"Demo/Label","color":"0e8a16","description":"Showboat demo label"}' 2>/dev/null | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
r = json.loads(data[0][\"text\"])[\"Result\"]
print(f\"created: id={r[\"id\"]}  name={r[\"name\"]}  color={r[\"color\"]}  description={r[\"description\"]}\")
"
```

```output
created: id=1761185  name=Demo/Label  color=0e8a16  description=Showboat demo label
```

Color  (no leading ) was normalised to  by the server before it reached the API — no . The returned uid=1000(goern) gid=1000(goern) groups=1000(goern),10(wheel) (1761185) is what all subsequent tools use.

Color `0e8a16` (no leading `#`) was normalised to `#0e8a16` before reaching the API — no 422. The returned id (1761185) is what all subsequent tools use.

## 3. get_repo_label — read one label by id

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli get_repo_label \
  --args '{"owner":"goern","repo":"forgejo-mcp","id":1761185}' 2>/dev/null | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
r = json.loads(data[0][\"text\"])[\"Result\"]
print(f\"id={r[\"id\"]}  name={r[\"name\"]}  color={r[\"color\"]}  url={r[\"url\"]}\")
"
```

```output
id=1761185  name=Demo/Label  color=0e8a16  url=https://codeberg.org/api/v1/repos/goern/forgejo-mcp/labels/1761185
```

## 4. edit_repo_label — PATCH semantics (only supplied fields change)

Change only the color; name and description stay untouched.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli edit_repo_label \
  --args '{"owner":"goern","repo":"forgejo-mcp","id":1761185,"color":"0288d1"}' 2>/dev/null | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
r = json.loads(data[0][\"text\"])[\"Result\"]
print(f\"id={r[\"id\"]}  name={r[\"name\"]}  color={r[\"color\"]}  description={r[\"description\"]}\")
"
```

```output
id=1761185  name=Demo/Label  color=0288d1  description=Showboat demo label
```

Color changed to `0288d1`; `name` and `description` are identical to the values set on creation. PATCH semantics confirmed.

## 5. Invalid color is rejected at the boundary

No network call is made; the server returns an error before touching the API.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli create_repo_label \
  --args '{"owner":"goern","repo":"forgejo-mcp","name":"Bad","color":"not-a-color"}' 2>&1 | head -3
```

```output
2026-06-10 08:30:59	[31mERROR[0m	to/to.go:45	color must be a 6-digit hex string (e.g. #0088ff or 0088ff), got "not-a-color"
codeberg.org/goern/forgejo-mcp/v2/pkg/to.ErrorResult
	/var/home/goern/Source/codeberg.org/goern/forgejo-mcp/pkg/to/to.go:45
```

## 6. delete_repo_label — safe-by-default guard

### 6a. Refuse when in use (safe mode)

First apply the label to an issue, then attempt deletion without force.

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli add_issue_labels \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":190,"labels":"1761185"}' 2>/dev/null | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
r = json.loads(data[0][\"text\"])[\"Result\"]
labels = [l[\"name\"] for l in r[\"labels\"]]
print(f\"issue #190 labels: {labels}\")
"
```

```output
issue #190 labels: ['Demo/Label', 'Kind/Feature', 'Priority/High']
```

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli delete_repo_label \
  --args '{"owner":"goern","repo":"forgejo-mcp","id":1761185}' 2>&1 | grep -v "^codeberg" | head -3
```

```output
2026-06-10 08:31:10	[34mINFO[0m	forgejo/forgejo.go:79	Successfully created Forgejo client	{"url": "https://codeberg.org/", "token_configured": true, "user_agent": "forgejo-mcp/2.28.0-dev+00221ef"}
2026-06-10 08:31:10	[31mERROR[0m	to/to.go:45	label "Demo/Label" is used by 1 issue(s)/PR(s); set delete_mode=force to delete anyway
	/var/home/goern/Source/codeberg.org/goern/forgejo-mcp/pkg/to/to.go:45
```

`Demo/Label` is applied to issue #190, so the safe-mode guard fires and reports the count. No label was deleted.

### 6b. Force delete removes the label even when in use

```bash
${FORGEJO_MCP_BIN:-./forgejo-mcp} --cli delete_repo_label \
  --args '{"owner":"goern","repo":"forgejo-mcp","id":1761185,"delete_mode":"force"}' 2>/dev/null | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
r = json.loads(data[0][\"text\"])[\"Result\"]
print(f\"deleted={r[\"deleted\"]}  id={r[\"id\"]}\")
"
```

```output
deleted=True  id=1761185
```

## 7. Resource templates — URI-addressable labels

### 7a. List registered templates

## 7. Resource templates — URI-addressable labels

Three new resource templates let MCP-resource-aware clients read labels by URI
without tool calls. These are additive — existing tools are unchanged.

| URI | What it returns |
|-----|----------------|
|  | Single label (JSON) |
|  | Bounded list, cap 30 |
|  | Bounded org-label list, cap 30 |

The demo uses the MCP stdio transport directly.

## 7. Resource templates — URI-addressable labels

Three new resource templates let MCP-resource-aware clients read labels by URI
without tool calls. These are additive — existing tools are unchanged.

The new templates:

- `forgejo://repo/{owner}/{repo}/label/{id}` — single label (JSON)
- `forgejo://repo/{owner}/{repo}/labels{?page,limit}` — bounded list, cap 30
- `forgejo://org/{org}/labels{?page,limit}` — bounded org-label list, cap 30

The demo uses the MCP stdio transport directly.

### 7a. resources/templates/list — confirm the three label templates are registered

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"showboat","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"resources/templates/list","params":{}}
' | ${FORGEJO_MCP_BIN:-./forgejo-mcp} -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq -r 'select(.id==2) | .result.resourceTemplates[].uriTemplate' | grep label
```

```output
forgejo://org/{org}/labels
forgejo://repo/{owner}/{repo}/label/{id}
forgejo://repo/{owner}/{repo}/labels
```

### 7b. Read single label — forgejo://repo/{owner}/{repo}/label/{id}

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"showboat","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/label/335058"}}
' | ${FORGEJO_MCP_BIN:-./forgejo-mcp} -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==2) | .result.contents[0].text | fromjson'
```

```output
{
  "id": 335058,
  "name": "Kind/Feature",
  "color": "0288d1",
  "description": "New functionality",
  "url": "https://codeberg.org/api/v1/repos/goern/forgejo-mcp/labels/335058"
}
```

### 7c. Read bounded label list — forgejo://repo/{owner}/{repo}/labels

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"showboat","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://repo/goern/forgejo-mcp/labels"}}
' | ${FORGEJO_MCP_BIN:-./forgejo-mcp} -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==2) | .result.contents[0].text | fromjson | {owner, repo, label_count: (.labels | length), truncated}'
```

```output
{
  "owner": "goern",
  "repo": "forgejo-mcp",
  "label_count": 26,
  "truncated": null
}
```

26 labels, under the 30-item cap — no truncation sentinel emitted. For a repo with more than 30 labels, the response would include  and a  field naming  as the enumeration fallback.

26 labels, under the 30-item cap — no truncation sentinel emitted.
For repos with more than 30 labels the response includes `truncated: true`
and a `sentinel` field naming `list_repo_labels` as the unbounded fallback.

### 7d. Read org-label list — forgejo://org/{org}/labels

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"showboat","version":"1"}}}
{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"forgejo://org/forgejo/labels"}}
' | ${FORGEJO_MCP_BIN:-./forgejo-mcp} -t stdio -url "$FORGEJO_URL" -token "$FORGEJO_ACCESS_TOKEN" 2>/dev/null \
  | jq 'select(.id==2) | .result.contents[0].text | fromjson | {org, label_count: (.labels | length), truncated, list_tool}'
```

```output
{
  "org": "forgejo",
  "label_count": 17,
  "truncated": null,
  "list_tool": null
}
```

## 8. End-to-end: autonomous label bootstrap workflow

An agent setting up a new repository can bootstrap a full label taxonomy
without leaving the MCP loop:

1. `create_repo_label` for each label — color normalised automatically,
   numeric id returned for immediate use in `add_issue_labels`.
2. `get_repo_label` to verify a specific label is what the agent expects.
3. `edit_repo_label` to rename or recolor a label after reviewing issue
   distributions — only the changed field is sent upstream.
4. `delete_repo_label` (safe mode) to clean up stale labels — the in-use
   guard prevents silent data loss; `delete_mode=force` only when the agent
   has confirmed the label should be stripped from all referencing issues.
5. Resource URIs (`forgejo://repo/{owner}/{repo}/labels`) let resource-aware
   clients cache the current label set between calls without a separate
   `list_repo_labels` invocation.

For org-wide triage (e.g. applying a shared priority taxonomy across multiple
repos), the same workflow applies to `create_org_label` / `edit_org_label` /
`delete_org_label` — the org-level delete guard counts usage across all
org repos the token can read and discloses when the count may be
under-reported due to inaccessible repos.
