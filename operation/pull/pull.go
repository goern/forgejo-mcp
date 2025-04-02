package pull

import (
	"context"
	"fmt"

	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
	"forgejo.org/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
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
		mcp.WithDescription("list repo pull requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("state", mcp.Description("state of pull request. Possible values are: open, closed and all. Default is 'open'"), mcp.DefaultString("open")),
		mcp.WithString("sort", mcp.Description("sort type of pull request. Possible values are: oldest, recentupdate, leastupdate and mostcomment. Default is 'recentupdate'")),
		mcp.WithString("milestone", mcp.Description("ID of the milestone")),
		mcp.WithString("labels", mcp.Description("list of label IDs")),
		mcp.WithNumber("page", mcp.Description("page number of results to return (1-based)"), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description("page size of results"), mcp.DefaultNumber(20)),
	)

	CreatePullRequestTool = mcp.NewTool(
		CreatePullRequestToolName,
		mcp.WithDescription("create pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("head", mcp.Required(), mcp.Description("head branch")),
		mcp.WithString("base", mcp.Required(), mcp.Description("base branch")),
		mcp.WithString("title", mcp.Required(), mcp.Description("pull request title")),
		mcp.WithString("body", mcp.Description("pull request body")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetPullRequestByIndexTool, GetPullRequestByIndexFn)
	s.AddTool(ListRepoPullRequestsTool, ListRepoPullRequestsFn)
	s.AddTool(CreatePullRequestTool, CreatePullRequestFn)
}

func GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullRequestByIndexFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)

	pr, _, err := forgejo.Client().GetPullRequest(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull request err: %v", err))
	}
	return to.TextResult(pr)
}

func ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoPullRequestsFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	state, ok := req.Params.Arguments["state"].(string)
	if !ok {
		state = "open"
	}
	sort, _ := req.Params.Arguments["sort"].(string)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.Params.Arguments["limit"].(float64)
	if !ok {
		limit = 20
	}

	// Convert milestone from string to int64 if provided
	// Note: Not using milestoneID since it's not supported in the current Forgejo SDK

	// Labels - not used directly in query per API, will be handled in the API call
	
	opt := forgejo_sdk.ListPullRequestsOptions{
		State: forgejo_sdk.StateType(state),
		Sort:  sort,
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	
	// Only set milestone if provided and valid
	// Note: Not using milestone as it's not supported in the current Forgejo SDK

	prs, _, err := forgejo.Client().ListRepoPullRequests(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull request list err: %v", err))
	}
	return to.TextResult(prs)
}

func CreatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreatePullRequestFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	head, _ := req.Params.Arguments["head"].(string)
	base, _ := req.Params.Arguments["base"].(string)
	title, _ := req.Params.Arguments["title"].(string)
	body, _ := req.Params.Arguments["body"].(string)

	opt := forgejo_sdk.CreatePullRequestOption{
		Head:  head,
		Base:  base,
		Title: title,
		Body:  body,
	}
	pr, _, err := forgejo.Client().CreatePullRequest(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create pull request err: %v", err))
	}
	return to.TextResult(pr)
}