package issue

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// issueSortValues are the Forgejo API's valid `sort` values for
// GET /repos/{owner}/{repo}/issues (see routers/api/v1/repo/issue.go's
// ListIssues swagger comment). "nearduedate"/"farduedate" are what daikon#93
// needs for EDF selection; the SDK (codeberg.org/mvdkleijn/forgejo-sdk v3.0.0)
// has no Sort field on ListIssueOption, so this list is duplicated here
// rather than sourced from the SDK.
var issueSortValues = []string{
	"relevance", "latest", "oldest", "recentupdate", "leastupdate",
	"mostcomment", "leastcomment", "nearduedate", "farduedate",
}

func isValidIssueSort(s string) bool {
	for _, v := range issueSortValues {
		if s == v {
			return true
		}
	}
	return false
}

// ScopedLabel wraps forgejo_sdk.Label with a scope marker so callers of
// list_repo_labels and list_org_labels can tell repo- and org-scoped
// labels apart in a merged response.
type ScopedLabel struct {
	*forgejo_sdk.Label
	Scope string `json:"scope"`
}

// fetchOrgLabels GETs /orgs/{org}/labels via the raw-HTTP helper and
// stamps each result with scope="org". A 404 is mapped to an empty slice
// by DoJSONListWithHeader. 401/403 surface as forgejo.ErrUnauthorized.
//
// The response headers come back alongside the labels because the bounded
// org-labels resource pages: it requests exactly the caller's limit and reads
// Link/X-Total-Count to decide whether more rows exist. Tool callers that
// enumerate without bounds can discard them.
func fetchOrgLabels(ctx context.Context, org string, page, limit int) ([]ScopedLabel, http.Header, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	path := forgejo.APIPath("orgs", org, "labels") + fmt.Sprintf("?page=%d&limit=%d", page, limit)
	var raw []*forgejo_sdk.Label
	header, err := forgejo.DoJSONListWithHeader(ctx, http.MethodGet, path, &raw)
	if err != nil {
		return nil, header, err
	}
	out := make([]ScopedLabel, 0, len(raw))
	for _, l := range raw {
		out = append(out, ScopedLabel{Label: l, Scope: "org"})
	}
	return out, header, nil
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
	SearchIssuesToolName       = "search_issues"
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
		mcp.WithString("sort", mcp.Description("Server-side sort order. One of: relevance, latest, oldest, recentupdate, leastupdate, mostcomment, leastcomment, nearduedate, farduedate. Default is the API's own default (latest).")),
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
		mcp.WithString("due_date", mcp.Description("Set the issue's due date (RFC3339, e.g. 2026-08-20T00:00:00Z). Mutually exclusive with 'clear_due_date'; setting both is an error.")),
		mcp.WithBoolean("clear_due_date", mcp.Description("Clear the issue's due date. Mutually exclusive with 'due_date'.")),
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

	SearchIssuesTool = mcp.NewTool(
		SearchIssuesToolName,
		mcp.WithDescription("Search issues across every repository belonging to one owner (organization or user), without naming a repo. "+
			"Returns a response envelope {issues, page, limit, count, has_next, total_count} rather than a bare array. "+
			"has_next true means a further page may exist; re-issue the call with page incremented to fetch it. "+
			"total_count, when present, is the total number of matching issues across all pages (from Forgejo's X-Total-Count header); it is omitted rather than 0 when the header is unavailable. "+
			"Each issue identifies its source repository only via its html_url/url field; there is no separate repository field. "+
			"Use list_repo_issues instead when the repo is already known."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("state", mcp.Description("State (open|closed|all)"), mcp.DefaultString("open")),
		mcp.WithString("type", mcp.Description("Type (issues|pulls)")),
		mcp.WithString("labels", mcp.Description("Labels (comma-separated); OR semantics — an issue matching any listed label is returned")),
		mcp.WithString("milestones", mcp.Description("Milestone names/IDs (comma-separated)")),
		mcp.WithString("q", mcp.Description(params.Keyword+" (matches issue title, body, and comments)")),
		mcp.WithString("created_by", mcp.Description("Filter by creator username")),
		mcp.WithString("assigned_by", mcp.Description("Filter by assignee username")),
		mcp.WithString("mentioned_by", mcp.Description("Filter by mentioned username")),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
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
	s.AddTool(SearchIssuesTool, SearchIssuesFn)
	RegisterLabelTool(s)
	RegisterDependencyTool(s)
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
		return to.ErrorResult(fmt.Errorf("get issue err: %w", err))
	}
	return to.TextResult(issue)
}

func ListRepoIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoIssuesFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	if owner == "" {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	if repo == "" {
		return to.ErrorResult(fmt.Errorf("repo is required; for owner-wide listing across repositories use search_issues instead"))
	}
	state, ok := req.GetArguments()["state"].(string)
	if !ok {
		state = "open"
	}
	issueType, _ := req.GetArguments()["type"].(string)
	milestones, _ := req.GetArguments()["milestones"].(string)
	labels, _ := req.GetArguments()["labels"].(string)
	sort, _ := req.GetArguments()["sort"].(string)
	page, _ := to.Float64(req.GetArguments()["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(req.GetArguments()["limit"])
	if limit == 0 {
		limit = 20
	}

	if sort != "" && !isValidIssueSort(sort) {
		return to.ErrorResult(fmt.Errorf("invalid sort %q: must be one of %s", sort, strings.Join(issueSortValues, ", ")))
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

	// The Forgejo API supports `sort` server-side on this endpoint (including
	// nearduedate/farduedate — see routers/api/v1/repo/issue.go's ListIssues
	// swagger comment), but the vendored SDK's ListIssueOption has no Sort
	// field to carry it (v3.0.0 QueryEncode never emits &sort=...). Go around
	// the SDK client with the raw-HTTP helper for this one param, same
	// pattern as fetchOrgLabels above.
	//
	// DoJSON, not DoJSONList: DoJSONList maps a 404 to an empty list, which is
	// right for endpoints where "none exist" really does 404 (fetchOrgLabels),
	// but /repos/{owner}/{repo}/issues returns 200 [] for a repo with no
	// issues — a 404 there means the repository does not exist. Swallowing it
	// would make a typo'd repo name read as "no issues", and only when the
	// caller happened to pass `sort`.
	if sort != "" {
		query := opt.QueryEncode() + "&sort=" + url.QueryEscape(sort)
		path := forgejo.APIPath("repos", owner, repo, "issues") + "?" + query
		var issues []*forgejo_sdk.Issue
		if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &issues); err != nil {
			return to.ErrorResult(fmt.Errorf("get issues list err: %w", err))
		}
		return to.TextResult(issues)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issues, _, err := client.ListRepoIssues(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issues list err: %w", err))
	}
	return to.TextResult(issues)
}

// searchIssuesEnvelope is the response shape for search_issues: the issues
// together with a continuation signal, per docs/design/output-bounding.md.
type searchIssuesEnvelope struct {
	Issues     []*forgejo_sdk.Issue `json:"issues"`
	Page       int                  `json:"page"`
	Limit      int                  `json:"limit"`
	Count      int                  `json:"count"`
	HasNext    bool                 `json:"has_next"`
	TotalCount *int                 `json:"total_count,omitempty"`
}

func SearchIssuesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SearchIssuesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	if owner == "" {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}

	state, ok := args["state"].(string)
	if !ok || state == "" {
		state = "open"
	}
	issueType, _ := args["type"].(string)
	labels, _ := args["labels"].(string)
	milestones, _ := args["milestones"].(string)
	keyword, _ := args["q"].(string)
	createdBy, _ := args["created_by"].(string)
	assignedBy, _ := args["assigned_by"].(string)
	mentionedBy, _ := args["mentioned_by"].(string)

	page, _ := to.Float64(args["page"])
	if page < 1 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit < 1 {
		limit = 20
	}
	requestedLimit := int(limit)

	// Instance pagination ceiling (design.md Decision 3): known ceilings are
	// enforced by rejecting an over-ceiling request before the upstream call
	// rather than silently clamping it, so the response never claims a limit
	// larger than what was actually applied. An unknown ceiling (settings
	// endpoint unreachable/403) falls back to the same-limit next-page probe
	// below.
	ceiling, ceilingKnown := forgejo.MaxResponseItems(ctx)
	if ceilingKnown && requestedLimit > ceiling {
		return to.ErrorResult(fmt.Errorf(
			"limit %d exceeds this instance's maximum of %d (max_response_items); retry with limit <= %d",
			requestedLimit, ceiling, ceiling,
		))
	}
	// effectiveLimit is the limit actually sent upstream. It feeds both the
	// SDK call and the response envelope from the same variable so the two
	// can never diverge; see the effective-limit reporting requirement in
	// specs/search-issues/spec.md.
	effectiveLimit := requestedLimit

	opt := forgejo_sdk.ListIssueOption{
		Owner: owner,
		State: forgejo_sdk.StateType(state),
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: effectiveLimit,
		},
	}
	if issueType != "" {
		opt.Type = forgejo_sdk.IssueType(issueType)
	}
	if labels != "" {
		opt.Labels = strings.Split(labels, ",")
	}
	if milestones != "" {
		opt.Milestones = strings.Split(milestones, ",")
	}
	if keyword != "" {
		opt.KeyWord = keyword
	}
	if createdBy != "" {
		opt.CreatedBy = createdBy
	}
	if assignedBy != "" {
		opt.AssignedBy = assignedBy
	}
	if mentionedBy != "" {
		opt.MentionedBy = mentionedBy
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issues, resp, err := client.ListIssues(opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("search issues err: %w", err))
	}
	count := len(issues)
	var totalCount *int
	if resp != nil {
		totalCount = forgejo.TotalCountPtr(resp.Header)
	}

	// has_next (design.md Decision 3, revised): when the ceiling is known,
	// the rejection above guarantees effectiveLimit <= ceiling, so the
	// request was honored in full and the simple count == limit rule is
	// sound again — a full page may have more, a short page cannot. That
	// rule was unsound only because nothing clamped limit client-side; now
	// something does. When the ceiling is unknown, fall back to the
	// same-limit next-page probe: never limit+1, since Forgejo derives page
	// offsets from page size and a changed page size would make later pages
	// skip rows.
	var hasNext bool
	if ceilingKnown {
		hasNext = count == effectiveLimit
	} else if count > 0 {
		probeOpt := opt
		probeOpt.Page = int(page) + 1
		nextIssues, _, err := client.ListIssues(probeOpt)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("probe next issues page err: %w", err))
		}
		hasNext = len(nextIssues) > 0
	}

	return to.TextResult(searchIssuesEnvelope{
		Issues:     issues,
		Page:       int(page),
		Limit:      effectiveLimit,
		Count:      count,
		HasNext:    hasNext,
		TotalCount: totalCount,
	})
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
		return to.ErrorResult(fmt.Errorf("create issue err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("create issue comment err: %w", err))
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
	dueDate, _ := req.GetArguments()["due_date"].(string)
	clearDueDate, _ := req.GetArguments()["clear_due_date"].(bool)

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
			return to.ErrorResult(fmt.Errorf("invalid milestone ID: %w", err))
		}
		opt.Milestone = &milestoneID
	}
	// due_date / clear_due_date map to the Forgejo API's due_date / unset_due_date
	// EditIssueOption fields (routers/api/v1/repo/issue.go's EditIssue: setting
	// unset_due_date=true clears the deadline; due_date sets it; both absent
	// leaves it unchanged). Mutually exclusive — ambiguous intent otherwise.
	switch {
	case clearDueDate && dueDate != "":
		return to.ErrorResult(fmt.Errorf("cannot set 'due_date' and 'clear_due_date' at the same time"))
	case clearDueDate:
		remove := true
		opt.RemoveDeadline = &remove
	case dueDate != "":
		parsed, err := time.Parse(time.RFC3339, dueDate)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid due_date format (expected RFC3339): %w", err))
		}
		opt.Deadline = &parsed
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	issue, _, err := client.EditIssue(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update issue err: %w", err))
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
			return to.ErrorResult(fmt.Errorf("invalid label ID '%s': %w - labels must be numeric IDs", labelStr, err))
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
		return to.ErrorResult(fmt.Errorf("add issue labels err: %w", err))
	}

	// Fetch the updated issue to return it with the new labels
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
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
			return to.ErrorResult(fmt.Errorf("invalid label ID '%s': %w - labels must be numeric IDs", labelStr, err))
		}
		_, err = client.DeleteIssueLabel(owner, repo, int64(index), labelID)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("remove issue label err: %w", err))
		}
	}

	// Fetch the updated issue to return it with the updated labels
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("change issue state err: %w", err))
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
			return to.ErrorResult(fmt.Errorf("invalid since time format (expected RFC3339): %w", err))
		}
		opt.Since = sinceTime
	}
	if before != "" {
		beforeTime, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("invalid before time format (expected RFC3339): %w", err))
		}
		opt.Before = beforeTime
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	comments, _, err := client.ListIssueComments(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list issue comments err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("get issue comment err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("edit issue comment err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("delete issue comment err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("list repo milestones err: %w", err))
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
		return to.ErrorResult(fmt.Errorf("list repo labels err: %w", err))
	}
	merged := make([]ScopedLabel, 0, len(repoLabels))
	for _, l := range repoLabels {
		merged = append(merged, ScopedLabel{Label: l, Scope: "repo"})
	}

	if includeOrg {
		orgLabels, _, oerr := fetchOrgLabels(ctx, owner, int(page), int(limit))
		if oerr != nil {
			return to.ErrorResult(fmt.Errorf("list org labels err: %w", oerr))
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

	labels, _, err := fetchOrgLabels(ctx, org, int(page), int(limit))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org labels err: %w", err))
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
