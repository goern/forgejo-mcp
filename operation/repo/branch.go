package repo

import (
	"context"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"github.com/mark3labs/mcp-go/mcp"

	gitea_sdk "code.gitea.io/sdk/gitea"
)

const (
	CreateBranchToolName = "create_branch"
)

var (
	CreateBranchTool = mcp.NewTool(
		CreateBranchToolName,
		mcp.WithDescription("Create branch"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("branch", mcp.Required(), mcp.Description("Name of the branch to create"), mcp.DefaultString("")),
		mcp.WithString("old_branch", mcp.Description("Name of the old branch to create from"), mcp.DefaultString("")),
	)
)

func CreateBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	branch := req.Params.Arguments["branch"].(string)
	oldBranch := req.Params.Arguments["old_branch"].(string)

	_, _, err := gitea.Client().CreateBranch(owner, repo, gitea_sdk.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: oldBranch,
	})
	if err != nil {
		return mcp.NewToolResultError("Create Branch Error"), err
	}

	return mcp.NewToolResultText("Branch Created"), nil
}
