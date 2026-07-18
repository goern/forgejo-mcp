// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterWikiResource(s *server.MCPServer) {
	resource.RegisterTemplate(s,
		"forgejo://repo/{owner}/{repo}/wiki/{pageName}",
		"Forgejo Wiki Page",
		wikiResourceHandler,
		mcp.WithTemplateDescription("Single wiki page metadata plus a text/markdown sidecar. URI: forgejo://repo/{owner}/{repo}/wiki/{pageName}. Use the server-normalized page name and percent-encode characters such as slash (%2F) and space (%20), for example Guides%2FGetting%20Started. If slash-bearing names cannot resolve through the resource, use get_wiki_page."),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered wiki resource template")
}

type wikiRevisionRef struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Message string `json:"message"`
}

type wikiResourcePayload struct {
	Owner           string            `json:"owner"`
	Repo            string            `json:"repo"`
	PageName        string            `json:"page_name"`
	Title           string            `json:"title"`
	CommitSHA       string            `json:"commit_sha"`
	RecentRevisions []wikiRevisionRef `json:"recent_revisions"`
	Truncated       bool              `json:"truncated,omitempty"`
	RevisionCount   int               `json:"revision_count,omitempty"`
	ListTool        string            `json:"list_tool,omitempty"`
	Sentinel        string            `json:"sentinel,omitempty"`
}

func wikiResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseWiki(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	page, err := forgejo.GetWikiPage(ctx, p.Owner, p.Repo, p.PageName)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	decoded, err := base64.StdEncoding.DecodeString(page.ContentBase64)
	if err != nil {
		return nil, resource.MapForgejoError(uri, fmt.Errorf("decode wiki page content: %w", err))
	}

	refs := make([]wikiRevisionRef, 0)
	revisionIDs := make([]string, 0)
	revisionCount := 0
	if revisions, revisionErr := forgejo.GetWikiPageRevisions(ctx, p.Owner, p.Repo, p.PageName, 1, resource.EmbeddedListCap+1); revisionErr == nil {
		revisionCount = revisions.Count
		for _, revision := range revisions.Commits {
			refs = append(refs, wikiRevisionRef{SHA: revision.SHA, Author: revision.Author.Name, Message: revision.Message})
			revisionIDs = append(revisionIDs, revision.SHA)
		}
	}
	bounded := resource.Bounded(revisionIDs, resource.EmbeddedListCap, GetWikiRevisionsToolName)
	// The revisions endpoint returns the repository-wide count separately from the
	// cap+1 window. Preserve that authoritative total in the shared sentinel.
	if revisionCount > bounded.Total {
		bounded.Total = revisionCount
		bounded.Truncated = revisionCount > resource.EmbeddedListCap
	}
	if len(refs) > resource.EmbeddedListCap {
		refs = refs[:resource.EmbeddedListCap]
	}
	payload := wikiResourcePayload{Owner: p.Owner, Repo: p.Repo, PageName: page.SubURL, Title: page.Title, CommitSHA: page.LastCommit.SHA, RecentRevisions: refs, Truncated: bounded.Truncated}
	if bounded.Truncated {
		payload.RevisionCount = bounded.Total
		payload.ListTool = bounded.ListTool
		payload.Sentinel = bounded.Sentinel()
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wiki payload: %w", err)
	}

	markdown := string(decoded)
	if len(markdown) > forgejo.MaxInlineDownloadBytes {
		marker := "\n\n[truncated: use get_wiki_page with start_line/end_line to retrieve the remainder.]"
		keep := forgejo.MaxInlineDownloadBytes - len(marker)
		for keep > 0 && !utf8.RuneStart(markdown[keep]) {
			keep--
		}
		markdown = markdown[:keep] + marker
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: markdown},
	}, nil
}
