package pull

import (
	"context"
	"fmt"
	"strconv"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/ptr"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetPullRequestByIndexToolName  = "get_pull_request_by_index"
	ListRepoPullRequestsToolName   = "list_repo_pull_requests"
	CreatePullRequestToolName      = "create_pull_request"
	UpdatePullRequestToolName      = "update_pull_request"
	ListPullReviewsToolName        = "list_pull_reviews"
	GetPullReviewToolName          = "get_pull_review"
	ListPullReviewCommentsToolName = "list_pull_review_comments"
	MergePullRequestToolName       = "merge_pull_request"
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

	ListPullReviewsTool = mcp.NewTool(
		ListPullReviewsToolName,
		mcp.WithDescription("List reviews for a pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetPullReviewTool = mcp.NewTool(
		GetPullReviewToolName,
		mcp.WithDescription("Get a specific pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Review ID")),
	)

	ListPullReviewCommentsTool = mcp.NewTool(
		ListPullReviewCommentsToolName,
		mcp.WithDescription("List comments on a pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Review ID")),
	)

	MergePullRequestTool = mcp.NewTool(
		MergePullRequestToolName,
		mcp.WithDescription("Merge a pull request"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("style", mcp.Required(), mcp.Description("Merge style (merge, rebase, rebase-merge, squash)")),
		mcp.WithString("title", mcp.Description("Merge commit title")),
		mcp.WithString("message", mcp.Description("Merge commit message")),
		mcp.WithBoolean("delete_branch_after_merge", mcp.Description("Delete head branch after merge")),
		mcp.WithBoolean("force_merge", mcp.Description("Force merge even if checks have not passed")),
		mcp.WithBoolean("merge_when_checks_succeed", mcp.Description("Schedule merge for when all checks succeed")),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetPullRequestByIndexTool, GetPullRequestByIndexFn)
	s.AddTool(ListRepoPullRequestsTool, ListRepoPullRequestsFn)
	s.AddTool(CreatePullRequestTool, CreatePullRequestFn)
	s.AddTool(UpdatePullRequestTool, UpdatePullRequestFn)
	s.AddTool(ListPullReviewsTool, ListPullReviewsFn)
	s.AddTool(GetPullReviewTool, GetPullReviewFn)
	s.AddTool(ListPullReviewCommentsTool, ListPullReviewCommentsFn)
	s.AddTool(MergePullRequestTool, MergePullRequestFn)
}

func GetPullRequestByIndexFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullRequestByIndexFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)

	pr, _, err := forgejo.Client().GetPullRequest(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull request err: %v", err))
	}
	return to.TextResult(pr)
}

func ListRepoPullRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoPullRequestsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	state, ok := req.GetArguments()["state"].(string)
	if !ok {
		state = "open"
	}
	sort, _ := req.GetArguments()["sort"].(string)
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
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
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	head, _ := req.GetArguments()["head"].(string)
	base, _ := req.GetArguments()["base"].(string)
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)

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
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	title, _ := req.GetArguments()["title"].(string)
	body, _ := req.GetArguments()["body"].(string)
	base, _ := req.GetArguments()["base"].(string)
	assignee, _ := req.GetArguments()["assignee"].(string)
	milestone, _ := req.GetArguments()["milestone"].(string)

	opt := forgejo_sdk.EditPullRequestOption{}

	if title != "" {
		opt.Title = title
	}
	if body != "" {
		opt.Body = ptr.To(body)
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

func ListPullReviewsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPullReviewsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 20
	}

	opt := forgejo_sdk.ListPullReviewsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	reviews, _, err := forgejo.Client().ListPullReviews(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list pull reviews err: %v", err))
	}
	return to.TextResult(reviews)
}

func GetPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPullReviewFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	id, _ := req.GetArguments()["id"].(float64)

	review, _, err := forgejo.Client().GetPullReview(owner, repo, int64(index), int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get pull review err: %v", err))
	}
	return to.TextResult(review)
}

func MergePullRequestFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called MergePullRequestFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	style, _ := req.GetArguments()["style"].(string)
	title, _ := req.GetArguments()["title"].(string)
	message, _ := req.GetArguments()["message"].(string)
	deleteBranch, _ := req.GetArguments()["delete_branch_after_merge"].(bool)
	forceMerge, _ := req.GetArguments()["force_merge"].(bool)
	mergeWhenChecks, _ := req.GetArguments()["merge_when_checks_succeed"].(bool)

	opt := forgejo_sdk.MergePullRequestOption{
		Style:                  forgejo_sdk.MergeStyle(style),
		DeleteBranchAfterMerge: deleteBranch,
		ForceMerge:             forceMerge,
		MergeWhenChecksSucceed: mergeWhenChecks,
	}

	if title != "" {
		opt.Title = title
	}
	if message != "" {
		opt.Message = message
	}

	_, _, err := forgejo.Client().MergePullRequest(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("merge pull request err: %v", err))
	}

	result := "Pull request merged successfully"
	if mergeWhenChecks {
		result = "Pull request scheduled to merge when all checks succeed"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(result)},
	}, nil
}

func ListPullReviewCommentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPullReviewCommentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	id, _ := req.GetArguments()["id"].(float64)

	comments, _, err := forgejo.Client().ListPullReviewComments(owner, repo, int64(index), int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list pull review comments err: %v", err))
	}
	return to.TextResult(comments)
}
