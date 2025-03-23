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
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository pull request index"), mcp.DefaultNumber(0)),
	)

	ListRepoPullRequestsTool = mcp.NewTool(
		ListRepoPullRequestsToolName,
		mcp.WithDescription("List repository pull requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("state", mcp.Description("state"), mcp.DefaultString("")),
		mcp.WithString("sort", mcp.Description("sort"), mcp.DefaultString("")),
		mcp.WithNumber("milestone", mcp.Description("milestone"), mcp.DefaultNumber(0)),
	)

	CreatePullRequestTool = mcp.NewTool(
		CreatePullRequestToolName,
		mcp.WithDescription("create pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("title", mcp.Required(), mcp.Description("pull request title"), mcp.DefaultString("")),
		mcp.WithString("body", mcp.Required(), mcp.Description("pull request body"), mcp.DefaultString("")),
		mcp.WithString("head", mcp.Required(), mcp.Description("pull request head"), mcp.DefaultString("")),
		mcp.WithString("base", mcp.Required(), mcp.Description("pull request base"), mcp.DefaultString("")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetPullRequestByIndexTool, GetPullRequestByIndexFn)
	s.AddTool(ListRepoPullRequestsTool, ListRepoPullRequestsFn)
	s.AddTool(CreatePullRequestTool, CreatePullRequestFn)
}

func GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullRequestByIndexFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	index := req.Params.Arguments["index"].(float64)
	pr, _, err := gitea.Client().GetPullRequest(owner, repo, int64(index))
	if err != nil {
		return nil, fmt.Errorf("get %v/%v/pr/%v err", owner, repo, int64(index))
	}

	return to.TextResult(pr)
}

func ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoPullRequests")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	opt := gitea_sdk.ListPullRequestsOptions{
		State:     gitea_sdk.StateType(req.Params.Arguments["state"].(string)),
		Sort:      req.Params.Arguments["sort"].(string),
		Milestone: req.Params.Arguments["milestone"].(int64),
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
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	title := req.Params.Arguments["title"].(string)
	body := req.Params.Arguments["body"].(string)
	head := req.Params.Arguments["head"].(string)
	base := req.Params.Arguments["base"].(string)
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
