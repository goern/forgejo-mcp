package repo

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CreateBranchToolName = "create_branch"
	DeleteBranchToolName = "delete_branch"
	ListBranchesToolName = "list_branches"
)

var (
	CreateBranchTool = mcp.NewTool(
		CreateBranchToolName,
		mcp.WithDescription("Create branch"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("branch", mcp.Required(), mcp.Description("Name of the branch to create")),
		mcp.WithString("old_branch", mcp.Required(), mcp.Description("Name of the old branch to create from")),
	)

	DeleteBranchTool = mcp.NewTool(
		DeleteBranchToolName,
		mcp.WithDescription("Delete branch"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("branch", mcp.Required(), mcp.Description("Name of the branch to delete")),
	)

	ListBranchesTool = mcp.NewTool(
		ListBranchesToolName,
		mcp.WithDescription("List branches"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
	)
)

func CreateBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateBranchFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("repo is required")
	}
	branch, ok := req.Params.Arguments["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("branch is required")
	}
	oldBranch, _ := req.Params.Arguments["old_branch"].(string)

	_, _, err := gitea.Client().CreateBranch(owner, repo, gitea_sdk.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: oldBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("create branch error: %v", err)
	}

	return mcp.NewToolResultText("Branch Created"), nil
}

func DeleteBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteBranchFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("repo is required")
	}
	branch, ok := req.Params.Arguments["branch"].(string)
	if !ok {
		return nil, fmt.Errorf("branch is required")
	}
	_, _, err := gitea.Client().DeleteRepoBranch(owner, repo, branch)
	if err != nil {
		return nil, fmt.Errorf("delete branch error: %v", err)
	}

	return to.TextResult("Branch Deleted")
}

func ListBranchesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListBranchesFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("repo is required")
	}
	opt := gitea_sdk.ListRepoBranchesOptions{
		ListOptions: gitea_sdk.ListOptions{
			Page:     1,
			PageSize: 100,
		},
	}
	branches, _, err := gitea.Client().ListRepoBranches(owner, repo, opt)
	if err != nil {
		return nil, fmt.Errorf("list branches error: %v", err)
	}

	return to.TextResult(branches)
}
