package user

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const ownerResourceURITemplate = "forgejo://owner/{owner}"

// ownerResourcePayload is the JSON body returned for user or org.
type ownerResourcePayload struct {
	Login            string `json:"login"`
	FullName         string `json:"full_name,omitempty"`
	HTMLURL          string `json:"html_url,omitempty"`
	Kind             string `json:"kind"` // "user" or "org"
	Description      string `json:"description,omitempty"`
	Location         string `json:"location,omitempty"`
	Website          string `json:"website,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	FollowersCount   int    `json:"followers_count,omitempty"`
	FollowingCount   int    `json:"following_count,omitempty"`
	PublicReposCount int    `json:"public_repos_count,omitempty"`
}

// RegisterOwnerResource registers the forgejo://owner/{owner} resource template.
func RegisterOwnerResource(s *server.MCPServer) {
	resource.RegisterTemplate(
		s,
		ownerResourceURITemplate,
		"Forgejo Owner",
		ownerResourceHandler,
		mcp.WithTemplateDescription(
			"User or organization profile addressed by login. "+
				"Tries user first; falls back to org if user returns 404. "+
				"URI: forgejo://owner/{owner}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered owner resource template")
}

func ownerResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseOwner(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	u, userResp, userErr := client.GetUserInfo(params.Owner)

	var payload ownerResourcePayload

	if userErr == nil && u != nil {
		payload = ownerResourcePayload{
			Login:          u.UserName,
			FullName:       u.FullName,
			HTMLURL:        u.HTMLURL,
			Kind:           "user",
			Description:    u.Description,
			Location:       u.Location,
			Website:        u.Website,
			CreatedAt:      u.Created.Format("2006-01-02T15:04:05Z07:00"),
			FollowersCount: u.FollowerCount,
			FollowingCount: u.FollowingCount,
		}
	} else {
		// if user fetch failed for a reason other than 404, surface it immediately
		if userErr != nil && !is404(userResp, userErr) {
			if userResp != nil {
				return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", userResp.StatusCode, userErr.Error()))
			}
			return nil, resource.MapForgejoError(uri, userErr)
		}

		// fall back to org
		org, orgResp, orgErr := client.GetOrg(params.Owner)
		if orgErr != nil {
			if orgResp != nil {
				return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", orgResp.StatusCode, orgErr.Error()))
			}
			return nil, resource.MapForgejoError(uri, userErr) // surface original 404
		}

		payload = ownerResourcePayload{
			Login:       org.UserName,
			FullName:    org.FullName,
			Kind:        "org",
			Description: org.Description,
			Location:    org.Location,
			Website:     org.Website,
		}
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal owner payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}, nil
}

// is404 returns true if the response status or error message indicates HTTP 404.
func is404(resp *forgejo_sdk.Response, err error) bool {
	if resp != nil && resp.StatusCode == 404 {
		return true
	}
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") || strings.Contains(msg, "not found")
}
