package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const statusResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}/status"

// statusItem is the per-context CI status shared by the commit status
// resource and get_commit_statuses.
type statusItem struct {
	Context     string `json:"context"`
	State       string `json:"state"`
	TargetURL   string `json:"target_url,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// statusResourcePayload is the JSON body returned as the primary content block.
type statusResourcePayload struct {
	SHA        string       `json:"sha"`
	State      string       `json:"state"`
	TotalCount int          `json:"total_count"`
	Statuses   []statusItem `json:"statuses"`
	Truncated  bool         `json:"truncated,omitempty"`
	ListTool   string       `json:"list_tool,omitempty"`
}

// RegisterStatusResource registers the forgejo://repo/{owner}/{repo}/commit/{sha}/status
// resource template. The resource aggregates per-context CI statuses for a commit SHA and
// returns a combined state. This is the canonical "is the commit green?" surface. The
// state is computed from the latest status of each context. The resource is pinned to
// an immutable SHA, but statuses on it still change while CI runs, so a cached read can
// be stale until the runs finish.
func RegisterStatusResource(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		statusResourceURITemplate,
		"Forgejo Commit Status",
		statusResourceHandler,
		mcp.WithTemplateDescription(
			"Aggregated CI status for a Forgejo repository commit. "+
				"URI: forgejo://repo/{owner}/{repo}/commit/{sha}/status — sha must be a full 40-character hex SHA. "+
				"Returns combined state (success|failure|pending|unknown) plus bounded per-context statuses. "+
				"Reports the latest status of each context; superseded statuses are not counted. "+
				"Pinned to an immutable SHA, but the state changes while CI runs. "+
				"Use get_commit_statuses tool for the paginated status history.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit status resource template")
}

func statusResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseStatus(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	// /status, not /statuses: the latter is the append-only history, where a run
	// that went pending then success is two rows and the pending one never leaves.
	// /status holds the latest row per context. Not client.GetCombinedStatus, which
	// cannot set a limit or return X-Total-Count.
	//
	// Request EmbeddedListCap+1 items so resource.Bounded can distinguish
	// "exactly at cap" from "over cap" and surface the truncated sentinel.
	// Forgejo's server default page size equals EmbeddedListCap (30), so
	// omitting the limit silently caps responses at the cap and the >cap check
	// in Bounded never fires.
	var combined struct {
		Statuses []*forgejo_sdk.Status `json:"statuses"`
	}
	path := forgejo.APIPath("repos", params.Owner, params.Repo, "commits", params.SHA, "status") +
		"?limit=" + strconv.Itoa(resource.EmbeddedListCap+1)
	header, err := forgejo.DoJSONWithHeader(ctx, http.MethodGet, path, nil, &combined)
	// Forgejo has answered a commit with no statuses with an empty body.
	if err != nil && !errors.Is(err, io.EOF) {
		var he *forgejo.HTTPError
		if errors.As(err, &he) {
			// Not err itself: its text holds the URL, and MapForgejoError matches
			// substrings, so a SHA containing "403" would read as access denied.
			err = fmt.Errorf("%s: %s", he.Status, he.Body)
		}
		return nil, resource.MapForgejoError(uri, err)
	}
	statuses := combined.Statuses

	items := make([]string, 0, len(statuses))
	itemData := make([]statusItem, 0, len(statuses))
	for _, s := range statuses {
		item, ok := toStatusItem(s)
		if !ok {
			continue
		}
		itemData = append(itemData, item)
		items = append(items, item.State)
	}

	bounded := resource.Bounded(items, resource.EmbeddedListCap, "get_commit_statuses")

	// X-Total-Count is the real number of contexts. The body's total_count is
	// only the page length, and an instance may cap the page below our limit.
	total := len(itemData)
	if n := forgejo.TotalCountPtr(header); n != nil && *n > total {
		total = *n
		bounded = bounded.WithMoreRemaining(total)
	}

	boundedItems := itemData
	if len(boundedItems) > resource.EmbeddedListCap {
		boundedItems = boundedItems[:resource.EmbeddedListCap]
	}

	state := computeAggregateState(statuses)
	// Only the fetched contexts were counted, so all-green proves nothing about
	// the rest. A failure or pending seen here still stands.
	if bounded.Truncated && state == "success" {
		state = "unknown"
	}

	payload := statusResourcePayload{
		SHA:        params.SHA,
		State:      state,
		TotalCount: total,
		Statuses:   boundedItems,
		Truncated:  bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = "get_commit_statuses"
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal status payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}, nil
}

func toStatusItem(s *forgejo_sdk.Status) (statusItem, bool) {
	if s == nil {
		return statusItem{}, false
	}
	return statusItem{
		Context:     s.Context,
		State:       string(s.State),
		TargetURL:   s.TargetURL,
		Description: s.Description,
		CreatedAt:   s.Created.Format(time.RFC3339),
	}, true
}

func computeAggregateState(statuses []*forgejo_sdk.Status) string {
	hasFailure := false
	hasPending := false
	allSuccess := true
	seen := 0
	for _, s := range statuses {
		if s == nil {
			continue
		}
		seen++
		switch s.State {
		case forgejo_sdk.StatusFailure, forgejo_sdk.StatusError:
			hasFailure = true
			allSuccess = false
		case forgejo_sdk.StatusPending:
			hasPending = true
			allSuccess = false
		case forgejo_sdk.StatusSuccess:
			// counted by allSuccess default
		default:
			allSuccess = false
		}
	}
	if seen == 0 {
		return "unknown"
	}
	switch {
	case hasFailure:
		return "failure"
	case hasPending:
		return "pending"
	case allSuccess:
		return "success"
	default:
		return "unknown"
	}
}
