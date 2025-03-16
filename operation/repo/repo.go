package repo

import (
	"context"
	"encoding/json"

	"code.gitea.io/sdk/gitea"
	giteaPkg "gitea.com/gitea/gitea-mcp/pkg/gitea"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListMyReposToolName = "list_my_repos"
)

var (
	ListMyReposTool = mcp.NewTool(
		ListMyReposToolName,
		mcp.WithDescription("List my repositories"),
		mcp.WithNumber(
			"page",
			mcp.Description("Page number"),
			mcp.DefaultNumber(1),
			mcp.Min(1),
		),
		mcp.WithNumber(
			"pageSize",
			mcp.Description("Page size number"),
			mcp.DefaultNumber(10),
			mcp.Min(1),
		),
	)
)

func ListMyReposFn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	page, ok := request.Params.Arguments["page"].(float64)
	if !ok {
		return mcp.NewToolResultError("get page number error"), nil
	}
	size, ok := request.Params.Arguments["pageSize"].(float64)
	if !ok {
		return mcp.NewToolResultError("get page size number error"), nil
	}
	opts := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{
			Page:     int(page),
			PageSize: int(size),
		},
	}
	repos, _, err := giteaPkg.Client().ListMyRepos(opts)
	if err != nil {
		return mcp.NewToolResultError("List my repositories error"), err
	}

	result, err := json.Marshal(repos)
	if err != nil {
		return mcp.NewToolResultError("marshal repository list error"), err
	}
	return mcp.NewToolResultText(string(result)), nil
}
