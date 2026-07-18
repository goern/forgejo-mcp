// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

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
		mcp.WithTemplateDescription("Single wiki page metadata plus a text/markdown sidecar. URI: forgejo://repo/{owner}/{repo}/wiki/{pageName}. Use the server-normalized page name; percent-encode slash as %2F. If slash-bearing names cannot resolve through the resource, use get_wiki_page."),
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
	revisionCount := 0
	if revisions, revisionErr := forgejo.GetWikiPageRevisions(ctx, p.Owner, p.Repo, p.PageName, 1, resource.EmbeddedListCap+1); revisionErr == nil {
		revisionCount = revisions.Count
		for _, revision := range revisions.Commits {
			refs = append(refs, wikiRevisionRef{SHA: revision.SHA, Author: revision.Author.Name, Message: revision.Message})
		}
	}
	truncated := len(refs) > resource.EmbeddedListCap || revisionCount > resource.EmbeddedListCap
	if len(refs) > resource.EmbeddedListCap {
		refs = refs[:resource.EmbeddedListCap]
	}
	payload := wikiResourcePayload{Owner: p.Owner, Repo: p.Repo, PageName: page.SubURL, Title: page.Title, CommitSHA: page.LastCommit.SHA, RecentRevisions: refs, Truncated: truncated}
	if truncated {
		payload.RevisionCount = revisionCount
		payload.ListTool = GetWikiRevisionsToolName
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wiki payload: %w", err)
	}

	markdown := string(decoded)
	if len(markdown) > forgejo.MaxInlineDownloadBytes {
		marker := "\n\n[truncated: use get_wiki_page with start_line/end_line to retrieve the remainder.]"
		keep := forgejo.MaxInlineDownloadBytes - len(marker)
		markdown = markdown[:keep] + marker
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: markdown},
	}, nil
}
