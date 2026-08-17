// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// resourceTimeFormat matches the timestamp rendering the single-issue and
// single-comment resources already use, so a client sees one time format
// across every issue-domain resource.
const resourceTimeFormat = "2006-01-02T15:04:05Z07:00"

// hasMore reports whether the server said another page exists. The SDK parses
// the response's `Link: …; rel="next"` header into Response.NextPage, so this
// is upstream's own answer rather than an inference from how many rows arrived
// — which is what lets these resources page without over-fetching.
func hasMore(resp *forgejo_sdk.Response) bool {
	return resp != nil && resp.NextPage != 0
}

// totalCount reads Forgejo's X-Total-Count header, the authoritative number of
// rows matching the query. Returns 0 when the header is absent or unparseable,
// which the sentinel renders as "total unknown" rather than guessing.
func totalCount(resp *forgejo_sdk.Response) int {
	if resp == nil || resp.Response == nil {
		return 0
	}
	return headerTotalCount(resp.Header)
}

// headerHasMore is hasMore for the raw-HTTP path, which has no SDK Response to
// parse the Link header for it. Same question, same answer: did the server
// advertise a next page.
func headerHasMore(h http.Header) bool {
	for _, link := range h.Values("Link") {
		// A single Link value may carry several comma-separated relations,
		// so this is a substring test rather than a parse. Forgejo quotes
		// the rel; tolerate the unquoted form too.
		if strings.Contains(link, `rel="next"`) || strings.Contains(link, "rel=next") {
			return true
		}
	}
	return false
}

