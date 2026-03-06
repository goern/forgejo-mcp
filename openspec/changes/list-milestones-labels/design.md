## Approach

Follow the existing pattern used by `list_repo_issues` and `list_my_repos` in the codebase.

### Tool Registration

Both tools are registered in `operation/issue/issue.go` via `RegisterTool()`.

New tool name constants:
```go
ListRepoMilestonesToolName = "list_repo_milestones"
ListRepoLabelsToolName     = "list_repo_labels"
```

### list_repo_milestones

**Input parameters:**
| Name  | Type   | Required | Description                                              |
|-------|--------|----------|----------------------------------------------------------|
| owner | string | yes      | Repository owner (user or organisation)                 |
| repo  | string | yes      | Repository name                                          |
| page  | number | yes      | Page number (default: 1, min: 1)                         |
| limit | number | yes      | Results per page (default: 100, min: 1)                  |
| state | string | no       | Filter by state: `open`, `closed`, `all` (default: `open`) |

**Implementation:**
Call `client.ListRepoMilestones(owner, repo, forgejo.ListMilestoneOption{...})`.
Return a JSON array with at minimum `id`, `title`, `description`, `state`, `open_issues`, `closed_issues`.

### list_repo_labels

**Input parameters:**
| Name  | Type   | Required | Description                              |
|-------|--------|----------|------------------------------------------|
| owner | string | yes      | Repository owner (user or organisation) |
| repo  | string | yes      | Repository name                          |
| page  | number | yes      | Page number (default: 1, min: 1)         |
| limit | number | yes      | Results per page (default: 100, min: 1)  |

**Implementation:**
Call `client.ListRepoLabels(owner, repo, forgejo.ListLabelsOptions{...})`.
Return a JSON array with at minimum `id`, `name`, `color`, `description`.

### File changes

- `operation/issue/issue.go`: add tool definitions, handlers `ListRepoMilestonesFn` and `ListRepoLabelsFn`, register in `RegisterTool`
