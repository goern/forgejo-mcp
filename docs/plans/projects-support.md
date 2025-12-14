# Projects Support Implementation Plan for forgejo-mcp

## Issue Reference
- **Issue**: [#42 - Add support for Projects](https://codeberg.org/goern/forgejo-mcp/issues/42)
- **Requester**: BasdP (Bas du Pré)
- **Use Case**: "Let AI clean up my projects, for instance to ask it to sort the project board by milestone"

## User Epic

> **As a** Forgejo user with project boards,
> **I want to** manage and organize my projects through AI assistance via the MCP server,
> **So that** I can automate project board maintenance tasks like sorting, organizing, and cleaning up my project boards without manual effort.

## Current State: BLOCKED

**The Forgejo/Gitea Projects API does not exist yet.**

### Upstream Status
- **Feature Request**: [Gitea #14299](https://github.com/go-gitea/gitea/issues/14299) (open since Jan 2021, 43 upvotes)
- **Target Release**: Gitea 1.26.0 (milestone at 85% progress)
- **Draft PRs**:
  - [PR #28111](https://github.com/go-gitea/gitea/pull/28111) - Core project endpoints (draft)
  - [PR #28209](https://github.com/go-gitea/gitea/pull/28209) - Board endpoints (draft)

### Proposed API Endpoints (from draft PRs)

**Projects:**
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/user/projects` | Create user project |
| GET | `/user/projects` | List user projects |
| POST | `/orgs/{org}/projects` | Create org project |
| GET | `/orgs/{org}/projects` | List org projects |
| POST | `/repos/{owner}/{repo}/projects` | Create repo project |
| GET | `/repos/{owner}/{repo}/projects` | List repo projects |
| GET | `/projects/{id}` | Get project details |
| PATCH | `/projects/{id}` | Update project |
| DELETE | `/projects/{id}` | Delete project |

**Boards (Columns):**
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/projects/{projectId}/boards` | Create board |
| GET | `/projects/{projectId}/boards` | List boards |
| GET | `/projects/boards/{boardId}` | Get board |
| PATCH | `/projects/boards/{boardId}` | Update board |
| DELETE | `/projects/boards/{boardId}` | Delete board |

**Not yet defined:** Board items (cards), issue-to-board operations, sorting

---

## Approach: Wait for Upstream

### Immediate Actions

1. **Update Issue #42** with research findings and blocked status
2. **Add label** `Status/Blocked` to indicate external dependency
3. **Track upstream** - monitor Gitea 1.26.0 release and forgejo-sdk updates

### When Upstream Becomes Available

#### Phase 1: SDK Contribution (if needed)

Check if `forgejo-sdk` adds Projects support. If not, contribute:

**Files to create in forgejo-sdk:**
- `forgejo/project.go` - Types and methods
- `forgejo/project_test.go` - Tests

**Required SDK Types:**
```go
type Project struct {
    ID          int64  `json:"id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    BoardType   string `json:"board_type"`
    // ...
}

type ProjectBoard struct {
    ID        int64  `json:"id"`
    Title     string `json:"title"`
    ProjectID int64  `json:"project_id"`
    // ...
}

type CreateProjectOption struct { ... }
type EditProjectOption struct { ... }
```

**Required SDK Methods:**
```go
func (c *Client) ListUserProjects(opts ListOptions) ([]*Project, *Response, error)
func (c *Client) ListOrgProjects(org string, opts ListOptions) ([]*Project, *Response, error)
func (c *Client) ListRepoProjects(owner, repo string, opts ListOptions) ([]*Project, *Response, error)
func (c *Client) GetProject(id int64) (*Project, *Response, error)
func (c *Client) CreateRepoProject(owner, repo string, opt CreateProjectOption) (*Project, *Response, error)
func (c *Client) EditProject(id int64, opt EditProjectOption) (*Project, *Response, error)
func (c *Client) DeleteProject(id int64) (*Response, error)
func (c *Client) ListProjectBoards(projectID int64, opts ListOptions) ([]*ProjectBoard, *Response, error)
// ... board methods
```

#### Phase 2: forgejo-mcp Implementation

**Create `/operation/project/project.go`:**

| Tool | Description |
|------|-------------|
| `list_user_projects` | List projects for authenticated user |
| `list_org_projects` | List projects in an organization |
| `list_repo_projects` | List projects in a repository |
| `get_project` | Get project details |
| `create_project` | Create a new project |
| `update_project` | Update project settings |
| `delete_project` | Delete a project |
| `list_project_boards` | List columns/boards in a project |
| `get_project_board` | Get board details |
| `create_project_board` | Create a new board/column |
| `update_project_board` | Update board properties |
| `delete_project_board` | Delete a board |

**Register in `/operation/operation.go`:**
```go
import "codeberg.org/goern/forgejo-mcp/operation/project"

func RegisterTool(s *server.MCPServer) {
    // ... existing registrations
    project.RegisterTool(s)
}
```

**Add parameters to `/operation/params/params.go`:**
```go
const (
    ProjectID    = "Project ID"
    ProjectTitle = "Project title"
    BoardID      = "Board/column ID"
    BoardTitle   = "Board/column title"
)
```

---

## Monitoring Checklist

- [ ] Watch [Gitea #14299](https://github.com/go-gitea/gitea/issues/14299) for updates
- [ ] Watch [PR #28111](https://github.com/go-gitea/gitea/pull/28111) for merge
- [ ] Watch [PR #28209](https://github.com/go-gitea/gitea/pull/28209) for merge
- [ ] Check Gitea 1.26.0 release notes when released
- [ ] Check forgejo-sdk for Projects support after Gitea release
- [ ] If SDK doesn't add support, contribute upstream

---

## References

- [Forgejo Projects Documentation](https://forgejo.org/docs/latest/user/project/)
- [Gitea Kanban Board PR #8346](https://github.com/go-gitea/gitea/pull/8346) (original implementation)
- [Wiki Support Plan](./wiki-support.md) (similar pattern)
