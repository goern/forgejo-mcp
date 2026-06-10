// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLabelResources registers the label resource templates.
func RegisterLabelResources(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/label/{id}",
		"Forgejo Repo Label",
		repoLabelResourceHandler,
		mcp.WithTemplateDescription(
			"Single repository label by id. "+
				"URI: forgejo://repo/{owner}/{repo}/label/{id}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/labels",
		"Forgejo Repo Labels",
		repoLabelsResourceHandler,
		mcp.WithTemplateDescription(
			"Bounded list of repository labels (page/limit client-controlled, ceiling "+
				strconv.Itoa(resource.EmbeddedListCap)+"). "+
				"URI: forgejo://repo/{owner}/{repo}/labels{?page,limit}. "+
				"Use list_repo_labels tool for the unbounded enumeration path.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		"forgejo://org/{org}/labels",
		"Forgejo Org Labels",
		orgLabelsResourceHandler,
		mcp.WithTemplateDescription(
			"Bounded list of organization-level labels (page/limit client-controlled, ceiling "+
				strconv.Itoa(resource.EmbeddedListCap)+"). "+
				"URI: forgejo://org/{org}/labels{?page,limit}. "+
				"Use list_org_labels tool for the unbounded enumeration path.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered label resource templates")
}

// ---- single repo label ----

type labelResourcePayload struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func repoLabelResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseLabel(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	label, resp, err := client.GetRepoLabel(p.Owner, p.Repo, p.ID)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	payload := labelResourcePayload{
		ID:          label.ID,
		Name:        label.Name,
		Color:       label.Color,
		Description: label.Description,
		URL:         label.URL,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal label payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

// ---- repo labels list ----

type labelsListPayload struct {
	Owner     string                 `json:"owner"`
	Repo      string                 `json:"repo"`
	Labels    []labelResourcePayload `json:"labels"`
	Truncated bool                   `json:"truncated,omitempty"`
	ListTool  string                 `json:"list_tool,omitempty"`
	Sentinel  string                 `json:"sentinel,omitempty"`
}

func repoLabelsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseLabels(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	page, limit := pageLimit(req)

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// Request cap+1 so Bounded's >cap check fires.
	fetch := limit + 1
	if fetch > resource.EmbeddedListCap+1 {
		fetch = resource.EmbeddedListCap + 1
	}

	rawLabels, resp, err := client.ListRepoLabels(p.Owner, p.Repo, forgejo_sdk.ListLabelsOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: fetch},
	})
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	items := make([]string, len(rawLabels))
	for i, l := range rawLabels {
		items[i] = strconv.FormatInt(l.ID, 10)
	}
	bounded := resource.Bounded(items, limit, ListRepoLabelsToolName)

	labels := make([]labelResourcePayload, 0, len(bounded.Items))
	for _, l := range rawLabels {
		if len(labels) >= len(bounded.Items) {
			break
		}
		labels = append(labels, labelResourcePayload{
			ID:          l.ID,
			Name:        l.Name,
			Color:       l.Color,
			Description: l.Description,
			URL:         l.URL,
		})
	}

	payload := labelsListPayload{
		Owner:     p.Owner,
		Repo:      p.Repo,
		Labels:    labels,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = ListRepoLabelsToolName
		payload.Sentinel = bounded.Sentinel()
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal labels payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

// ---- org labels list ----

type orgLabelsListPayload struct {
	Org       string                 `json:"org"`
	Labels    []labelResourcePayload `json:"labels"`
	Truncated bool                   `json:"truncated,omitempty"`
	ListTool  string                 `json:"list_tool,omitempty"`
	Sentinel  string                 `json:"sentinel,omitempty"`
}

func orgLabelsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseOrgLabels(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	page, limit := pageLimit(req)

	// Request cap+1 so Bounded's >cap check fires.
	fetch := limit + 1
	if fetch > resource.EmbeddedListCap+1 {
		fetch = resource.EmbeddedListCap + 1
	}

	rawLabels, err := fetchOrgLabels(ctx, p.Org, page, fetch)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	items := make([]string, len(rawLabels))
	for i, l := range rawLabels {
		items[i] = strconv.FormatInt(l.ID, 10)
	}
	bounded := resource.Bounded(items, limit, ListOrgLabelsToolName)

	labels := make([]labelResourcePayload, 0, len(bounded.Items))
	for _, l := range rawLabels {
		if len(labels) >= len(bounded.Items) {
			break
		}
		labels = append(labels, labelResourcePayload{
			ID:          l.ID,
			Name:        l.Name,
			Color:       l.Color,
			Description: l.Description,
			URL:         l.URL,
		})
	}

	payload := orgLabelsListPayload{
		Org:       p.Org,
		Labels:    labels,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = ListOrgLabelsToolName
		payload.Sentinel = bounded.Sentinel()
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal org labels payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

// pageLimit extracts page and limit from resource URI query params with
// EmbeddedListCap as the ceiling for limit. Malformed or absent values
// fall back to page=1, limit=EmbeddedListCap.
func pageLimit(req mcp.ReadResourceRequest) (page, limit int) {
	page = 1
	limit = resource.EmbeddedListCap
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return page, limit
	}
	q := u.Query()
	if v, err := strconv.Atoi(q.Get("page")); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v >= 1 {
		limit = v
		if limit > resource.EmbeddedListCap {
			limit = resource.EmbeddedListCap
		}
	}
	return page, limit
}
