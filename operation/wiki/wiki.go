//go:build wiki

package wiki

import (
	"context"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

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
		mcp.WithDescription("List wiki pages"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	CreateWikiPageTool = mcp.NewTool(
		CreateWikiPageToolName,
		mcp.WithDescription("Create wiki page"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("title", mcp.Required(), mcp.Description(params.WikiTitle)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
		mcp.WithString("message", mcp.Description(params.Message)),
	)

	UpdateWikiPageTool = mcp.NewTool(
		UpdateWikiPageToolName,
		mcp.WithDescription("Update wiki page"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
		mcp.WithString("title", mcp.Description(params.WikiTitle)),
		mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
		mcp.WithString("message", mcp.Description(params.Message)),
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
