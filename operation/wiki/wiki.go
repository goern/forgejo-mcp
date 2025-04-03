package wiki

import (
	"context"
	"fmt"

	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
	"forgejo.org/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListWikiPagesToolName   = "list_wiki_pages"
	CreateWikiPageToolName  = "create_wiki_page"
	UpdateWikiPageToolName  = "update_wiki_page"
)

var (
	ListWikiPagesTool = mcp.NewTool(
		ListWikiPagesToolName,
		mcp.WithDescription("list wiki pages"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
	)

	CreateWikiPageTool = mcp.NewTool(
		CreateWikiPageToolName,
		mcp.WithDescription("create a new wiki page"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("title", mcp.Required(), mcp.Description("wiki page title")),
		mcp.WithString("content", mcp.Required(), mcp.Description("wiki page content")),
		mcp.WithString("message", mcp.Description("commit message")),
	)

	UpdateWikiPageTool = mcp.NewTool(
		UpdateWikiPageToolName,
		mcp.WithDescription("update an existing wiki page"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("page_name", mcp.Required(), mcp.Description("name of the wiki page to update")),
		mcp.WithString("title", mcp.Description("new wiki page title")),
		mcp.WithString("content", mcp.Required(), mcp.Description("new wiki page content")),
		mcp.WithString("message", mcp.Description("commit message")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(ListWikiPagesTool, ListWikiPagesFn)
	s.AddTool(CreateWikiPageTool, CreateWikiPageFn)
	s.AddTool(UpdateWikiPageTool, UpdateWikiPageFn)
}

func ListWikiPagesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListWikiPagesFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)

	wikiPages, _, err := forgejo.Client().ListWikiPages(owner, repo)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list wiki pages err: %v", err))
	}
	return to.TextResult(wikiPages)
}

func CreateWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateWikiPageFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	title, _ := req.Params.Arguments["title"].(string)
	content, _ := req.Params.Arguments["content"].(string)
	message, _ := req.Params.Arguments["message"].(string)

	// Use default commit message if not provided
	if message == "" {
		message = fmt.Sprintf("Create wiki page '%s'", title)
	}

	opt := forgejo_sdk.CreateWikiPageOption{
		Title:   title,
		Content: content,
		Message: message,
	}

	wikiPage, _, err := forgejo.Client().CreateWikiPage(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create wiki page err: %v", err))
	}
	return to.TextResult(wikiPage)
}

func UpdateWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateWikiPageFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	pageName, _ := req.Params.Arguments["page_name"].(string)
	title, titleProvided := req.Params.Arguments["title"].(string)
	content, _ := req.Params.Arguments["content"].(string)
	message, _ := req.Params.Arguments["message"].(string)

	// If title is not provided, use the current page name
	if !titleProvided || title == "" {
		title = pageName
	}

	// Use default commit message if not provided
	if message == "" {
		message = fmt.Sprintf("Update wiki page '%s'", pageName)
	}

	opt := forgejo_sdk.EditWikiPageOption{
		Title:   title,
		Content: content,
		Message: message,
	}

	wikiPage, _, err := forgejo.Client().EditWikiPage(owner, repo, pageName, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update wiki page err: %v", err))
	}
	return to.TextResult(wikiPage)
}
