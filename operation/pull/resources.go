package pull

import (
	"context"
	"encoding/json"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterPullResources registers the forgejo://repo/{owner}/{repo}/pr/{index} resource template.
func RegisterPullResources(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/pr/{index}",
		"Forgejo Pull Request",
		prResourceHandler,
		mcp.WithTemplateDescription(
			"Pull request metadata, head/base refs, mergeability, body sidecar, "+
				"and bounded recent comments and reviews. "+
				"URI: forgejo://repo/{owner}/{repo}/pr/{index}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered pull request resource template")
}

type prBranchRef struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
}

type prCommentRef struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	BodyExcerpt string `json:"body_excerpt"`
}

type prReviewRef struct {
	ID          int64  `json:"id"`
	Reviewer    string `json:"reviewer"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
	BodyExcerpt string `json:"body_excerpt"`
}

type prResourcePayload struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Index     int64  `json:"index"`
	Title     string `json:"title"`
	State     string `json:"state"` // "open", "closed", "merged"
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at,omitempty"`
	MergedAt  string `json:"merged_at,omitempty"`
	Mergeable bool   `json:"mergeable"`

	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
	Milestone string   `json:"milestone,omitempty"`

	Head prBranchRef `json:"head"`
	Base prBranchRef `json:"base"`

	CommentCount int `json:"comment_count"`
	ReviewCount  int `json:"review_count"`

	RecentComments    []prCommentRef `json:"recent_comments"`
	CommentsTruncated bool           `json:"comments_truncated,omitempty"`
	CommentsListTool  string         `json:"comments_list_tool,omitempty"`

	RecentReviews    []prReviewRef `json:"recent_reviews"`
	ReviewsTruncated bool          `json:"reviews_truncated,omitempty"`
	ReviewsListTool  string        `json:"reviews_list_tool,omitempty"`
}

func prResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParsePR(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	pr, resp, err := client.GetPullRequest(params.Owner, params.Repo, params.Index)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	// fetch comments (PR comments live on the issue-comments endpoint)
	comments, _, _ := client.ListIssueComments(params.Owner, params.Repo, params.Index, forgejo_sdk.ListIssueCommentOptions{})

	// fetch reviews
	reviews, _, _ := client.ListPullReviews(params.Owner, params.Repo, params.Index, forgejo_sdk.ListPullReviewsOptions{})

	// bound comments
	commentItems := make([]string, len(comments))
	commentRefs := make([]prCommentRef, len(comments))
	for i, c := range comments {
		author := ""
		if c.Poster != nil {
			author = c.Poster.UserName
		}
		excerpt := c.Body
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		commentRefs[i] = prCommentRef{
			ID:          c.ID,
			Author:      author,
			CreatedAt:   c.Created.Format("2006-01-02T15:04:05Z07:00"),
			BodyExcerpt: excerpt,
		}
		commentItems[i] = fmt.Sprintf("%d", c.ID)
	}
	boundedComments := resource.Bounded(commentItems, resource.EmbeddedListCap, "list_issue_comments")
	boundedCommentRefs := commentRefs
	if boundedComments.Truncated {
		boundedCommentRefs = commentRefs[:resource.EmbeddedListCap]
	}

	// bound reviews
	reviewItems := make([]string, len(reviews))
	reviewRefs := make([]prReviewRef, len(reviews))
	for i, r := range reviews {
		reviewer := ""
		if r.Reviewer != nil {
			reviewer = r.Reviewer.UserName
		}
		excerpt := r.Body
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		reviewRefs[i] = prReviewRef{
			ID:          r.ID,
			Reviewer:    reviewer,
			State:       string(r.State),
			SubmittedAt: r.Submitted.Format("2006-01-02T15:04:05Z07:00"),
			BodyExcerpt: excerpt,
		}
		reviewItems[i] = fmt.Sprintf("%d", r.ID)
	}
	boundedReviews := resource.Bounded(reviewItems, resource.EmbeddedListCap, "list_pull_reviews")
	boundedReviewRefs := reviewRefs
	if boundedReviews.Truncated {
		boundedReviewRefs = reviewRefs[:resource.EmbeddedListCap]
	}

	// compute state string
	state := string(pr.State)
	if pr.HasMerged {
		state = "merged"
	}

	author := ""
	if pr.Poster != nil {
		author = pr.Poster.UserName
	}

	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, l.Name)
	}
	assignees := make([]string, 0, len(pr.Assignees))
	for _, a := range pr.Assignees {
		assignees = append(assignees, a.UserName)
	}
	milestone := ""
	if pr.Milestone != nil {
		milestone = pr.Milestone.Title
	}

	createdAt, updatedAt, closedAt, mergedAt := "", "", "", ""
	if pr.Created != nil {
		createdAt = pr.Created.Format("2006-01-02T15:04:05Z07:00")
	}
	if pr.Updated != nil {
		updatedAt = pr.Updated.Format("2006-01-02T15:04:05Z07:00")
	}
	if pr.Closed != nil {
		closedAt = pr.Closed.Format("2006-01-02T15:04:05Z07:00")
	}
	if pr.Merged != nil {
		mergedAt = pr.Merged.Format("2006-01-02T15:04:05Z07:00")
	}

	headRef, baseRef := prBranchRef{}, prBranchRef{}
	if pr.Head != nil {
		headRef = prBranchRef{Label: pr.Head.Name, Ref: pr.Head.Ref, SHA: pr.Head.Sha}
	}
	if pr.Base != nil {
		baseRef = prBranchRef{Label: pr.Base.Name, Ref: pr.Base.Ref, SHA: pr.Base.Sha}
	}

	payload := prResourcePayload{
		Owner:             params.Owner,
		Repo:              params.Repo,
		Index:             params.Index,
		Title:             pr.Title,
		State:             state,
		Author:            author,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
		ClosedAt:          closedAt,
		MergedAt:          mergedAt,
		Mergeable:         pr.Mergeable,
		Labels:            labels,
		Assignees:         assignees,
		Milestone:         milestone,
		Head:              headRef,
		Base:              baseRef,
		CommentCount:      pr.Comments,
		ReviewCount:       len(reviews),
		RecentComments:    boundedCommentRefs,
		CommentsTruncated: boundedComments.Truncated,
		RecentReviews:     boundedReviewRefs,
		ReviewsTruncated:  boundedReviews.Truncated,
	}
	if boundedComments.Truncated {
		payload.CommentsListTool = "list_issue_comments"
	}
	if boundedReviews.Truncated {
		payload.ReviewsListTool = "list_pull_reviews"
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal PR payload: %w", err)
	}

	mdSidecar := fmt.Sprintf("# %s\n%s · #%d · %s · %s\nhead: %s · base: %s\n\n%s",
		pr.Title, state, params.Index, author, createdAt,
		headRef.Label, baseRef.Label, pr.Body)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: mdSidecar},
	}, nil
}
