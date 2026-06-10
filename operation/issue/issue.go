package issue

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ScopedLabel wraps forgejo_sdk.Label with a scope marker so callers of
// list_repo_labels and list_org_labels can tell repo- and org-scoped
// labels apart in a merged response.
type ScopedLabel struct {
	*forgejo_sdk.Label
	Scope string `json:"scope"`
}

// fetchOrgLabels GETs /orgs/{org}/labels via the raw-HTTP helper and
// stamps each result with scope="org". A 404 is mapped to an empty slice
// by DoJSONList. 401/403 surface as forgejo.ErrUnauthorized.
func fetchOrgLabels(ctx context.Context, org string, page, limit int) ([]ScopedLabel, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	path := fmt.Sprintf("/orgs/%s/labels?page=%d&limit=%d", org, page, limit)
	var raw []*forgejo_sdk.Label
	if err := forgejo.DoJSONList(ctx, http.MethodGet, path, &raw); err != nil {
		return nil, err
	}
	out := make([]ScopedLabel, 0, len(raw))
	for _, l := range raw {
		out = append(out, ScopedLabel{Label: l, Scope: "org"})
	}
	return out, nil
}

const (
	GetIssueByIndexToolName    = "get_issue_by_index"
	ListRepoIssuesToolName     = "list_repo_issues"
	CreateIssueToolName        = "create_issue"
	CreateIssueCommentToolName = "create_issue_comment"
	UpdateIssueToolName        = "update_issue"
	AddIssueLabelsToolName     = "add_issue_labels"
	RemoveIssueLabelsToolName  = "remove_issue_labels"
	IssueStateChangeToolName   = "issue_state_change"
	ListIssueCommentsToolName  = "list_issue_comments"
	GetIssueCommentToolName    = "get_issue_comment"
	EditIssueCommentToolName   = "edit_issue_comment"
	DeleteIssueCommentToolName = "delete_issue_comment"
	ListRepoMilestonesToolName = "list_repo_milestones"
	ListRepoLabelsToolName     = "list_repo_labels"
	ListOrgLabelsToolName      = "list_org_labels"
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
		mcp.WithString("assignee", mcp.Description("Assignee username (convenience for a single user; equivalent to a one-element 'assignees')")),
		mcp.WithString("assignees", mcp.Description("Assignee usernames (comma-separated). Overrides 'assignee' if both are set. Pass an empty string to clear all assignees.")),
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

	RemoveIssueLabelsTools = mcp.NewTool(
		RemoveIssueLabelsToolName,
		mcp.WithDescription("Remove labels from issue"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.IssueIndex)),
		mcp.WithString("labels", mcp.Required(), mcp.Description("Labels to remove (comma-separated label IDs)")),
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

	ListRepoMilestonesTool = mcp.NewTool(
		ListRepoMilestonesToolName,
		mcp.WithDescription("List repository milestones"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
		mcp.WithString("state", mcp.Description("Milestone state (open|closed|all)"), mcp.DefaultString("open")),
	)

	ListRepoLabelsTool = mcp.NewTool(
		ListRepoLabelsToolName,
		mcp.WithDescription("List repository labels. When the owner is an organization and include_org_labels is true (default), org-level labels are merged into the response. Each label carries a scope field of \"repo\" or \"org\"."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
		mcp.WithBoolean("include_org_labels", mcp.Description("Merge org-level labels into the response when the owner is an organization. Default true."), mcp.DefaultBool(true)),
	)

	ListOrgLabelsTool = mcp.NewTool(
		ListOrgLabelsToolName,
		mcp.WithDescription("List organization-level labels. Each label carries a scope field of \"org\"."),
		mcp.WithString("org", mcp.Required(), mcp.Description("Organization name")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100)),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetIssueByIndexTool, GetIssueByIndexFn)
	s.AddTool(ListRepoIssuesTool, ListRepoIssuesFn)
	s.AddTool(CreateIssueTool, CreateIssueFn)
	s.AddTool(CreateIssueCommentTool, CreateIssueCommentFn)
	s.AddTool(UpdateIssueTool, UpdateIssueFn)
	s.AddTool(AddIssueLabelsTools, AddIssueLabelsFn)
	s.AddTool(RemoveIssueLabelsTools, RemoveIssueLabelsFn)
	s.AddTool(IssueStateChangeTool, IssueStateChangeFn)
	s.AddTool(ListIssueCommentsTool, ListIssueCommentsFn)
	s.AddTool(GetIssueCommentTool, GetIssueCommentFn)
	s.AddTool(EditIssueCommentTool, EditIssueCommentFn)
	s.AddTool(DeleteIssueCommentTool, DeleteIssueCommentFn)
	s.AddTool(ListRepoMilestonesTool, ListRepoMilestonesFn)
	s.AddTool(ListRepoLabelsTool, ListRepoLabelsFn)
	s.AddTool(ListOrgLabelsTool, ListOrgLabelsFn)
	RegisterLabelTool(s)
}

func GetIssueByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueByIndexFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue err: %v", err))
	}
	return to.TextResult(issue)
}

func ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoIssuesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	state, ok := req.GetArguments()["state"].(string)
	if !ok {
		state = "open"
	}
	issueType, _ := req.GetArguments()["type"].(string)
	milestones, _ := req.GetArguments()["milestones"].(string)
	labels, _ := req.GetArguments()["labels"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
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

	// Set issue type if provided
	if issueType != "" {
		opt.Type = forgejo_sdk.IssueType(issueType)
	}

	// Set milestones if provided
	if milestones != "" {
		opt.Milestones = strings.Split(milestones, ",")
	}

	// Set labels if provided
	if labels != "" {
		opt.Labels = strings.Split(labels, ",")
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issues, _, err := client.ListRepoIssues(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issues list err: %v", err))
	}
	return to.TextResult(issues)
}

func CreateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)

	opt := forgejo_sdk.CreateIssueOption{
		Title: title,
		Body:  body,
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, _, err := client.CreateIssue(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue err: %v", err))
	}
	return to.TextResult(issue)
}

func CreateIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	body, _ := req.GetArguments()["body"].(string)

	opt := forgejo_sdk.CreateIssueCommentOption{
		Body: body,
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comment, _, err := client.CreateIssueComment(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue comment err: %v", err))
	}
	return to.TextResult(comment)
}

func UpdateIssueFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateIssueFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)
	assignee, _ := req.GetArguments()["assignee"].(string)
	assigneesRaw, assigneesProvided := req.GetArguments()["assignees"].(string)
	milestone, _ := req.GetArguments()["milestone"].(string)

	opt := forgejo_sdk.EditIssueOption{}

	// Only set fields that were provided
	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = &body
	}
	// Assignees: 'assignees' (CSV) wins if provided; otherwise fall back to singular 'assignee'.
	// An explicitly provided empty 'assignees' clears all assignees (sent as []).
	switch {
	case assigneesProvided:
		opt.Assignees = splitCSV(assigneesRaw)
	case assignee != "":
		opt.Assignees = []string{assignee}
	}
	if milestone != "" {
		milestoneID, err := strconv.ParseInt(milestone, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %v", err))
		}
		opt.Milestone = &milestoneID
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, _, err := client.EditIssue(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update issue err: %v", err))
	}
	return to.TextResult(issue)
}

func AddIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueLabelsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	labels, _ := req.GetArguments()["labels"].(string)

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

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	_, _, err = client.AddIssueLabels(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("add issue labels err: %v", err))
	}

	// Fetch the updated issue to return it with the new labels
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %v", err))
	}
	return to.TextResult(issue)
}

func RemoveIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveIssueLabelsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	labels, _ := req.GetArguments()["labels"].(string)

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	for _, labelStr := range strings.Split(labels, ",") {
		labelStr = strings.TrimSpace(labelStr)
		labelID, err := strconv.ParseInt(labelStr, 10, 64)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid label ID '%s': %v - labels must be numeric IDs", labelStr, err))
		}
		_, err = client.DeleteIssueLabel(owner, repo, int64(index), labelID)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("remove issue label err: %v", err))
		}
	}

	// Fetch the updated issue to return it with the updated labels
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %v", err))
	}
	return to.TextResult(issue)
}

func IssueStateChangeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called IssueStateChangeFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	state, _ := req.GetArguments()["state"].(string)

	if state != "open" && state != "closed" {
		return to.ErrorResult(fmt.Errorf("invalid state: %s, must be 'open' or 'closed'", state))
	}

	// Convert string to StateType and create pointer
	stateType := forgejo_sdk.StateType(state)

	opt := forgejo_sdk.EditIssueOption{
		State: &stateType,
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, _, err := client.EditIssue(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("change issue state err: %v", err))
	}
	return to.TextResult(issue)
}

func ListIssueCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueCommentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	since, _ := req.GetArguments()["since"].(string)
	before, _ := req.GetArguments()["before"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
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

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comments, _, err := client.ListIssueComments(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list issue comments err: %v", err))
	}
	return to.TextResult(comments)
}

func GetIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	commentID, _ := to.Float64(req.GetArguments()["comment_id"])

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comment, _, err := client.GetIssueComment(owner, repo, int64(commentID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue comment err: %v", err))
	}
	return to.TextResult(comment)
}

func EditIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	commentID, _ := to.Float64(req.GetArguments()["comment_id"])
	body, _ := req.GetArguments()["body"].(string)

	opt := forgejo_sdk.EditIssueCommentOption{
		Body: body,
	}
	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comment, _, err := client.EditIssueComment(owner, repo, int64(commentID), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit issue comment err: %v", err))
	}
	return to.TextResult(comment)
}

func DeleteIssueCommentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueCommentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	commentID, _ := to.Float64(req.GetArguments()["comment_id"])

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	_, err = client.DeleteIssueComment(owner, repo, int64(commentID))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete issue comment err: %v", err))
	}
	return to.TextResult("Delete comment success")
}
func ListRepoMilestonesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoMilestonesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	state, ok := req.GetArguments()["state"].(string)
	if !ok || state == "" {
		state = "open"
	}
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 100
	}

	opt := forgejo_sdk.ListMilestoneOption{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
		State: forgejo_sdk.StateType(state),
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	milestones, _, err := client.ListRepoMilestones(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo milestones err: %v", err))
	}
	return to.TextResult(milestones)
}

func ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoLabelsFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 100
	}
	includeOrg := true
	if v, ok := args["include_org_labels"].(bool); ok {
		includeOrg = v
	}

	opt := forgejo_sdk.ListLabelsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	repoLabels, _, err := client.ListRepoLabels(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo labels err: %v", err))
	}
	merged := make([]ScopedLabel, 0, len(repoLabels))
	for _, l := range repoLabels {
		merged = append(merged, ScopedLabel{Label: l, Scope: "repo"})
	}

	if includeOrg {
		orgLabels, oerr := fetchOrgLabels(ctx, owner, int(page), int(limit))
		if oerr != nil {
			return to.ErrorResult(fmt.Errorf("list org labels err: %v", oerr))
		}
		merged = append(merged, orgLabels...)
	}

	return to.TextResult(merged)
}

func ListOrgLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListOrgLabelsFn")
	args := req.GetArguments()
	org, _ := args["org"].(string)
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 100
	}

	labels, err := fetchOrgLabels(ctx, org, int(page), int(limit))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org labels err: %v", err))
	}
	return to.TextResult(labels)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
