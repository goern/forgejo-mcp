package repo

import (
	"context"
	"fmt"

	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
	"forgejo.org/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
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
		mcp.WithNumber("page", mcp.Required(), mcp.Description("Page number"), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("pageSize", mcp.Required(), mcp.Description("Page size number"), mcp.DefaultNumber(100), mcp.Min(1)),
	)
)

func CreateBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateBranchFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	branch, ok := req.Params.Arguments["branch"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("branch is required"))
	}
	oldBranch, _ := req.Params.Arguments["old_branch"].(string)

	_, _, err := forgejo.Client().CreateBranch(owner, repo, forgejo_sdk.CreateBranchOption{
		BranchName:    branch,
		OldBranchName: oldBranch,
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create branch error: %v", err))
	}

	return mcp.NewToolResultText("Branch Created"), nil
}

func DeleteBranchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteBranchFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	branch, _ := req.Params.Arguments["branch"].(string)

	success, _, err := forgejo.Client().DeleteRepoBranch(owner, repo, branch)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete branch err: %v", err))
	}
	if !success {
		return to.ErrorResult(fmt.Errorf("failed to delete branch (status not 204)"))
	}
	return to.TextResult("Delete Branch Success")
}

func ListBranchesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListBranchesFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}

	opt := forgejo_sdk.ListRepoBranchesOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}

	branches, _, err := forgejo.Client().ListRepoBranches(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list branches err: %v", err))
	}
	return to.TextResult(branches)
}
