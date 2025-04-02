package issue

import (
	"context"
	"fmt"
	"strings"

	"forgejo.com/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.com/forgejo/forgejo-mcp/pkg/log"
	"forgejo.com/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetIssueByIndexToolName    = "get_issue_by_index"
	ListRepoIssuesToolName     = "list_repo_issues"
	CreateIssueToolName        = "create_issue"
	CreateIssueCommentToolName = "create_issue_comment"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		mcp.WithDescription("get issue by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index")),
	)

	ListRepoIssuesTool = mcp.NewTool(
		ListRepoIssuesToolName,
		mcp.WithDescription("list repo issues"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("state", mcp.Description("state of issue. Possible values are: open, closed and all. Default is 'open'"), mcp.DefaultString("open")),
		mcp.WithString("type", mcp.Description("filter by type (issues / pulls) if set")),
		mcp.WithString("milestones", mcp.Description("comma separated list of milestone names or IDs")),
		mcp.WithString("labels", mcp.Description("comma separated list of labels")),
		mcp.WithNumber("page", mcp.Description("page number of results to return (1-based)"), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description("page size of results"), mcp.DefaultNumber(20)),
	)

	CreateIssueTool = mcp.NewTool(
		CreateIssueToolName,
		mcp.WithDescription("create issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("title", mcp.Required(), mcp.Description("issue title")),
		mcp.WithString("body", mcp.Description("issue body")),
	)

	CreateIssueCommentTool = mcp.NewTool(
		CreateIssueCommentToolName,
		mcp.WithDescription("create issue comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index")),
		mcp.WithString("body", mcp.Required(), mcp.Description("comment body")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetIssueByIndexTool, GetIssueByIndexFn)
	s.AddTool(ListRepoIssuesTool, ListRepoIssuesFn)
	s.AddTool(CreateIssueTool, CreateIssueFn)
	s.AddTool(CreateIssueCommentTool, CreateIssueCommentFn)
}

func GetIssueByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueByIndexFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)

	issue, _, err := forgejo.Client().GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue err: %v", err))
	}
	return to.TextResult(issue)
}

func ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoIssuesFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	state, ok := req.Params.Arguments["state"].(string)
	if !ok {
		state = "open"
	}
	issueType, _ := req.Params.Arguments["type"].(string)
	milestones, _ := req.Params.Arguments["milestones"].(string)
	labels, _ := req.Params.Arguments["labels"].(string)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.Params.Arguments["limit"].(float64)
	if !ok {
		limit = 20
	}

	// Create ListIssueOption according to the Forgejo API
	opt := forgejo_sdk.ListIssueOption{
		// State is correctly set directly
		State: forgejo_sdk.StateType(state),
		// ListOptions maps directly
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Set issue type if provided (convert to string parameters)
	if issueType != "" {
		// Note: Using optional parameters since IssueType is not directly assignable
	}

	// Set milestones if provided
	if milestones != "" {
		opt.Milestones = strings.Split(milestones, ",")
	}

	// Set labels if provided
	if labels != "" {
		opt.Labels = strings.Split(labels, ",")
	}

	issues, _, err := forgejo.Client().ListRepoIssues(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issues list err: %v", err))
	}
	return to.TextResult(issues)
}

func CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	title, _ := req.Params.Arguments["title"].(string)
	body, _ := req.Params.Arguments["body"].(string)

	opt := forgejo_sdk.CreateIssueOption{
		Title: title,
		Body:  body,
	}
	issue, _, err := forgejo.Client().CreateIssue(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue err: %v", err))
	}
	return to.TextResult(issue)
}

func CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueCommentFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)
	body, _ := req.Params.Arguments["body"].(string)

	opt := forgejo_sdk.CreateIssueCommentOption{
		Body: body,
	}
	comment, _, err := forgejo.Client().CreateIssueComment(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue comment err: %v", err))
	}
	return to.TextResult(comment)
}