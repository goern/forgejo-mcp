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
	GetIssueByIndexToolName       = "get_issue_by_index"
	GetPullRequestByIndexToolName = "get_pull_request_by_index"
	CreateIssueToolName           = "create_issue"
	CreateIssueCommentToolName    = "create_issue_comment"
	CreatePullRequestToolName     = "create_pull_request"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		mcp.WithDescription("get issue by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index"), mcp.DefaultNumber(0)),
	)
	GetPullRequestByIndexTool = mcp.NewTool(
		GetPullRequestByIndexToolName,
		mcp.WithDescription("get pull request by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner"), mcp.DefaultString("")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name"), mcp.DefaultString("")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository pull request index"), mcp.DefaultNumber(0)),
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
	s.AddTool(GetIssueByIndexTool, GetIssueByIndexFn)
	s.AddTool(GetPullRequestByIndexTool, GetPullRequestByIndexFn)
	s.AddTool(CreateIssueTool, CreateIssueFn)
	s.AddTool(CreateIssueCommentTool, CreateIssueCommentFn)
	s.AddTool(CreatePullRequestTool, CreatePullRequestFn)
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
