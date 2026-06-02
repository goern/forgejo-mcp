package branchprotection

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

const (
	branchProtectionsResourceURITemplate = "forgejo://repo/{owner}/{repo}/branch_protections"
	branchProtectionResourceURITemplate  = "forgejo://repo/{owner}/{repo}/branch_protection/{rule}"
)

// bpCollectionPayload is the JSON body for the branch_protections collection resource.
type bpCollectionPayload struct {
	Owner             string                          `json:"owner"`
	Repo              string                          `json:"repo"`
	TotalCount        int                             `json:"total_count"`
	BranchProtections []*forgejo_sdk.BranchProtection `json:"branch_protections"`
	Truncated         bool                            `json:"truncated,omitempty"`
	ListTool          string                          `json:"list_tool,omitempty"`
}

// RegisterResource registers the branch protection collection and single-rule
// resource templates on the MCP server.
func RegisterResource(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		branchProtectionsResourceURITemplate,
		"Forgejo Branch Protections",
		branchProtectionsResourceHandler,
		mcp.WithTemplateDescription(
			"Branch protection rules for a Forgejo repository. "+
				"URI: forgejo://repo/{owner}/{repo}/branch_protections. "+
				"Returns the rules as a read-only JSON document, bounded to the first "+
				"EmbeddedListCap rules; when truncated, use the list_branch_protections "+
				"tool to page through the remainder.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		branchProtectionResourceURITemplate,
		"Forgejo Branch Protection",
		branchProtectionResourceHandler,
		mcp.WithTemplateDescription(
			"A single branch protection rule for a Forgejo repository. "+
				"URI: forgejo://repo/{owner}/{repo}/branch_protection/{rule} — rule is the rule name. "+
				"Returns the rule's full protection state as a read-only JSON document.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered branch protection resource templates")
}

func branchProtectionsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseBranchProtections(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// Request EmbeddedListCap+1 so Bounded can distinguish "at cap" from "over cap".
	bps, resp, err := client.ListBranchProtections(p.Owner, p.Repo, forgejo_sdk.ListBranchProtectionsOptions{
		ListOptions: forgejo_sdk.ListOptions{PageSize: resource.EmbeddedListCap + 1},
	})
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	names := make([]string, len(bps))
	for i, bp := range bps {
		names[i] = bp.RuleName
	}
	bounded := resource.Bounded(names, resource.EmbeddedListCap, "list_branch_protections")

	shown := bps
	if bounded.Truncated {
		shown = bps[:resource.EmbeddedListCap]
	}

	payload := bpCollectionPayload{
		Owner:             p.Owner,
		Repo:              p.Repo,
		TotalCount:        len(bps),
		BranchProtections: shown,
		Truncated:         bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = "list_branch_protections"
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal branch protections payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

func branchProtectionResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseBranchProtection(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	bp, resp, err := client.GetBranchProtection(p.Owner, p.Repo, p.Rule)
	if err != nil {
		if resp != nil {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", resp.StatusCode, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	jsonBytes, err := json.Marshal(bp)
	if err != nil {
		return nil, fmt.Errorf("marshal branch protection payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}
