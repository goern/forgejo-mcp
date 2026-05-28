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

const commitResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}"

// RegisterCommitResource registers the forgejo://repo/{owner}/{repo}/commit/{sha} resource template.
func RegisterCommitResource(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		commitResourceURITemplate,
		"Forgejo Commit",
		commitResourceHandler,
		mcp.WithTemplateDescription(
			"Immutable commit metadata for a Forgejo repository commit. "+
				"URI: forgejo://repo/{owner}/{repo}/commit/{sha} — sha must be a full 40-character hex SHA. "+
				"Returns JSON metadata plus a text/markdown body sidecar for the commit message.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit resource template")
}

func commitResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseCommit(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	commit, resp, err := client.GetSingleCommit(params.Owner, params.Repo, params.SHA)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	jsonBytes, err := json.Marshal(commit)
	if err != nil {
		return nil, fmt.Errorf("marshal commit: %w", err)
	}

	contents := []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}

	if commit.RepoCommit != nil && commit.RepoCommit.Message != "" {
		contents = append(contents, mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     commit.RepoCommit.Message,
		})
	}

	return contents, nil
}
