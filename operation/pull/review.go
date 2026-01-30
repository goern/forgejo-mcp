package pull

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/params"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreatePullReviewToolName      = "create_pull_review"
	SubmitPullReviewToolName      = "submit_pull_review"
	DismissPullReviewToolName     = "dismiss_pull_review"
	DeletePullReviewToolName      = "delete_pull_review"
	CreateReviewRequestsToolName  = "create_review_requests"
	DeleteReviewRequestsToolName  = "delete_review_requests"
)

var (
	CreatePullReviewTool = mcp.NewTool(
		CreatePullReviewToolName,
		mcp.WithDescription("Create a pull request review with optional inline comments"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("body", mcp.Description(params.ReviewBody)),
		mcp.WithString("state", mcp.Required(), mcp.Description(params.ReviewState)),
		mcp.WithString("comments", mcp.Description(params.ReviewComments)),
	)

	SubmitPullReviewTool = mcp.NewTool(
		SubmitPullReviewToolName,
		mcp.WithDescription("Submit a pending pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description(params.ReviewID)),
		mcp.WithString("body", mcp.Description(params.ReviewBody)),
		mcp.WithString("state", mcp.Required(), mcp.Description(params.ReviewState)),
	)

	DismissPullReviewTool = mcp.NewTool(
		DismissPullReviewToolName,
		mcp.WithDescription("Dismiss a pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description(params.ReviewID)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.DismissMessage)),
	)

	DeletePullReviewTool = mcp.NewTool(
		DeletePullReviewToolName,
		mcp.WithDescription("Delete a pending pull request review"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description(params.ReviewID)),
	)

	CreateReviewRequestsTool = mcp.NewTool(
		CreateReviewRequestsToolName,
		mcp.WithDescription("Request reviews from specific users or teams"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("reviewers", mcp.Description(params.Reviewers)),
		mcp.WithString("team_reviewers", mcp.Description(params.TeamReviewers)),
	)

	DeleteReviewRequestsTool = mcp.NewTool(
		DeleteReviewRequestsToolName,
		mcp.WithDescription("Cancel pending review requests"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.PRIndex)),
		mcp.WithString("reviewers", mcp.Description(params.Reviewers)),
		mcp.WithString("team_reviewers", mcp.Description(params.TeamReviewers)),
	)
)

func RegisterReviewTools(s *server.MCPServer) {
	s.AddTool(CreatePullReviewTool, CreatePullReviewFn)
	s.AddTool(SubmitPullReviewTool, SubmitPullReviewFn)
	s.AddTool(DismissPullReviewTool, DismissPullReviewFn)
	s.AddTool(DeletePullReviewTool, DeletePullReviewFn)
	s.AddTool(CreateReviewRequestsTool, CreateReviewRequestsFn)
	s.AddTool(DeleteReviewRequestsTool, DeleteReviewRequestsFn)
}

func CreatePullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreatePullReviewFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	body, _ := req.GetArguments()["body"].(string)
	state, _ := req.GetArguments()["state"].(string)
	commentsJSON, _ := req.GetArguments()["comments"].(string)

	opt := forgejo_sdk.CreatePullReviewOptions{
		State: forgejo_sdk.ReviewStateType(state),
		Body:  body,
	}

	if commentsJSON != "" {
		var comments []forgejo_sdk.CreatePullReviewComment
		if err := json.Unmarshal([]byte(commentsJSON), &comments); err != nil {
			return to.ErrorResult(fmt.Errorf("invalid comments JSON: %v", err))
		}
		opt.Comments = comments
	}

	review, _, err := forgejo.Client().CreatePullReview(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create pull review err: %v", err))
	}
	return to.TextResult(review)
}

func SubmitPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called SubmitPullReviewFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	id, _ := req.GetArguments()["id"].(float64)
	body, _ := req.GetArguments()["body"].(string)
	state, _ := req.GetArguments()["state"].(string)

	opt := forgejo_sdk.SubmitPullReviewOptions{
		State: forgejo_sdk.ReviewStateType(state),
		Body:  body,
	}

	review, _, err := forgejo.Client().SubmitPullReview(owner, repo, int64(index), int64(id), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("submit pull review err: %v", err))
	}
	return to.TextResult(review)
}

func DismissPullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DismissPullReviewFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	id, _ := req.GetArguments()["id"].(float64)
	message, _ := req.GetArguments()["message"].(string)

	opt := forgejo_sdk.DismissPullReviewOptions{
		Message: message,
	}

	_, err := forgejo.Client().DismissPullReview(owner, repo, int64(index), int64(id), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("dismiss pull review err: %v", err))
	}
	return mcp.NewToolResultText(`{"Result":"review dismissed successfully"}`), nil
}

func DeletePullReviewFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeletePullReviewFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	id, _ := req.GetArguments()["id"].(float64)

	_, err := forgejo.Client().DeletePullReview(owner, repo, int64(index), int64(id))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete pull review err: %v", err))
	}
	return mcp.NewToolResultText(`{"Result":"review deleted successfully"}`), nil
}

func CreateReviewRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateReviewRequestsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	reviewers, _ := req.GetArguments()["reviewers"].(string)
	teamReviewers, _ := req.GetArguments()["team_reviewers"].(string)

	opt := forgejo_sdk.PullReviewRequestOptions{}
	if reviewers != "" {
		opt.Reviewers = splitCSV(reviewers)
	}
	if teamReviewers != "" {
		opt.TeamReviewers = splitCSV(teamReviewers)
	}

	_, err := forgejo.Client().CreateReviewRequests(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create review requests err: %v", err))
	}
	return mcp.NewToolResultText(`{"Result":"review requests created successfully"}`), nil
}

func DeleteReviewRequestsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReviewRequestsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := req.GetArguments()["index"].(float64)
	reviewers, _ := req.GetArguments()["reviewers"].(string)
	teamReviewers, _ := req.GetArguments()["team_reviewers"].(string)

	opt := forgejo_sdk.PullReviewRequestOptions{}
	if reviewers != "" {
		opt.Reviewers = splitCSV(reviewers)
	}
	if teamReviewers != "" {
		opt.TeamReviewers = splitCSV(teamReviewers)
	}

	_, err := forgejo.Client().DeleteReviewRequests(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete review requests err: %v", err))
	}
	return mcp.NewToolResultText(`{"Result":"review requests deleted successfully"}`), nil
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
