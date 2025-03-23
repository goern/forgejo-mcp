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
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index"), mcp.DefaultNumber(0)),
	)

	ListRepoIssuesTool = mcp.NewTool(
		ListRepoIssuesToolName,
		mcp.WithDescription("List repository issues"),
	)

	CreateIssueTool = mcp.NewTool(
		CreateIssueToolName,
		mcp.WithDescription("create issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithString("title", mcp.Required(), mcp.Description("issue title"), mcp.DefaultString("")),
		mcp.WithString("body", mcp.Required(), mcp.Description("issue body"), mcp.DefaultString("")),
	)
	CreateIssueCommentTool = mcp.NewTool(
		CreateIssueCommentToolName,
		mcp.WithDescription("create issue comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index"), mcp.DefaultNumber(0)),
		mcp.WithString("body", mcp.Required(), mcp.Description("issue comment body"), mcp.DefaultString("")),
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
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	index := req.Params.Arguments["index"].(float64)
	issue, _, err := gitea.Client().GetIssue(owner, repo, int64(index))
	if err != nil {
		return nil, fmt.Errorf("get %v/%v/issue/%v err", owner, repo, int64(index))
	}

	return to.TextResult(issue)
}

func ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssuesFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	opt := gitea_sdk.ListIssueOption{}
	issues, _, err := gitea.Client().ListRepoIssues(owner, repo, opt)
	if err != nil {
		return nil, fmt.Errorf("get %v/%v/issues err", owner, repo)
	}
	return to.TextResult(issues)
}

func CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	title := req.Params.Arguments["title"].(string)
	body := req.Params.Arguments["body"].(string)
	issue, _, err := gitea.Client().CreateIssue(owner, repo, gitea_sdk.CreateIssueOption{
		Title: title,
		Body:  body,
	})
	if err != nil {
		return nil, fmt.Errorf("create %v/%v/issue err", owner, repo)
	}

	return to.TextResult(issue)
}

func CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueCommentFn")
	owner := req.Params.Arguments["owner"].(string)
	repo := req.Params.Arguments["repo"].(string)
	index := req.Params.Arguments["index"].(float64)
	body := req.Params.Arguments["body"].(string)
	issueComment, _, err := gitea.Client().CreateIssueComment(owner, repo, int64(index), gitea_sdk.CreateIssueCommentOption{
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("create %v/%v/issue/%v/comment err", owner, repo, int64(index))
	}

	return to.TextResult(issueComment)
}
