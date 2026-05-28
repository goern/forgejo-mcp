package repo

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

const statusResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}/status"

// statusItem is the per-context entry included in the resource response payload.
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
// combined state is computed server-side from the per-context list. The resource is
// pinned to an immutable SHA; clients may cache responses keyed by URI.
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
				"Pinned to an immutable SHA; safe to cache by URI. "+
				"Use get_commit_statuses tool for paginated per-context enumeration.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit status resource template")
}

func statusResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseStatus(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid status resource URI: %w", err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	statuses, resp, err := client.ListStatuses(params.Owner, params.Repo, params.SHA, forgejo_sdk.ListStatusesOption{})
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	items := make([]string, len(statuses))
	itemData := make([]statusItem, len(statuses))
	for i, s := range statuses {
		itemData[i] = statusItem{
			Context:     s.Context,
			State:       string(s.State),
			TargetURL:   s.TargetURL,
			Description: s.Description,
			CreatedAt:   s.Created.Format("2006-01-02T15:04:05Z07:00"),
		}
		items[i] = string(s.State)
	}

	bounded := resource.Bounded(items, resource.EmbeddedListCap, "get_commit_statuses")

	boundedItems := itemData
	if bounded.Truncated {
		boundedItems = itemData[:resource.EmbeddedListCap]
	}

	state := computeAggregateState(statuses)

	payload := statusResourcePayload{
		SHA:        params.SHA,
		State:      state,
		TotalCount: len(statuses),
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

func computeAggregateState(statuses []*forgejo_sdk.Status) string {
	if len(statuses) == 0 {
		return "unknown"
	}
	hasFailure := false
	hasPending := false
	allSuccess := true
	for _, s := range statuses {
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
