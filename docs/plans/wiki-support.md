# Wiki Support Implementation Plan for forgejo-mcp

## Summary

Complete wiki support in forgejo-mcp requires wiki API methods in the upstream forgejo-sdk. This plan outlines the upstream contribution strategy and downstream integration.

## Current State

**forgejo-mcp** (`operation/wiki/wiki.go`) defines 3 wiki tools but uses SDK methods that don't exist:

- `ListWikiPages()`, `CreateWikiPage()`, `EditWikiPage()` - undefined
- `CreateWikiPageOption`, `EditWikiPageOption` - undefined
- **Build fails** with undefined method errors

**forgejo-sdk v2.0.0-v2.2.0** has no wiki page API methods.

## Target State

### forgejo-sdk (upstream)

Add wiki page methods matching the Forgejo/Gitea API:

- `ListWikiPages(owner, repo string, opts ListWikiPagesOptions) ([]*WikiPageMeta, *Response, error)`
- `GetWikiPage(owner, repo, pageName string) (*WikiPage, *Response, error)`
- `GetWikiPageRevisions(owner, repo, pageName string, opts ListOptions) ([]*WikiCommit, *Response, error)`
- `CreateWikiPage(owner, repo string, opt CreateWikiPageOption) (*WikiPage, *Response, error)`
- `EditWikiPage(owner, repo, pageName string, opt EditWikiPageOption) (*WikiPage, *Response, error)`
- `DeleteWikiPage(owner, repo, pageName string) (*Response, error)`

### forgejo-mcp (downstream)

Once SDK is updated, complete wiki support with 6 tools:

| Tool | Description |
|------|-------------|
| `list_wiki_pages` | List all wiki pages in a repository |
| `get_wiki_page` | Get a wiki page content and metadata |
| `get_wiki_revisions` | Get revisions history of a wiki page |
| `create_wiki_page` | Create a new wiki page |
| `update_wiki_page` | Update an existing wiki page |
| `delete_wiki_page` | Delete a wiki page |

---

## Part 1: Upstream Contribution to forgejo-sdk

### Repository

- **URL**: <https://codeberg.org/mvdkleijn/forgejo-sdk>
- **Branch**: `main`

### Files to Create/Modify

#### 1. `forgejo/wiki.go` (NEW FILE)

```go
// Copyright 2024 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forgejo

import (
    "fmt"
    "net/url"
    "time"
)

// WikiPage represents a wiki page
type WikiPage struct {
    Title      string      `json:"title"`
    PageName   string      `json:"page_name"`
    SubURL     string      `json:"sub_url"`
    Content    string      `json:"content_base64"` // base64 encoded
    CommitSHA  string      `json:"commit_sha"`
    Sidebar    string      `json:"sidebar"`
    Footer     string      `json:"footer"`
    LastCommit *WikiCommit `json:"last_commit,omitempty"`
}

// WikiPageMeta represents wiki page metadata (for list operations)
type WikiPageMeta struct {
    Title      string      `json:"title"`
    PageName   string      `json:"page_name"`
    SubURL     string      `json:"sub_url"`
    LastCommit *WikiCommit `json:"last_commit,omitempty"`
}

// WikiCommit represents a wiki commit
type WikiCommit struct {
    SHA       string         `json:"sha"`
    Author    *WikiAuthor    `json:"author"`
    Committer *WikiCommitter `json:"committer"`
    Message   string         `json:"message"`
}

// WikiAuthor represents a wiki commit author
type WikiAuthor struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// WikiCommitter represents a wiki commit committer
type WikiCommitter struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// CreateWikiPageOption options for creating a wiki page
type CreateWikiPageOption struct {
    Title   string `json:"title"`
    Content string `json:"content_base64"` // base64 encoded
    Message string `json:"message,omitempty"`
}

// EditWikiPageOption options for editing a wiki page
type EditWikiPageOption struct {
    Title   string `json:"title,omitempty"`
    Content string `json:"content_base64,omitempty"` // base64 encoded
    Message string `json:"message,omitempty"`
}

// ListWikiPages lists all wiki pages in a repository
func (c *Client) ListWikiPages(owner, repo string, opts ListOptions) ([]*WikiPageMeta, *Response, error) {
    opt := newListOptions(opts)
    pages := make([]*WikiPageMeta, 0)
    resp, err := c.getParsedResponse("GET",
        fmt.Sprintf("/repos/%s/%s/wiki/pages?%s",
            url.PathEscape(owner), url.PathEscape(repo), opt.toQuery()),
        nil, nil, &pages)
    return pages, resp, err
}

// GetWikiPage gets a wiki page
func (c *Client) GetWikiPage(owner, repo, pageName string) (*WikiPage, *Response, error) {
    page := new(WikiPage)
    resp, err := c.getParsedResponse("GET",
        fmt.Sprintf("/repos/%s/%s/wiki/page/%s",
            url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(pageName)),
        nil, nil, page)
    return page, resp, err
}

// GetWikiPageRevisions gets the revision history of a wiki page
func (c *Client) GetWikiPageRevisions(owner, repo, pageName string, opts ListOptions) ([]*WikiCommit, *Response, error) {
    opt := newListOptions(opts)
    commits := make([]*WikiCommit, 0)
    resp, err := c.getParsedResponse("GET",
        fmt.Sprintf("/repos/%s/%s/wiki/revisions/%s?%s",
            url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(pageName), opt.toQuery()),
        nil, nil, &commits)
    return commits, resp, err
}

// CreateWikiPage creates a new wiki page
func (c *Client) CreateWikiPage(owner, repo string, opt CreateWikiPageOption) (*WikiPage, *Response, error) {
    page := new(WikiPage)
    resp, err := c.getParsedResponse("POST",
        fmt.Sprintf("/repos/%s/%s/wiki/new",
            url.PathEscape(owner), url.PathEscape(repo)),
        jsonHeader, opt, page)
    return page, resp, err
}

// EditWikiPage edits a wiki page
func (c *Client) EditWikiPage(owner, repo, pageName string, opt EditWikiPageOption) (*WikiPage, *Response, error) {
    page := new(WikiPage)
    resp, err := c.getParsedResponse("PATCH",
        fmt.Sprintf("/repos/%s/%s/wiki/page/%s",
            url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(pageName)),
        jsonHeader, opt, page)
    return page, resp, err
}

// DeleteWikiPage deletes a wiki page
func (c *Client) DeleteWikiPage(owner, repo, pageName string) (*Response, error) {
    resp, err := c.getResponse("DELETE",
        fmt.Sprintf("/repos/%s/%s/wiki/page/%s",
            url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(pageName)),
        nil, nil)
    return resp, err
}
```

