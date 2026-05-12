# Demo: list_org_labels + merged list_repo_labels

*2026-05-12T10:45:00Z by Showboat 0.6.1*
<!-- showboat-id: 7a1c9b2f-org-labels-demo-2026 -->

## Background

Forgejo organizations can carry their own label set, separate from each
repo. `add_issue_labels` accepts *either* a repo-scoped or an org-scoped
label ID, but until this release MCP callers could only discover the
repo-scoped half. Issue [#125](https://codeberg.org/goern/forgejo-mcp/issues/125)
asked us to close that gap.

Two changes in v2.22.0:

- `list_org_labels` — new tool, lists org-level labels for any organization.
- `list_repo_labels` — now merges org labels into its response when the
  owner is an organization. Each returned label carries a `scope` field
  of `"repo"` or `"org"` so callers can tell them apart.

Opt out of the merge with `include_org_labels=false`.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
make build
```

## Tool surface

```bash
./forgejo-mcp --cli list 2>/dev/null | grep -E 'list_(org|repo)_labels'
```

```output
  list_org_labels                          List organization-level labels. Each label carries a scope field of "org".
  list_repo_labels                         List repository labels. When the owner is an organization and include_org_labels is true (default), org-level labels are merged into the response. Each label carries a scope field of "repo" or "org".
```

## 1. list_org_labels — org-only discovery

```bash
./forgejo-mcp --cli list_org_labels \
  --args '{"org":"forgejo"}' 2>/dev/null | python3 -c "
import sys, json
data = json.loads(json.load(sys.stdin)[0]['text'])
labels = data.get('Result', [])
print(f'Total org labels: {len(labels)}')
for l in labels[:5]:
    print(f\"  id={l['id']:7d}  scope={l['scope']:<4s}  {l['name']}\")
"
```

```output
Total org labels: 17
  id= 223765  scope=org   User research - Accessibility
  id= 440466  scope=org   User research - Blocked
  id= 440496  scope=org   User research - Community
  id= 209569  scope=org   User research - Config (instance)
  id= 209666  scope=org   User research - Errors
```

## 2. list_repo_labels — merged response (default)

```bash
./forgejo-mcp --cli list_repo_labels \
  --args '{"owner":"forgejo","repo":"forgejo","limit":50}' 2>/dev/null | python3 -c "
import sys, json
from collections import Counter
data = json.loads(json.load(sys.stdin)[0]['text'])
labels = data.get('Result', [])
scopes = Counter(l['scope'] for l in labels)
print(f'Merged labels: {len(labels)} | by scope: {dict(scopes)}')
print('Sample:')
for l in [labels[0], labels[20], labels[-1]]:
    print(f\"  id={l['id']:7d}  scope={l['scope']:<4s}  {l['name']}\")
"
```

```output
Merged labels: 67 | by scope: {'repo': 50, 'org': 17}
Sample:
  id= 204851  scope=repo  arch/riscv64
  id= 223008  scope=repo  dependency-upgrade
  id= 208225  scope=org   User research - Settings (in-app)
```

## 3. list_repo_labels — opt out with include_org_labels=false

```bash
./forgejo-mcp --cli list_repo_labels \
  --args '{"owner":"forgejo","repo":"forgejo","limit":50,"include_org_labels":false}' 2>/dev/null | python3 -c "
import sys, json
from collections import Counter
data = json.loads(json.load(sys.stdin)[0]['text'])
labels = data.get('Result', [])
print(f'Total: {len(labels)} | scopes: {dict(Counter(l[\"scope\"] for l in labels))}')
"
```

```output
Total: 50 | scopes: {'repo': 50}
```

## 4. End-to-end: autonomous label triage across scopes

An agent can now build a single label lookup table that spans both scopes,
then call `add_issue_labels` with whichever ID matches — Forgejo applies
org-scoped and repo-scoped labels through the same endpoint:

1. `list_repo_labels` with default `include_org_labels=true` → unified
   `{id, name, scope}` table.
2. Match issue title/body against label names (e.g. `"User research - Labels"`
   → id `206178`, scope `org`).
3. `add_issue_labels` with the discovered ID — no separate code path
   for org labels.

For non-org owners (regular users) `list_repo_labels` returns only
repo-scoped labels; the org fetch is skipped automatically. No caller
change needed.

## Edge cases

- **Owner is a user, not an org.** The `/orgs/{user}/labels` endpoint
  returns 404; the handler maps that to an empty org-label slice and
  the response still succeeds with `scope:"repo"` entries only.
- **Org has no labels.** Empty `org` slice merged in — response is
  effectively just the repo labels.
- **Pagination.** `page` and `limit` apply to both the repo and the org
  fetch. To paginate across all 67 labels above, walk pages of the
  merged response.
