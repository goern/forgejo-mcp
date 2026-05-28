package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const repoResourceURITemplate = "forgejo://repo/{owner}/{repo}"

// repoResourcePayload is the counts-only repository summary returned as JSON.
type repoResourcePayload struct {
	Owner           string `json:"owner"`
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description,omitempty"`
	HTMLURL         string `json:"html_url"`
	DefaultBranch   string `json:"default_branch"`
	Fork            bool   `json:"fork"`
	Archived        bool   `json:"archived"`
	Private         bool   `json:"private"`
	StarsCount      int    `json:"stars_count"`
	ForksCount      int    `json:"forks_count"`
	WatchersCount   int    `json:"watchers_count"`
	OpenIssuesCount int    `json:"open_issues_count"`
	OpenPRCount     int    `json:"open_pr_count"`
	Size            int    `json:"size"`
	HasIssues       bool   `json:"has_issues"`
	HasWiki         bool   `json:"has_wiki"`
	HasPullRequests bool   `json:"has_pull_requests"`
}

// RegisterRepoResource registers the forgejo://repo/{owner}/{repo} resource template.
func RegisterRepoResource(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		repoResourceURITemplate,
		"Forgejo Repository",
		repoResourceHandler,
		mcp.WithTemplateDescription(
			"Repository overview by name. Returns immutable identity (owner, name) plus "+
				"mutable counts (stars, forks, open issues, open PRs). No embedded lists. "+
				"URI: forgejo://repo/{owner}/{repo}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered repo resource template")
}

func repoResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseRepo(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid repo resource URI: %w", err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	r, resp, err := client.GetRepo(params.Owner, params.Repo)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	ownerLogin := params.Owner
	if r.Owner != nil {
		ownerLogin = r.Owner.UserName
	}

	payload := repoResourcePayload{
		Owner:           ownerLogin,
		Name:            r.Name,
		FullName:        r.FullName,
		Description:     r.Description,
		HTMLURL:         r.HTMLURL,
		DefaultBranch:   r.DefaultBranch,
		Fork:            r.Fork,
		Archived:        r.Archived,
		Private:         r.Private,
		StarsCount:      r.Stars,
		ForksCount:      r.Forks,
		WatchersCount:   r.Watchers,
		OpenIssuesCount: r.OpenIssues,
		OpenPRCount:     r.OpenPulls,
		Size:            r.Size,
		HasIssues:       r.HasIssues,
		HasWiki:         r.HasWiki,
		HasPullRequests: r.HasPullRequests,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal repo payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}, nil
}
