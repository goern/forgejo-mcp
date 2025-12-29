package repo

import (
	"context"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

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
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("branch", mcp.Required(), mcp.Description(params.Branch)),
		mcp.WithString("old_branch", mcp.Required(), mcp.Description(params.OldBranch)),
	)

	DeleteBranchTool = mcp.NewTool(
		DeleteBranchToolName,
		mcp.WithDescription("Delete branch"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("branch", mcp.Required(), mcp.Description(params.Branch)),
	)

	ListBranchesTool = mcp.NewTool(
		ListBranchesToolName,
		mcp.WithDescription("List branches"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
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
	limit, ok := req.Params.Arguments["limit"].(float64)
	if !ok {
		limit = 100
	}

	opt := forgejo_sdk.ListRepoBranchesOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	branches, _, err := forgejo.Client().ListRepoBranches(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list branches err: %v", err))
	}
	return to.TextResult(branches)
}
