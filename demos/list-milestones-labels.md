# Demo: list_repo_milestones + list_repo_labels

*Created by Showboat 0.6.1*
<!-- showboat-id: a1b2c3d4-list-milestones-labels-demo-clean -->

## Background

Two existing forgejo-mcp tools require numeric IDs with no discovery counterparts:

- `update_issue` — needs a milestone **ID** (integer)
- `add_issue_labels` — needs label **IDs** (comma-separated integers)

Without discovery tools, autonomous AI workflows are blocked. The two new tools in this PR close that gap.

## Setup

Set connection details via environment variables, then build and invoke:

    export FORGEJO_URL=https://codeberg.org
    export FORGEJO_ACCESS_TOKEN=<your-token>
    go build -o ./forgejo-mcp .

## Available tools (after this PR)

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=$CODEBERG_TOKEN ./forgejo-mcp --cli list 2>/dev/null | grep -E 'list_repo_(labels|milestones)'
```

```output
  list_repo_labels                         List repository labels
  list_repo_milestones                     List repository milestones
```

## list_repo_labels — discover label names and IDs

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=$CODEBERG_TOKEN ./forgejo-mcp --cli list_repo_labels --args '{"owner":"goern","repo":"forgejo-mcp"}' 2>/dev/null | python3 -c "
import sys, json
result = json.load(sys.stdin)
for l in json.loads(result[0]['text']).get('Result', []):
    print(f\"  id={l['id']:7d}  {l['name']:<30s}  #{l['color']}\")
"
```

```output
  id= 335073  Compat/Breaking                 #c62828
  id= 335055  Kind/Bug                        #ee0701
  id= 335070  Kind/Documentation              #37474f
  id= 335061  Kind/Enhancement                #84b6eb
  id= 335058  Kind/Feature                    #0288d1
  id= 335064  Kind/Security                   #9c27b0
  id= 335067  Kind/Testing                    #795548
  id= 335097  Priority/Critical               #b71c1c
  id= 335100  Priority/High                   #d32f2f
  id= 335106  Priority/Low                    #4caf50
  id= 335103  Priority/Medium                 #e64a19
  id= 335082  Reviewed/Confirmed              #795548
  id= 335076  Reviewed/Duplicate              #616161
  id= 335079  Reviewed/Invalid                #546e7a
  id= 335085  Reviewed/Won't Fix              #eeeeee
  id= 335094  Status/Abandoned                #222222
  id= 335091  Status/Blocked                  #880e4f
  id= 335088  Status/Need More Info           #424242
```

## list_repo_milestones — discover milestone names and IDs

```bash
FORGEJO_URL=https://codeberg.org FORGEJO_ACCESS_TOKEN=$CODEBERG_TOKEN ./forgejo-mcp --cli list_repo_milestones --args '{"owner":"goern","repo":"forgejo-mcp","state":"all"}' 2>/dev/null | python3 -c "
import sys, json
result = json.load(sys.stdin)
ms = json.loads(result[0]['text']).get('Result', [])
print('  (no milestones defined for this repository)') if not ms else [print(f\"  id={m['id']}  {m['title']!r}  state={m['state']}\") for m in ms]
"
```

```output
  id=67153  'test_milestone for shoboat'  state=open
```

## End-to-end: autonomous label + milestone assignment

With the discovered IDs, the mutating tools can be called without any manual lookup:

- `add_issue_labels` with `labels="335055,335058"` (Kind/Bug, Kind/Feature)
- `update_issue` with `milestone="<id>"` once milestones are defined

Fully autonomous workflow — no human needed to look up IDs.
