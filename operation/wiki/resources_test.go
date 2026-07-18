// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

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
	jsonBlock, ok := contents[0].(mcp.TextResourceContents)
	if !ok || !strings.Contains(jsonBlock.Text, `"recent_revisions"`) || !strings.Contains(jsonBlock.Text, `"author":"Ada"`) {
		t.Fatalf("unexpected JSON block: %#v", contents[0])
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
	jsonBlock := contents[0].(mcp.TextResourceContents)
	if !strings.Contains(jsonBlock.Text, `"recent_revisions":[]`) {
		t.Fatalf("expected degraded empty revisions: %s", jsonBlock.Text)
	}
}
