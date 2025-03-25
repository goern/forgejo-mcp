package issue

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
		mcp.WithDescription("List repository issues"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("state", mcp.Description("issue state"), mcp.DefaultString("all")),
		mcp.WithNumber("page", mcp.Description("page number"), mcp.DefaultNumber(1)),
		mcp.WithNumber("pageSize", mcp.Description("page size"), mcp.DefaultNumber(100)),
	)

	CreateIssueTool = mcp.NewTool(
		CreateIssueToolName,
		mcp.WithDescription("create issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithString("title", mcp.Required(), mcp.Description("issue title")),
		mcp.WithString("body", mcp.Required(), mcp.Description("issue body")),
	)
	CreateIssueCommentTool = mcp.NewTool(
		CreateIssueCommentToolName,
		mcp.WithDescription("create issue comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index")),
		mcp.WithString("body", mcp.Required(), mcp.Description("issue comment body")),
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
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	index, ok := req.Params.Arguments["index"].(float64)
	if !ok {
		return to.ErrorResult(fmt.Errorf("index is required"))
	}
	issue, _, err := gitea.Client().GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get %v/%v/issue/%v err: %v", owner, repo, int64(index), err))
	}

	return to.TextResult(issue)
}

func ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssuesFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	state, ok := req.Params.Arguments["state"].(string)
	if !ok {
		state = "all"
	}
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	pageSize, ok := req.Params.Arguments["pageSize"].(float64)
	if !ok {
		pageSize = 100
	}
	opt := gitea_sdk.ListIssueOption{
		State: gitea_sdk.StateType(state),
		ListOptions: gitea_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(pageSize),
		},
	}
	issues, _, err := gitea.Client().ListRepoIssues(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get %v/%v/issues err: %v", owner, repo, err))
	}
	return to.TextResult(issues)
}

func CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	title, ok := req.Params.Arguments["title"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("title is required"))
	}
	body, ok := req.Params.Arguments["body"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("body is required"))
	}
	issue, _, err := gitea.Client().CreateIssue(owner, repo, gitea_sdk.CreateIssueOption{
		Title: title,
		Body:  body,
	})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create %v/%v/issue err", owner, repo))
	}

	return to.TextResult(issue)
}

func CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueCommentFn")
	owner, ok := req.Params.Arguments["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.Params.Arguments["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	index, ok := req.Params.Arguments["index"].(float64)
	if !ok {
		return to.ErrorResult(fmt.Errorf("index is required"))
	}
	body, ok := req.Params.Arguments["body"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("body is required"))
	}
	opt := gitea_sdk.CreateIssueCommentOption{
		Body: body,
	}
	issueComment, _, err := gitea.Client().CreateIssueComment(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create %v/%v/issue/%v/comment err", owner, repo, int64(index)))
	}

	return to.TextResult(issueComment)
}
