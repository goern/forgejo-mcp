# Demo: list_repo_milestones + list_repo_labels

*2026-03-06T18:37:02Z by Showboat 0.6.1*
<!-- showboat-id: cb549ef9-b74d-41a7-9c9b-aeedc81f7c01 -->

## Background

Two existing forgejo-mcp tools require numeric IDs:

- `update_issue` — needs a **milestone ID** (e.g. `42`)
- `add_issue_labels` — needs **label IDs** (e.g. `[7, 12]`)

Without discovery tools, an AI agent has no way to resolve human-readable names to IDs autonomously.

This demo shows the two new tools that close that gap:
- `list_repo_milestones` — returns milestones with their numeric IDs
- `list_repo_labels` — returns labels with their numeric IDs

## Setup

We call the Codeberg Forgejo API directly — the same endpoints the tools wrap — to show realistic output.

```bash

curl -s -H "Authorization: token $CODEBERG_TOKEN"   'https://codeberg.org/api/v1/repos/goern/forgejo-mcp/milestones?state=open&limit=10'   | python3 -c "
import sys, json
milestones = json.load(sys.stdin)
if not milestones:
    print('(no open milestones)')
else:
    for m in milestones:
        print(f\"id={m['id']}  title={m['title']!r}  state={m['state']}  open_issues={m['open_issues']}\")
"
```

```output
(no open milestones)
```

## list_repo_labels in action

Now discover all labels — their IDs are what `add_issue_labels` requires.

```bash

curl -s -H "Authorization: token $CODEBERG_TOKEN"   'https://codeberg.org/api/v1/repos/goern/forgejo-mcp/labels?limit=50'   | python3 -c "
import sys, json
labels = json.load(sys.stdin)
if not labels:
    print('(no labels defined)')
else:
    for l in labels:
        print(f\"id={l['id']}  name={l['name']!r}  color=#{l['color']}\")
"
```

```output
id=335073  name='Compat/Breaking'  color=#c62828
id=335055  name='Kind/Bug'  color=#ee0701
id=335070  name='Kind/Documentation'  color=#37474f
id=335061  name='Kind/Enhancement'  color=#84b6eb
id=335058  name='Kind/Feature'  color=#0288d1
id=335064  name='Kind/Security'  color=#9c27b0
id=335067  name='Kind/Testing'  color=#795548
id=335097  name='Priority/Critical'  color=#b71c1c
id=335100  name='Priority/High'  color=#d32f2f
id=335106  name='Priority/Low'  color=#4caf50
id=335103  name='Priority/Medium'  color=#e64a19
id=335082  name='Reviewed/Confirmed'  color=#795548
id=335076  name='Reviewed/Duplicate'  color=#616161
id=335079  name='Reviewed/Invalid'  color=#546e7a
id=335085  name="Reviewed/Won't Fix"  color=#eeeeee
id=335094  name='Status/Abandoned'  color=#222222
id=335091  name='Status/Blocked'  color=#880e4f
id=335088  name='Status/Need More Info'  color=#424242
```

## How an AI agent uses these tools

With the two discovery tools, a fully autonomous workflow becomes possible:

1. Call `list_repo_milestones` → get `{"id": 3, "title": "v2.13.0", ...}`
2. Call `list_repo_labels` → get `{"id": 335055, "name": "Kind/Bug"}, {"id": 335061, "name": "Kind/Enhancement"}`
3. Pass IDs to `update_issue(milestone="3")` and `add_issue_labels(labels="335055,335061")`

No manual ID lookup required — fully autonomous workflow.
