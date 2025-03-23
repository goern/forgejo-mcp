package repo

import (
	"context"
	"fmt"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoCommitsToolName = "list_repo_commits"
)

var (
	ListRepoCommitsTool = mcp.NewTool(
		ListRepoCommitsToolName,
		mcp.WithDescription("List repository commits"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("sha", mcp.Description("sha"), mcp.DefaultString("")),
		mcp.WithString("path", mcp.Description("path"), mcp.DefaultString("")),
	)
)

func ListRepoCommitsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoCommitsFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	opt := gitea_sdk.ListCommitOptions{
		ListOptions: gitea_sdk.ListOptions{
			Page:     1,
			PageSize: 1000,
		},
		SHA:  req.Params.Arguments["sha"].(string),
		Path: req.Params.Arguments["path"].(string),
	}
	commits, _, err := gitea.Client().ListRepoCommits(owner, repo, opt)
	if err != nil {
		return nil, fmt.Errorf("list repo commits err: %v", err)
	}
	return to.TextResult(commits)
}
