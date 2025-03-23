package pull

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/gitea"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"gitea.com/gitea/gitea-mcp/pkg/to"

	gitea_sdk "code.gitea.io/sdk/gitea"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetPullRequestByIndexToolName = "get_pull_request_by_index"
	ListRepoPullRequestsToolName  = "list_repo_pull_requests"
	CreatePullRequestToolName     = "create_pull_request"
)

var (
	GetPullRequestByIndexTool = mcp.NewTool(
		GetPullRequestByIndexToolName,
		mcp.WithDescription("get pull request by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository pull request index")),
	)

	ListRepoPullRequestsTool = mcp.NewTool(
		ListRepoPullRequestsToolName,
		mcp.WithDescription("List repository pull requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("state", mcp.Description("state")),
		mcp.WithString("sort", mcp.Description("sort")),
		mcp.WithNumber("milestone", mcp.Description("milestone")),
	)

	CreatePullRequestTool = mcp.NewTool(
		CreatePullRequestToolName,
		mcp.WithDescription("create pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("title", mcp.Required(), mcp.Description("pull request title")),
		mcp.WithString("body", mcp.Required(), mcp.Description("pull request body")),
		mcp.WithString("head", mcp.Required(), mcp.Description("pull request head")),
		mcp.WithString("base", mcp.Required(), mcp.Description("pull request base")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetPullRequestByIndexTool, GetPullRequestByIndexFn)
	s.AddTool(ListRepoPullRequestsTool, ListRepoPullRequestsFn)
	s.AddTool(CreatePullRequestTool, CreatePullRequestFn)
}

func GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullRequestByIndexFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("repo is required")
	}
	index, ok := req.Params.Arguments["index"].(float64)
	if !ok {
		return nil, fmt.Errorf("index is required")
	}
	pr, _, err := gitea.Client().GetPullRequest(owner, repo, int64(index))
	if err != nil {
		return nil, fmt.Errorf("get %v/%v/pr/%v err", owner, repo, int64(index))
	}

	return to.TextResult(pr)
}

func ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoPullRequests")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("repo is required")
	}
	state, _ := req.Params.Arguments["state"].(string)
	sort, _ := req.Params.Arguments["sort"].(string)
	milestone, _ := req.Params.Arguments["milestone"].(float64)
	opt := gitea_sdk.ListPullRequestsOptions{
		State:     gitea_sdk.StateType(state),
		Sort:      sort,
		Milestone: int64(milestone),
		ListOptions: gitea_sdk.ListOptions{
			Page:     1,
			PageSize: 1000,
		},
	}
	pullRequests, _, err := gitea.Client().ListRepoPullRequests("", "", opt)
	if err != nil {
		return nil, fmt.Errorf("list %v/%v/pull_requests err", owner, repo)
	}

	return to.TextResult(pullRequests)
}

func CreatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreatePullRequestFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return nil, fmt.Errorf("owner is required")
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return nil, fmt.Errorf("repo is required")
	}
	title, ok := req.Params.Arguments["title"].(string)
	if !ok {
		return nil, fmt.Errorf("title is required")
	}
	body, ok := req.Params.Arguments["body"].(string)
	if !ok {
		return nil, fmt.Errorf("body is required")
	}
	head, ok := req.Params.Arguments["head"].(string)
	if !ok {
		return nil, fmt.Errorf("head is required")
	}
	base, ok := req.Params.Arguments["base"].(string)
	if !ok {
		return nil, fmt.Errorf("base is required")
	}
	pr, _, err := gitea.Client().CreatePullRequest(owner, repo, gitea_sdk.CreatePullRequestOption{
		Title: title,
		Body:  body,
		Head:  head,
		Base:  base,
	})
	if err != nil {
		return nil, fmt.Errorf("create %v/%v/pull_request err", owner, repo)
	}

	return to.TextResult(pr)
}