#### 2. `forgejo/wiki_test.go` (NEW FILE)

Add tests for all wiki operations following the SDK's testing patterns.

### API Reference

Forgejo/Gitea Wiki API endpoints:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/repos/{owner}/{repo}/wiki/pages` | List all wiki pages |
| GET | `/repos/{owner}/{repo}/wiki/page/{pageName}` | Get a wiki page |
| GET | `/repos/{owner}/{repo}/wiki/revisions/{pageName}` | Get wiki page revisions |
| POST | `/repos/{owner}/{repo}/wiki/new` | Create a wiki page |
| PATCH | `/repos/{owner}/{repo}/wiki/page/{pageName}` | Edit a wiki page |
| DELETE | `/repos/{owner}/{repo}/wiki/page/{pageName}` | Delete a wiki page |

### Pull Request Checklist

- [ ] Add `wiki.go` with types and methods
- [ ] Add `wiki_test.go` with tests
- [ ] Update CHANGELOG if required
- [ ] Ensure CI passes
- [ ] Follow SDK coding conventions

---

## Part 2: Downstream Integration in forgejo-mcp

After forgejo-sdk releases with wiki support:

### 1. Update go.mod

```bash
go get codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2@latest
```

### 2. Update `operation/wiki/wiki.go`

Add 3 new tools:

- `get_wiki_page` - Get wiki page content and metadata
- `get_wiki_revisions` - Get revision history
- `delete_wiki_page` - Delete a wiki page

Fix 3 existing tools to use correct SDK methods.

### 3. Add to params if needed

```go
// Wiki parameters (already exist)
WikiTitle   = "Wiki page title"
WikiContent = "Wiki page content"
WikiPage    = "Wiki page name"
```

---

## Timeline Estimate

1. **SDK Contribution**: 1-2 weeks (including review cycles)
2. **forgejo-mcp Integration**: 1 day (after SDK release)

---

## Feature Request Comment for forgejo-sdk

Copy the following comment to submit to the Codeberg feature request:

---

### Wiki API Support for forgejo-sdk

**Summary**: Add wiki page management methods to the forgejo-sdk.

**Background**: The [forgejo-mcp](https://codeberg.org/goern/forgejo-mcp) project provides an MCP (Model Context Protocol) server for Forgejo integration. We're implementing wiki support but discovered that the forgejo-sdk (v2.0.0-v2.2.0) doesn't include wiki page API methods.

**Forgejo/Gitea Wiki API endpoints exist and are documented**:

- `GET /repos/{owner}/{repo}/wiki/pages` - List wiki pages
- `GET /repos/{owner}/{repo}/wiki/page/{pageName}` - Get a wiki page
- `GET /repos/{owner}/{repo}/wiki/revisions/{pageName}` - Get page revisions
- `POST /repos/{owner}/{repo}/wiki/new` - Create a wiki page
- `PATCH /repos/{owner}/{repo}/wiki/page/{pageName}` - Edit a wiki page
- `DELETE /repos/{owner}/{repo}/wiki/page/{pageName}` - Delete a wiki page

**Requested SDK methods**:

```go
// Types
type WikiPage struct { ... }
type WikiPageMeta struct { ... }
type WikiCommit struct { ... }
type CreateWikiPageOption struct { ... }
type EditWikiPageOption struct { ... }

// Methods on Client
func (c *Client) ListWikiPages(owner, repo string, opts ListOptions) ([]*WikiPageMeta, *Response, error)
func (c *Client) GetWikiPage(owner, repo, pageName string) (*WikiPage, *Response, error)
func (c *Client) GetWikiPageRevisions(owner, repo, pageName string, opts ListOptions) ([]*WikiCommit, *Response, error)
func (c *Client) CreateWikiPage(owner, repo string, opt CreateWikiPageOption) (*WikiPage, *Response, error)
func (c *Client) EditWikiPage(owner, repo, pageName string, opt EditWikiPageOption) (*WikiPage, *Response, error)
func (c *Client) DeleteWikiPage(owner, repo, pageName string) (*Response, error)
```

**Reference implementation**: The [gitea-mcp](https://gitea.com/gitea/gitea-mcp) project implements wiki support using raw HTTP calls. We'd prefer to use proper SDK methods for consistency and maintainability.

**Willingness to contribute**: We're happy to submit a PR adding these methods if the maintainers are open to it.

---

## Temporary Workaround

Until the SDK is updated, the current `operation/wiki/wiki.go` in forgejo-mcp cannot build. Options:

1. **Comment out wiki.go** - Disable wiki tools temporarily
2. **Implement raw HTTP calls** - Similar to gitea-mcp approach (more work, but unblocks immediately)
3. **Wait for SDK** - Cleanest approach but blocks wiki functionality
