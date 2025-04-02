package repo

import (
	"context"
	"fmt"

	"forgejo.com/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.com/forgejo/forgejo-mcp/pkg/log"
	"forgejo.com/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoCommitsToolName = "list_repo_commits"
)

var (
	ListRepoCommitsTool = mcp.NewTool(
		ListRepoCommitsToolName,
		mcp.WithDescription("List repository commits"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("path", mcp.Description("filepath of a file or directory")),
		mcp.WithString("sha", mcp.Description("SHA or branch name to start listing commits from")),
		mcp.WithNumber("page", mcp.Required(), mcp.Description("Page number"), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("pageSize", mcp.Required(), mcp.Description("Page size number"), mcp.DefaultNumber(100), mcp.Min(1)),
	)
)

func ListRepoCommitsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoCommitsFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	path, ok := req.Params.Arguments["path"].(string)
	pathStr := ""
	if ok && path != "" {
		pathStr = path
	}
	sha, ok := req.Params.Arguments["sha"].(string)
	shaStr := ""
	if ok && sha != "" {
		shaStr = sha
	}
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}
	opt := forgejo_sdk.ListCommitOptions{
		Path: pathStr,
		SHA:  shaStr,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}
	commits, _, err := forgejo.Client().ListRepoCommits(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo commits error: %v", err))
	}
	return to.TextResult(commits)
}
