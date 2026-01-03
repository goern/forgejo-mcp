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
	ListRepoCommitsToolName = "list_repo_commits"
)

var (
	ListRepoCommitsTool = mcp.NewTool(
		ListRepoCommitsToolName,
		mcp.WithDescription("List repo commits"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("path", mcp.Description("File/dir path")),
		mcp.WithString("sha", mcp.Description("SHA/branch to start from")),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)
)

func ListRepoCommitsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoCommitsFn")
	owner, err := req.RequireString("owner")
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := req.RequireString("repo")
	if err != nil {
		return to.ErrorResult(err)
	}
	path := req.GetString("path", "")
	sha := req.GetString("sha", "")
	page, err := req.RequireFloat("page")
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := req.RequireFloat("limit")
	if err != nil {
		return to.ErrorResult(err)
	}
	opt := forgejo_sdk.ListCommitOptions{
		Path: path,
		SHA:  sha,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	commits, _, err := forgejo.Client().ListRepoCommits(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo commits error: %v", err))
	}
	return to.TextResult(commits)
}