// headerTotalCount is totalCount for a bare http.Header.
func headerTotalCount(h http.Header) int {
	n, err := strconv.Atoi(h.Get("X-Total-Count"))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ---- repo issues list ----

// issueRef is one row of the bounded issue list: enough to decide which issue to
// open, and nothing more. Bodies are deliberately absent — the per-issue resource
// (forgejo://repo/{owner}/{repo}/issue/{index}) carries the body and comments.
type issueRef struct {
	Index        int64    `json:"index"`
	Title        string   `json:"title"`
	State        string   `json:"state"`
	Author       string   `json:"author"`
	Labels       []string `json:"labels"`
	Assignees    []string `json:"assignees,omitempty"`
	Milestone    string   `json:"milestone,omitempty"`
	CommentCount int      `json:"comment_count"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	DueDate      string   `json:"due_date,omitempty"`
	IsPull       bool     `json:"is_pull,omitempty"`
}

type issuesListPayload struct {
	Owner     string     `json:"owner"`
	Repo      string     `json:"repo"`
	State     string     `json:"state"`
	Labels    []string   `json:"labels,omitempty"`
	Page      int        `json:"page"`
	Limit     int        `json:"limit"`
	Issues    []issueRef `json:"issues"`
	Truncated bool       `json:"truncated,omitempty"`
	ListTool  string     `json:"list_tool,omitempty"`
	Sentinel  string     `json:"sentinel,omitempty"`
}

// repoIssuesResourceHandler serves forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}.
//
// WHY THIS EXISTS: list_repo_issues returns whole issue objects — every body, plus
// a full user object per issue. A triage question ("what is open and labelled X?")
// therefore costs a payload proportional to the prose in the repo rather than to
// the number of rows. Measured on one real repo: 53 open issues rendered as
// index/title/labels is ~5 KB, while the same query as full issue objects is two
// orders of magnitude larger. This resource is the row-shaped answer; callers open
// the per-issue resource for anything they actually want to read.
func repoIssuesResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseIssues(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	page, limit := pageLimit(req)
	state, labels := issuesQuery(req)

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// PageSize MUST equal the caller's limit. The label resources over-fetch by
	// one to make truncation detectable, but they are not paged: upstream
	// computes the offset as (page-1)*PageSize, so a PageSize of limit+1 that
	// hands back only limit rows makes page N+1 start one row past the last row
	// page N showed, and that row is unreachable from any page the client can
	// ask for. "More exists" comes from the response's Link header instead.
	opt := forgejo_sdk.ListIssueOption{
		State: forgejo_sdk.StateType(state),
		ListOptions: forgejo_sdk.ListOptions{
			Page:     page,
			PageSize: limit,
		},
	}
	if len(labels) > 0 {
		opt.Labels = labels
	}

	rawIssues, resp, err := client.ListRepoIssues(p.Owner, p.Repo, opt)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	items := make([]string, len(rawIssues))
	for i, iss := range rawIssues {
		items[i] = strconv.FormatInt(iss.Index, 10)
	}
	bounded := resource.Bounded(items, limit, ListRepoIssuesToolName)
	if hasMore(resp) {
		bounded = bounded.WithMoreRemaining(totalCount(resp))
	}

	refs := make([]issueRef, 0, len(bounded.Items))
	for _, iss := range rawIssues {
		if len(refs) >= len(bounded.Items) {
			break
		}
		refs = append(refs, issueRefOf(iss))
	}

	payload := issuesListPayload{
		Owner:     p.Owner,
		Repo:      p.Repo,
		State:     state,
		Labels:    labels,
		Page:      page,
		Limit:     limit,
		Issues:    refs,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = ListRepoIssuesToolName
		payload.Sentinel = bounded.Sentinel()
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal issues payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

func issueRefOf(iss *forgejo_sdk.Issue) issueRef {
	if iss == nil {
		return issueRef{}
	}
	labels := make([]string, 0, len(iss.Labels))
	for _, l := range iss.Labels {
		if l != nil {
			labels = append(labels, l.Name)
		}
	}
	assignees := make([]string, 0, len(iss.Assignees))
	for _, a := range iss.Assignees {
		if a != nil {
			assignees = append(assignees, a.UserName)
		}
	}
	author := ""
	if iss.Poster != nil {
		author = iss.Poster.UserName
	}
	milestone := ""
	if iss.Milestone != nil {
		milestone = iss.Milestone.Title
	}
	dueDate := ""
	if iss.Deadline != nil {
		dueDate = iss.Deadline.Format(resourceTimeFormat)
	}
	return issueRef{
		Index:        iss.Index,
		Title:        iss.Title,
		State:        string(iss.State),
		Author:       author,
		Labels:       labels,
		Assignees:    assignees,
		Milestone:    milestone,
		CommentCount: iss.Comments,
		CreatedAt:    iss.Created.Format(resourceTimeFormat),
		UpdatedAt:    iss.Updated.Format(resourceTimeFormat),
		DueDate:      dueDate,
		IsPull:       iss.PullRequest != nil,
	}
}

// issuesQuery reads the state and labels filters from the URI query string.
// state defaults to "open" — the same default list_repo_issues uses — because a
// resource read with no filters should answer "what is live", not "everything
// that ever existed".
func issuesQuery(req mcp.ReadResourceRequest) (state string, labels []string) {
	state = "open"
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return state, nil
	}
	q := u.Query()
	switch v := q.Get("state"); v {
	case "open", "closed", "all":
		state = v
	}
	if raw := q.Get("labels"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				labels = append(labels, trimmed)
			}
		}
	}
	return state, labels
}

// ---- issue/PR comment thread ----

// commentBody is one comment with its FULL text. The per-issue resource excerpts
// comments at 200 characters, which is right for deciding whether to read a thread
// and wrong for actually reading it; this resource is the second half of that pair.
type commentBody struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Body      string `json:"body"`
}

type commentsListPayload struct {
	Owner     string        `json:"owner"`
	Repo      string        `json:"repo"`
	Kind      string        `json:"kind"`
	Index     int64         `json:"index"`
	Page      int           `json:"page"`
	Limit     int           `json:"limit"`
	Comments  []commentBody `json:"comments"`
	Truncated bool          `json:"truncated,omitempty"`
	ListTool  string        `json:"list_tool,omitempty"`
	Sentinel  string        `json:"sentinel,omitempty"`
}

// issueCommentsResourceHandler serves
// forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}.
//
// Reading a whole thread previously meant either the excerpted comments on the
// issue resource or one resource read per comment id. This is the paged middle:
// full bodies, client-controlled bound, and the same truncation sentinel every
// other embedded list uses.
func issueCommentsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseIssueComments(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	page, limit := pageLimit(req)

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// PageSize == limit, for the paging reason spelled out in
	// repoIssuesResourceHandler above.
	//
	// PR comments use the same issue-comment API, exactly as the single-comment
	// resource does — the index is the shared issue/PR index.
	rawComments, resp, err := client.ListIssueComments(p.Owner, p.Repo, p.Index, forgejo_sdk.ListIssueCommentOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: limit},
	})
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	// Forgejo's issue-comments endpoint ignores page and limit: it returns the
	// whole thread whatever is asked for. Verified against a 69-comment thread —
	// `?limit=5` returns all 69, and `?page=2` returns the same rows as page 1.
	// It sends X-Total-Count but no Link header. (The issues endpoint honours
	// both and sends both headers, which is why only this resource needs the
	// following.)
	//
	// So the page offset is applied here when the server declined to apply it.
	// The test is a property of the response rather than a hardcoded assumption,
	// so this corrects itself if Forgejo starts honouring the bounds: then the
	// rows already are the requested page and slicing them again would wrongly
	// return nothing for page 2.
	//
	// The property is "upstream handed back the entire thread", which is true
	// when either:
	//
	//   - more rows arrived than were asked for; or
	//   - exactly as many rows arrived as X-Total-Count says the thread holds.
	//
	// The row count alone cannot carry this. A server that honours the bounds
	// also returns exactly `limit` rows for any full page, so "len >= limit"
	// would double-slice it. The total is what separates them: a paging server
	// on page N>1 returns at most total-(N-1)*limit rows, which is strictly
	// fewer than total, so the equality can only hold for a server that ignored
	// the offset. On page 1 the offset is 0 and the slice is a no-op either way.
	window := rawComments
	offset := 0
	total := totalCount(resp)
	serverIgnoredBounds := len(rawComments) > limit || (total > 0 && len(rawComments) == total)
	if serverIgnoredBounds {
		offset = min((page-1)*limit, len(rawComments))
		window = rawComments[offset:min(offset+limit, len(rawComments))]
	}

	items := make([]string, len(window))
	for i, c := range window {
		items[i] = strconv.FormatInt(c.ID, 10)
	}
	bounded := resource.Bounded(items, limit, ListIssueCommentsToolName)
	switch {
	case serverIgnoredBounds:
		// Everything is in hand, so the row count is the authoritative total.
		if offset+len(window) < len(rawComments) {
			bounded = bounded.WithMoreRemaining(len(rawComments))
		}
	case hasMore(resp):
		bounded = bounded.WithMoreRemaining(total)
	}

	comments := make([]commentBody, 0, len(bounded.Items))
	for _, c := range window {
		if len(comments) >= len(bounded.Items) {
			break
		}
		author := ""
		if c.Poster != nil {
			author = c.Poster.UserName
		}
		updated := ""
		if !c.Updated.IsZero() && !c.Updated.Equal(c.Created) {
			updated = c.Updated.Format(resourceTimeFormat)
		}
		comments = append(comments, commentBody{
			ID:        c.ID,
			Author:    author,
			CreatedAt: c.Created.Format(resourceTimeFormat),
			UpdatedAt: updated,
			Body:      c.Body,
		})
	}

	payload := commentsListPayload{
		Owner:     p.Owner,
		Repo:      p.Repo,
		Kind:      p.Kind,
		Index:     p.Index,
		Page:      page,
		Limit:     limit,
		Comments:  comments,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = ListIssueCommentsToolName
		payload.Sentinel = bounded.Sentinel()
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal comments payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}
