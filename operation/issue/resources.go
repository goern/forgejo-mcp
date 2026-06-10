package issue

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

// RegisterIssueResources registers the issue and comment resource templates.
func RegisterIssueResources(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/issue/{index}",
		"Forgejo Issue",
		issueResourceHandler,
		mcp.WithTemplateDescription(
			"Single issue metadata, rendered body, and bounded recent comments. "+
				"URI: forgejo://repo/{owner}/{repo}/issue/{index}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}",
		"Forgejo Comment",
		commentResourceHandler,
		mcp.WithTemplateDescription(
			"Single comment by id. kind ∈ {issue, pr}. "+
				"PR comments use the same Forgejo issue-comment API. "+
				"URI: forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered issue resource templates")
}

// ---- issue handler ----

type commentRef struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	BodyExcerpt string `json:"body_excerpt"`
}

type issueResourcePayload struct {
	Owner          string       `json:"owner"`
	Repo           string       `json:"repo"`
	Index          int64        `json:"index"`
	Title          string       `json:"title"`
	State          string       `json:"state"`
	Author         string       `json:"author"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
	ClosedAt       string       `json:"closed_at,omitempty"`
	Labels         []string     `json:"labels"`
	Assignees      []string     `json:"assignees"`
	Milestone      string       `json:"milestone,omitempty"`
	CommentCount   int          `json:"comment_count"`
	RecentComments []commentRef `json:"recent_comments"`
	Truncated      bool         `json:"truncated,omitempty"`
	ListTool       string       `json:"list_tool,omitempty"`
}

func issueResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseIssue(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	iss, resp, err := client.GetIssue(params.Owner, params.Repo, params.Index)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	// Request EmbeddedListCap+1 items so resource.Bounded can distinguish
	// "exactly at cap" from "over cap" and surface the truncated sentinel.
	// Forgejo's server default page size equals EmbeddedListCap (30), so
	// leaving PageSize at zero silently caps responses at the cap and the
	// >cap check in Bounded never fires.
	comments, _, _ := client.ListIssueComments(params.Owner, params.Repo, params.Index, forgejo_sdk.ListIssueCommentOptions{
		ListOptions: forgejo_sdk.ListOptions{PageSize: resource.EmbeddedListCap + 1},
	})

	items := make([]string, len(comments))
	refs := make([]commentRef, len(comments))
	for i, c := range comments {
		author := ""
		if c.Poster != nil {
			author = c.Poster.UserName
		}
		excerpt := resource.Excerpt(c.Body, 200)
		refs[i] = commentRef{
			ID:          c.ID,
			Author:      author,
			CreatedAt:   c.Created.Format("2006-01-02T15:04:05Z07:00"),
			BodyExcerpt: excerpt,
		}
		items[i] = fmt.Sprintf("%d", c.ID)
	}

	bounded := resource.Bounded(items, resource.EmbeddedListCap, "list_issue_comments")
	boundedRefs := refs
	if bounded.Truncated {
		boundedRefs = refs[:resource.EmbeddedListCap]
	}

	labels := make([]string, 0, len(iss.Labels))
	for _, l := range iss.Labels {
		labels = append(labels, l.Name)
	}
	assignees := make([]string, 0, len(iss.Assignees))
	for _, a := range iss.Assignees {
		assignees = append(assignees, a.UserName)
	}

	author := ""
	if iss.Poster != nil {
		author = iss.Poster.UserName
	}
	milestone := ""
	if iss.Milestone != nil {
		milestone = iss.Milestone.Title
	}
	closedAt := ""
	if iss.Closed != nil {
		closedAt = iss.Closed.Format("2006-01-02T15:04:05Z07:00")
	}

	payload := issueResourcePayload{
		Owner:          params.Owner,
		Repo:           params.Repo,
		Index:          params.Index,
		Title:          iss.Title,
		State:          string(iss.State),
		Author:         author,
		CreatedAt:      iss.Created.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      iss.Updated.Format("2006-01-02T15:04:05Z07:00"),
		ClosedAt:       closedAt,
		Labels:         labels,
		Assignees:      assignees,
		Milestone:      milestone,
		CommentCount:   iss.Comments,
		RecentComments: boundedRefs,
		Truncated:      bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = "list_issue_comments"
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal issue payload: %w", err)
	}

	mdSidecar := fmt.Sprintf("# %s\nState: %s · #%d · %s · %s\n\n%s",
		iss.Title, iss.State, params.Index, author, iss.Created.Format("2006-01-02"), iss.Body)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: mdSidecar},
	}, nil
}

// ---- comment handler ----

type commentResourcePayload struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Kind      string `json:"kind"`
	Index     int64  `json:"index"`
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
}

func commentResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseComment(uri)
	if err != nil {
		// unknown kind and other parse errors → -32602
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// Forgejo PR comments ARE issue comments under the same API; kind only
	// affects display context, not the fetch path.
	c, resp, err := client.GetIssueComment(params.Owner, params.Repo, params.ID)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	author := ""
	if c.Poster != nil {
		author = c.Poster.UserName
	}

	payload := commentResourcePayload{
		Owner:     params.Owner,
		Repo:      params.Repo,
		Kind:      params.Kind,
		Index:     params.Index,
		ID:        c.ID,
		Author:    author,
		CreatedAt: c.Created.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: c.Updated.Format("2006-01-02T15:04:05Z07:00"),
		Body:      c.Body,
		HTMLURL:   c.HTMLURL,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal comment payload: %w", err)
	}

	mdSidecar := fmt.Sprintf("%s commented on %s#%d:\n%s", author, params.Kind, params.Index, c.Body)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: mdSidecar},
	}, nil
}
