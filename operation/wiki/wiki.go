// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/operation/repo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListWikiPagesToolName    = "list_wiki_pages"
	GetWikiPageToolName      = "get_wiki_page"
	GetWikiRevisionsToolName = "get_wiki_revisions"
	CreateWikiPageToolName   = "create_wiki_page"
	UpdateWikiPageToolName   = "update_wiki_page"
	DeleteWikiPageToolName   = "delete_wiki_page"
	defaultLimit             = 30
)

var ListWikiPagesTool = mcp.NewTool(ListWikiPagesToolName,
	mcp.WithDescription("List wiki pages with page/limit pagination; returns has_next."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultLimit), mcp.Min(1)),
)

var GetWikiPageTool = mcp.NewTool(GetWikiPageToolName,
	mcp.WithDescription("Get one wiki page as decoded Markdown. Optional start_line/end_line bound content; total_lines is always returned."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithNumber("start_line", mcp.Description("First line to return (1-based, inclusive)"), mcp.Min(1)),
	mcp.WithNumber("end_line", mcp.Description("Last line to return (1-based, inclusive)"), mcp.Min(1)),
)

var GetWikiRevisionsTool = mcp.NewTool(GetWikiRevisionsToolName,
	mcp.WithDescription("Get a wiki page's revision history with page/limit pagination; returns has_next."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultLimit), mcp.Min(1)),
)

var CreateWikiPageTool = mcp.NewTool(CreateWikiPageToolName,
	mcp.WithDescription("Create a wiki page. Slash-separated titles such as Parent/Child are a flat naming convention: Forgejo stores no parent-child relationship and does not create a parent page automatically. Creating an existing title overwrites it. Use the returned page_name verbatim; never derive it from title."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("title", mcp.Required(), mcp.Description(params.WikiTitle)),
	mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
	mcp.WithString("message", mcp.Description(params.Message)),
)

var UpdateWikiPageTool = mcp.NewTool(UpdateWikiPageToolName,
	mcp.WithDescription("Update a wiki page. Writes are last-writer-wins; Forgejo provides no optimistic concurrency precondition."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithString("title", mcp.Description(params.WikiTitle)),
	mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
	mcp.WithString("message", mcp.Description(params.Message)),
)

var DeleteWikiPageTool = mcp.NewTool(DeleteWikiPageToolName,
	mcp.WithDescription("Delete one wiki page by its server-normalized page_name."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListWikiPagesTool, ListWikiPagesFn)
	s.AddTool(GetWikiPageTool, GetWikiPageFn)
	s.AddTool(GetWikiRevisionsTool, GetWikiRevisionsFn)
	s.AddTool(CreateWikiPageTool, CreateWikiPageFn)
	s.AddTool(UpdateWikiPageTool, UpdateWikiPageFn)
	s.AddTool(DeleteWikiPageTool, DeleteWikiPageFn)
}

func pagination(args map[string]any) (int, int) {
	page, limit := 1, defaultLimit
	if value, ok := args["page"].(float64); ok && value > 0 {
		page = int(value)
	}
	if value, ok := args["limit"].(float64); ok && value > 0 {
		limit = int(value)
	}
	return page, limit
}

type wikiPageSummary struct {
	Title    string `json:"title"`
	PageName string `json:"page_name"`
	SubURL   string `json:"sub_url"`
}

type wikiRevisionSummary struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Message string `json:"message"`
}

type wikiWriteResult struct {
	Title     string `json:"title"`
	PageName  string `json:"page_name"`
	CommitSHA string `json:"commit_sha"`
}

func pageSummary(page forgejo.WikiPageMeta) wikiPageSummary {
	return wikiPageSummary{Title: page.Title, PageName: page.SubURL, SubURL: page.SubURL}
}

func writeResult(page *forgejo.WikiPage) wikiWriteResult {
	return wikiWriteResult{Title: page.Title, PageName: page.SubURL, CommitSHA: page.LastCommit.SHA}
}

func ListWikiPagesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListWikiPagesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repoName, _ := args["repo"].(string)
	page, limit := pagination(args)
	pages, err := forgejo.ListWikiPages(ctx, owner, repoName, page, limit)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list wiki pages: %w", err))
	}
	hasNext := false
	if len(pages) == limit {
		next, err := forgejo.ListWikiPages(ctx, owner, repoName, page+1, limit)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("probe next wiki page: %w", err))
		}
		hasNext = len(next) > 0
	}
	resultPages := make([]wikiPageSummary, len(pages))
	for i, wikiPage := range pages {
		resultPages[i] = pageSummary(wikiPage)
	}
	return to.TextResult(struct {
		Pages   []wikiPageSummary `json:"pages"`
		Page    int               `json:"page"`
		HasNext bool              `json:"has_next"`
	}{resultPages, page, hasNext})
}

func GetWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repoName, _ := args["repo"].(string)
	pageName, _ := args["page_name"].(string)
	page, err := forgejo.GetWikiPage(ctx, owner, repoName, pageName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get wiki page: %w", err))
	}
	decoded, err := base64.StdEncoding.DecodeString(page.ContentBase64)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("decode wiki page content: %w", err))
	}
	content := string(decoded)
	total := len(strings.Split(content, "\n"))
	start, end := 0, 0
	startValue, hasStart := args["start_line"].(float64)
	endValue, hasEnd := args["end_line"].(float64)
	if hasStart || hasEnd {
		start, end = int(startValue), int(endValue)
		content, err = repo.SliceLines(content, start, end)
		if err != nil {
			return to.ErrorResult(err)
		}
		if start <= 0 {
			start = 1
		}
		if end <= 0 || end > total {
			end = total
		}
	}
	return to.TextResult(struct {
		Title      string `json:"title"`
		PageName   string `json:"page_name"`
		Content    string `json:"content"`
		CommitSHA  string `json:"commit_sha"`
		TotalLines int    `json:"total_lines"`
		StartLine  int    `json:"start_line,omitempty"`
		EndLine    int    `json:"end_line,omitempty"`
	}{page.Title, page.SubURL, content, page.LastCommit.SHA, total, start, end})
}

func GetWikiRevisionsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repoName, _ := args["repo"].(string)
	pageName, _ := args["page_name"].(string)
	page, limit := pagination(args)
	revisions, err := forgejo.GetWikiPageRevisions(ctx, owner, repoName, pageName, page, limit)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get wiki revisions: %w", err))
	}
	hasNext := false
	if len(revisions.Commits) == limit {
		next, err := forgejo.GetWikiPageRevisions(ctx, owner, repoName, pageName, page+1, limit)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("probe next wiki revision page: %w", err))
		}
		hasNext = len(next.Commits) > 0
	}
	resultRevisions := make([]wikiRevisionSummary, len(revisions.Commits))
	for i, revision := range revisions.Commits {
		resultRevisions[i] = wikiRevisionSummary{SHA: revision.SHA, Author: revision.Author.Name, Message: revision.Message}
	}
	return to.TextResult(struct {
		Revisions []wikiRevisionSummary `json:"revisions"`
		Page      int                   `json:"page"`
		HasNext   bool                  `json:"has_next"`
	}{resultRevisions, page, hasNext})
}

func CreateWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repoName, _ := args["repo"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	message, _ := args["message"].(string)
	if message == "" {
		message = fmt.Sprintf("Create wiki page '%s'", title)
	}
	page, err := forgejo.CreateWikiPage(ctx, owner, repoName, title, base64.StdEncoding.EncodeToString([]byte(content)), message)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create wiki page: %w", err))
	}
	return to.TextResult(writeResult(page))
}

func UpdateWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repoName, _ := args["repo"].(string)
	pageName, _ := args["page_name"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	message, _ := args["message"].(string)
	if title == "" {
		current, err := forgejo.GetWikiPage(ctx, owner, repoName, pageName)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("read wiki page before update: %w", err))
		}
		title = current.Title
	}
	if message == "" {
		message = fmt.Sprintf("Update wiki page '%s'", pageName)
	}
	page, err := forgejo.EditWikiPage(ctx, owner, repoName, pageName, title, base64.StdEncoding.EncodeToString([]byte(content)), message)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update wiki page: %w", err))
	}
	return to.TextResult(writeResult(page))
}

func DeleteWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repoName, _ := args["repo"].(string)
	pageName, _ := args["page_name"].(string)
	if err := forgejo.DeleteWikiPage(ctx, owner, repoName, pageName); err != nil {
		return to.ErrorResult(fmt.Errorf("delete wiki page: %w", err))
	}
	return to.TextResult(struct {
		Deleted  bool   `json:"deleted"`
		PageName string `json:"page_name"`
	}{true, pageName})
}
