# Demo: add_issue_labels + remove_issue_labels

*2026-03-20T08:30:00Z by Showboat 0.6.1*
<!-- showboat-id: d7e1a42b-issue-labels-demo-2026 -->

## What these tools do

Two issue label management tools let AI agents programmatically tag and untag issues without leaving the MCP/CLI workflow:

- `add_issue_labels` — Add one or more labels to an issue (by numeric label ID)
- `remove_issue_labels` — Remove one or more labels from an issue (by numeric label ID)

Both tools require numeric label IDs. Use `list_repo_labels` first to discover the ID↔name mapping, then pass IDs to the mutation tools. This enables fully autonomous label triage workflows.

## Setup

Set `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` environment variables (or use direnv), then build:

```bash
make build
```

## CLI mode: parameter schemas

```bash
./forgejo-mcp --cli add_issue_labels --help 2>/dev/null
```

```output
Tool: add_issue_labels
Description: Add labels to issue

Parameters:
  index                number     required   Issue index
  labels               string     required   Labels to add (comma-separated)
  owner                string     required   Repository owner
  repo                 string     required   Repository name
```

```bash
./forgejo-mcp --cli remove_issue_labels --help 2>/dev/null
```

```output
Tool: remove_issue_labels
Description: Remove labels from issue

Parameters:
  index                number     required   Issue index
  labels               string     required   Labels to remove (comma-separated label IDs)
  owner                string     required   Repository owner
  repo                 string     required   Repository name
```

## End-to-end workflow: discover, add, remove

### Step 1: Discover available labels

```bash
./forgejo-mcp --cli list_repo_labels \
  --args '{"owner":"goern","repo":"forgejo-mcp"}' 2>/dev/null
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

### Step 2: Check current labels on an issue

```bash
./forgejo-mcp --cli get_issue_by_index \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":92}' 2>/dev/null
```

```output
  Issue #92: Add support for creating organisations
  State: open
  Labels:
    - Kind/Enhancement (id=335061)
    - Kind/Feature (id=335058)
```

### Step 3: Add a label (Priority/Medium = 335103)

```bash
./forgejo-mcp --cli add_issue_labels \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":92,"labels":"335103"}' 2>/dev/null
```

```output
  Issue #92: Add support for creating organisations
  Labels after add:
    - Kind/Enhancement (id=335061)
    - Kind/Feature (id=335058)
    - Priority/Medium (id=335103)
```

### Step 4: Remove the label

```bash
./forgejo-mcp --cli remove_issue_labels \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":92,"labels":"335103"}' 2>/dev/null
```

```output
  Issue #92: Add support for creating organisations
  Labels after remove:
    - Kind/Enhancement (id=335061)
    - Kind/Feature (id=335058)
```

## MCP stdio mode: adding multiple labels at once

An MCP client can add several labels in a single call by passing comma-separated IDs:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{
  "name":"add_issue_labels",
  "arguments":{"owner":"goern","repo":"forgejo-mcp","index":92,"labels":"335067,335106"}
}}' | ./forgejo-mcp 2>/dev/null
```

```output
  Issue #92: Add support for creating organisations
    - Kind/Enhancement (id=335061)
    - Kind/Feature (id=335058)
    - Kind/Testing (id=335067)
    - Priority/Low (id=335106)
```

Multiple labels can also be removed in one call the same way — `remove_issue_labels` iterates and deletes each one.

## Autonomous triage workflow

An AI agent can implement a full label triage pipeline:

1. `list_repo_labels` — Build a label ID lookup table
2. `list_repo_issues` — Fetch open issues
3. For each issue, analyze title/body and decide labels
4. `add_issue_labels` — Apply labels (e.g., `"335055,335100"` for Kind/Bug + Priority/High)
5. `remove_issue_labels` — Fix misclassifications by removing wrong labels

No human needed to look up IDs or visit the web UI — the agent discovers IDs at runtime and applies them programmatically.
