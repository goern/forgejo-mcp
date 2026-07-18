// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

func wikiPayload(t *testing.T, contents []mcp.ResourceContents) wikiResourcePayload {
	t.Helper()
	jsonBlock, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected text JSON block, got %#v", contents[0])
	}
	var payload wikiResourcePayload
	if err := json.Unmarshal([]byte(jsonBlock.Text), &payload); err != nil {
		t.Fatalf("decode JSON block: %v", err)
	}
	return payload
}

func TestWikiResourceReturnsMetadataAndMarkdown(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/revisions/") {
			_, _ = w.Write([]byte(`{"commits":[{"sha":"abc","author":{"name":"Ada"},"message":"create"}],"count":1}`))
			return
		}
		content := base64.StdEncoding.EncodeToString([]byte("# Hello\n"))
		_, _ = w.Write([]byte(`{"title":"Hello","sub_url":"Hello","content_base64":"` + content + `","last_commit":{"sha":"abc"}}`))
	})
	contents, err := wikiResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/o/r/wiki/Hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected JSON and markdown, got %d blocks", len(contents))
	}
	payload := wikiPayload(t, contents)
	if len(payload.RecentRevisions) != 1 || payload.RecentRevisions[0].Author != "Ada" {
		t.Fatalf("unexpected JSON payload: %#v", payload)
	}
	if payload.CommitSHA != "abc" || payload.Truncated || payload.Sentinel != "" {
		t.Fatalf("unexpected metadata/bounding: %#v", payload)
	}
	markdown, ok := contents[1].(mcp.TextResourceContents)
	if !ok || markdown.Text != "# Hello\n" {
		t.Fatalf("unexpected markdown block: %#v", contents[1])
	}
}

func TestWikiResourceDegradesWhenRevisionsFail(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/revisions/") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"title":"Hello","sub_url":"Hello","content_base64":""}`))
	})
	contents, err := wikiResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/o/r/wiki/Hello"}})
	if err != nil {
		t.Fatal(err)
	}
	payload := wikiPayload(t, contents)
	if payload.RecentRevisions == nil || len(payload.RecentRevisions) != 0 {
		t.Fatalf("expected degraded non-nil empty revisions: %#v", payload.RecentRevisions)
	}
	if payload.Truncated || payload.Sentinel != "" {
		t.Fatalf("degraded revisions must not claim truncation: %#v", payload)
	}
}

func TestWikiResourceBoundsRevisionsWithSharedSentinel(t *testing.T) {
	const total = resource.EmbeddedListCap + 7
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/revisions/") {
			commits := make([]string, resource.EmbeddedListCap+1)
			for i := range commits {
				commits[i] = `{"sha":"sha-` + strconv.Itoa(i) + `","author":{"name":"Ada"},"message":"edit"}`
			}
			_, _ = w.Write([]byte(`{"commits":[` + strings.Join(commits, ",") + `],"count":` + strconv.Itoa(total) + `}`))
			return
		}
		_, _ = w.Write([]byte(`{"title":"Hello","sub_url":"Hello","content_base64":"","last_commit":{"sha":"page-sha"}}`))
	})

	contents, err := wikiResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/o/r/wiki/Hello"}})
	if err != nil {
		t.Fatal(err)
	}
	payload := wikiPayload(t, contents)
	if len(payload.RecentRevisions) != resource.EmbeddedListCap {
		t.Fatalf("expected %d revisions, got %d", resource.EmbeddedListCap, len(payload.RecentRevisions))
	}
	if !payload.Truncated || payload.RevisionCount != total || payload.ListTool != GetWikiRevisionsToolName {
		t.Fatalf("unexpected bounding metadata: %#v", payload)
	}
	if !strings.Contains(payload.Sentinel, "[truncated:") || !strings.Contains(payload.Sentinel, strconv.Itoa(total)) || !strings.Contains(payload.Sentinel, GetWikiRevisionsToolName) {
		t.Fatalf("unexpected sentinel: %q", payload.Sentinel)
	}
	if payload.CommitSHA != "page-sha" {
		t.Fatalf("commit SHA must come from page payload, got %q", payload.CommitSHA)
	}
}

func TestWikiResourceMapsPrimaryHTTPError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   int
	}{
		{name: "forbidden", status: http.StatusForbidden, code: -32002},
		{name: "not found", status: http.StatusNotFound, code: -32003},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wikiServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			})
			uri := "forgejo://repo/o/r/wiki/Missing"
			_, err := wikiResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
			var resourceErr *resource.ResourceError
			if !errors.As(err, &resourceErr) || resourceErr.Code != tc.code {
				t.Fatalf("expected resource error %d, got %v", tc.code, err)
			}
		})
	}
}

func TestWikiResourceTruncatesMarkdownAtUTF8Boundary(t *testing.T) {
	const marker = "\n\n[truncated: use get_wiki_page with start_line/end_line to retrieve the remainder.]"
	keep := forgejo.MaxInlineDownloadBytes - len(marker)
	// Put the nominal byte boundary in the middle of a multi-byte rune.
	content := strings.Repeat("a", keep-1) + "€" + strings.Repeat("z", len(marker)+10)
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/revisions/") {
			_, _ = w.Write([]byte(`{"commits":[],"count":0}`))
			return
		}
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		_, _ = w.Write([]byte(`{"title":"Hello","sub_url":"Hello","content_base64":"` + encoded + `"}`))
	})
	contents, err := wikiResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/o/r/wiki/Hello"}})
	if err != nil {
		t.Fatal(err)
	}
	markdown := contents[1].(mcp.TextResourceContents).Text
	if !utf8.ValidString(markdown) {
		t.Fatal("truncated markdown is not valid UTF-8")
	}
	if len(markdown) > forgejo.MaxInlineDownloadBytes || !strings.Contains(markdown, "[truncated:") {
		t.Fatalf("unexpected truncated markdown length=%d", len(markdown))
	}
	if !strings.Contains(markdown, GetWikiPageToolName) {
		t.Fatalf("truncation marker must name %s", GetWikiPageToolName)
	}
}
