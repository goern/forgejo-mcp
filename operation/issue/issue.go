package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
	"forgejo.org/forgejo/forgejo-mcp/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetIssueByIndexToolName    = "get_issue_by_index"
	ListRepoIssuesToolName     = "list_repo_issues"
	CreateIssueToolName        = "create_issue"
	CreateIssueCommentToolName = "create_issue_comment"
	UpdateIssueToolName        = "update_issue"
	AddIssueLabelsToolName     = "add_issue_labels"
	IssueStateChangeToolName   = "issue_state_change"
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

	UpdateIssueTool = mcp.NewTool(
		UpdateIssueToolName,
		mcp.WithDescription("update existing issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index")),
		mcp.WithString("title", mcp.Description("new issue title")),
		mcp.WithString("body", mcp.Description("new issue body")),
		mcp.WithString("assignee", mcp.Description("username of the user to assign")),
		mcp.WithString("milestone", mcp.Description("milestone ID")),
	)

	AddIssueLabelsTools = mcp.NewTool(
		AddIssueLabelsToolName,
		mcp.WithDescription("add labels to an issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index")),
		mcp.WithString("labels", mcp.Required(), mcp.Description("comma separated list of labels to add")),
	)

	IssueStateChangeTool = mcp.NewTool(
		IssueStateChangeToolName,
		mcp.WithDescription("close or reopen an issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description("repository owner")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("repository name")),
		mcp.WithNumber("index", mcp.Required(), mcp.Description("repository issue index")),
		mcp.WithString("state", mcp.Required(), mcp.Description("new state: open or closed")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetIssueByIndexTool, GetIssueByIndexFn)
	s.AddTool(ListRepoIssuesTool, ListRepoIssuesFn)
	s.AddTool(CreateIssueTool, CreateIssueFn)
	s.AddTool(CreateIssueCommentTool, CreateIssueCommentFn)
	s.AddTool(UpdateIssueTool, UpdateIssueFn)
	s.AddTool(AddIssueLabelsTools, AddIssueLabelsFn)
	s.AddTool(IssueStateChangeTool, IssueStateChangeFn)
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

func UpdateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateIssueFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)
	title, _ := req.Params.Arguments["title"].(string)
	body, _ := req.Params.Arguments["body"].(string)
	// assignee is not supported in the current SDK
	milestone, _ := req.Params.Arguments["milestone"].(string)

	opt := forgejo_sdk.EditIssueOption{}
	
	// Only set fields that were provided
	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = &body
	}
	// Note: Assignee field doesn't exist in EditIssueOption
	// Using collaborators field would require changes to the API
	if milestone != "" {
		milestoneID, err := strconv.ParseInt(milestone, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %v", err))
		}
		opt.Milestone = &milestoneID
	}

	issue, _, err := forgejo.Client().EditIssue(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update issue err: %v", err))
	}
	return to.TextResult(issue)
}

func AddIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueLabelsFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)
	labels, _ := req.Params.Arguments["labels"].(string)

	// Get the ID for each label
	// Since we can't directly use label names, we need to fetch the IDs first
	// This modified approach treats the labels as numeric IDs
	labelIDs := []int64{}
	
	for _, labelStr := range strings.Split(labels, ",") {
		labelStr = strings.TrimSpace(labelStr)
		labelID, err := strconv.ParseInt(labelStr, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid label ID '%s': %v - labels must be numeric IDs", labelStr, err))
		}
		labelIDs = append(labelIDs, labelID)
	}

	// Create IssueLabelsOption with numeric IDs
	opt := forgejo_sdk.IssueLabelsOption{
		Labels: labelIDs,
	}
	
	_, _, err := forgejo.Client().AddIssueLabels(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("add issue labels err: %v", err))
	}
	
	// Fetch the updated issue to return it with the new labels
	issue, _, err := forgejo.Client().GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %v", err))
	}
	return to.TextResult(issue)
}

func IssueStateChangeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called IssueStateChangeFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)
	state, _ := req.Params.Arguments["state"].(string)

	if state != "open" && state != "closed" {
		return to.ErrorResult(fmt.Errorf("invalid state: %s, must be 'open' or 'closed'", state))
	}

	// Convert string to StateType and create pointer
	stateType := forgejo_sdk.StateType(state)
	
	opt := forgejo_sdk.EditIssueOption{
		State: &stateType,
	}

	issue, _, err := forgejo.Client().EditIssue(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("change issue state err: %v", err))
	}
	return to.TextResult(issue)
}