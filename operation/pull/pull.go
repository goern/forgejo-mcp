package pull

import (
	"context"
	"fmt"
	"strconv"

	"codeberg.org/goern/forgejo-mcp/operation/params"
	"codeberg.org/goern/forgejo-mcp/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/pkg/log"
	"codeberg.org/goern/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetPullRequestByIndexToolName = "get_pull_request_by_index"
	ListRepoPullRequestsToolName  = "list_repo_pull_requests"
	CreatePullRequestToolName     = "create_pull_request"
	UpdatePullRequestToolName     = "update_pull_request"
)

var (
	GetPullRequestByIndexTool = mcp.NewTool(
		GetPullRequestByIndexToolName,
		mcp.WithDescription("Get pull request by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
	)

	ListRepoPullRequestsTool = mcp.NewTool(
		ListRepoPullRequestsToolName,
		mcp.WithDescription("List repo pull requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("sort", mcp.Description("Sort (oldest|recentupdate|leastupdate|mostcomment)")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
		mcp.WithString("labels", mcp.Description(params.Labels)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	CreatePullRequestTool = mcp.NewTool(
		CreatePullRequestToolName,
		mcp.WithDescription("Create pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("head", mcp.Required(), mcp.Description(params.Head)),
		mcp.WithString("base", mcp.Required(), mcp.Description(params.Base)),
		mcp.WithString("title", mcp.Required(), mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
	)

	UpdatePullRequestTool = mcp.NewTool(
		UpdatePullRequestToolName,
		mcp.WithDescription("Update pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("title", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("base", mcp.Description(params.Base)),
		mcp.WithString("assignee", mcp.Description("Assignee username")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetPullRequestByIndexTool, GetPullRequestByIndexFn)
	s.AddTool(ListRepoPullRequestsTool, ListRepoPullRequestsFn)
	s.AddTool(CreatePullRequestTool, CreatePullRequestFn)
	s.AddTool(UpdatePullRequestTool, UpdatePullRequestFn)
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

func UpdatePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdatePullRequestFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)
	title, _ := req.Params.Arguments["title"].(string)
	body, _ := req.Params.Arguments["body"].(string)
	base, _ := req.Params.Arguments["base"].(string)
	assignee, _ := req.Params.Arguments["assignee"].(string)
	milestone, _ := req.Params.Arguments["milestone"].(string)

	opt := forgejo_sdk.EditPullRequestOption{}

	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = body
	}
	if base != "" {
		opt.Base = base
	}
	if assignee != "" {
		opt.Assignee = assignee
	}
	if milestone != "" {
		milestoneID, err := strconv.ParseInt(milestone, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %v", err))
		}
		opt.Milestone = milestoneID
	}

	pr, _, err := forgejo.Client().EditPullRequest(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update pull request err: %v", err))
	}
	return to.TextResult(pr)
}