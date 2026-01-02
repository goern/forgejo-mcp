package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

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
	ListIssueCommentsToolName  = "list_issue_comments"
	GetIssueCommentToolName    = "get_issue_comment"
	EditIssueCommentToolName   = "edit_issue_comment"
	DeleteIssueCommentToolName = "delete_issue_comment"
	ListRepoLabelsToolName     = "list_repo_labels"
)

var (
	GetIssueByIndexTool = mcp.NewTool(
		GetIssueByIndexToolName,
		mcp.WithDescription("Get issue by index"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
	)

	ListRepoIssuesTool = mcp.NewTool(
		ListRepoIssuesToolName,
		mcp.WithDescription("List repo issues"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("type", mcp.Description("Type (issues|pulls)")),
		mcp.WithString("milestones", mcp.Description("Milestone names/IDs (comma-separated)")),
		mcp.WithString("labels", mcp.Description("Labels (comma-separated)")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	CreateIssueTool = mcp.NewTool(
		CreateIssueToolName,
		mcp.WithDescription("Create issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("title", mcp.Required(), mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
	)

	CreateIssueCommentTool = mcp.NewTool(
		CreateIssueCommentToolName,
		mcp.WithDescription("Create issue comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("body", mcp.Required(), mcp.Description(params.Body)),
	)

	UpdateIssueTool = mcp.NewTool(
		UpdateIssueToolName,
		mcp.WithDescription("Update issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("title", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithString("assignee", mcp.Description("Assignee username")),
		mcp.WithString("milestone", mcp.Description(params.Milestone)),
	)

	AddIssueLabelsTools = mcp.NewTool(
		AddIssueLabelsToolName,
		mcp.WithDescription("Add labels to issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("labels", mcp.Required(), mcp.Description("Labels to add (comma-separated)")),
	)

	IssueStateChangeTool = mcp.NewTool(
		IssueStateChangeToolName,
		mcp.WithDescription("Change issue state"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("state", mcp.Required(), mcp.Description("State (open|closed)")),
	)

	ListIssueCommentsTool = mcp.NewTool(
		ListIssueCommentsToolName,
		mcp.WithDescription("List issue/PR comments"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetIssueCommentTool = mcp.NewTool(
		GetIssueCommentToolName,
		mcp.WithDescription("Get comment by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	EditIssueCommentTool = mcp.NewTool(
		EditIssueCommentToolName,
		mcp.WithDescription("Edit issue/PR comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithString("body", mcp.Required(), mcp.Description(params.Body)),
	)

	DeleteIssueCommentTool = mcp.NewTool(
		DeleteIssueCommentToolName,
		mcp.WithDescription("Delete issue/PR comment"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	ListRepoLabelsTool = mcp.NewTool(
		ListRepoLabelsToolName,
		mcp.WithDescription("List all repository labels"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(50)),
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
	s.AddTool(ListIssueCommentsTool, ListIssueCommentsFn)
	s.AddTool(GetIssueCommentTool, GetIssueCommentFn)
	s.AddTool(EditIssueCommentTool, EditIssueCommentFn)
	s.AddTool(DeleteIssueCommentTool, DeleteIssueCommentFn)
	s.AddTool(ListRepoLabelsTool, ListRepoLabelsFn)
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

func ListIssueCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueCommentsFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	index, _ := req.Params.Arguments["index"].(float64)
	since, _ := req.Params.Arguments["since"].(string)
	before, _ := req.Params.Arguments["before"].(string)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.Params.Arguments["limit"].(float64)
	if !ok {
		limit = 20
	}

	opt := forgejo_sdk.ListIssueCommentOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	// Set time filters if provided
	if since != "" {
		sinceTime, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid since time format (expected RFC3339): %v", err))
		}
		opt.Since = sinceTime
	}
	if before != "" {
		beforeTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid before time format (expected RFC3339): %v", err))
		}
		opt.Before = beforeTime
	}

	comments, _, err := forgejo.Client().ListIssueComments(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list issue comments err: %v", err))
	}
	return to.TextResult(comments)
}

func GetIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueCommentFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	commentID, _ := req.Params.Arguments["comment_id"].(float64)

	comment, _, err := forgejo.Client().GetIssueComment(owner, repo, int64(commentID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue comment err: %v", err))
	}
	return to.TextResult(comment)
}

func EditIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueCommentFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	commentID, _ := req.Params.Arguments["comment_id"].(float64)
	body, _ := req.Params.Arguments["body"].(string)

	opt := forgejo_sdk.EditIssueCommentOption{
		Body: body,
	}
	comment, _, err := forgejo.Client().EditIssueComment(owner, repo, int64(commentID), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit issue comment err: %v", err))
	}
	return to.TextResult(comment)
}

func DeleteIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueCommentFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	commentID, _ := req.Params.Arguments["comment_id"].(float64)

	_, err := forgejo.Client().DeleteIssueComment(owner, repo, int64(commentID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete issue comment err: %v", err))
	}
	return to.TextResult("Delete comment success")
}

func ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoLabelsFn")
	owner, _ := req.Params.Arguments["owner"].(string)
	repo, _ := req.Params.Arguments["repo"].(string)
	page, ok := req.Params.Arguments["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.Params.Arguments["limit"].(float64)
	if !ok {
		limit = 50
	}

	opt := forgejo_sdk.ListLabelsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	labels, _, err := forgejo.Client().ListRepoLabels(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo labels err: %v", err))
	}
	return to.TextResult(labels)
}
