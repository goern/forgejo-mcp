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
	GetMyReposTool = mcp.NewTool(
		ListMyReposToolName,
		mcp.WithDescription("List My Repositories"),
	)
)

func MyUserReposFn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	opts := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{
			Page:     1,
			PageSize: 100,
		},
	}
	repos, _, err := giteaPkg.Client().ListMyRepos(opts)
	if err != nil {
		return mcp.NewToolResultError("Get My User Info Error"), err
	}

	result, err := json.Marshal(repos)
	if err != nil {
		return mcp.NewToolResultError("Get My User Info Error"), err
	}
	return mcp.NewToolResultText(string(result)), nil
}
