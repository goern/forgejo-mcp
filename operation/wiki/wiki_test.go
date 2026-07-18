// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"github.com/mark3labs/mcp-go/mcp"
)

func wikiRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func wikiServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "test"
	return srv
}

func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	content, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("not text content")
	}
	return content.Text
}

func TestGetWikiPageDecodesAndSlicesLines(t *testing.T) {
	content := "a\r\nb\r\n"
	wikiServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"title":"T","sub_url":"T","content_base64":"` + base64.StdEncoding.EncodeToString([]byte(content)) + `","last_commit":{"sha":"abc"}}`))
	})
	result, err := GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "T", "start_line": float64(1), "end_line": float64(2)}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"total_lines":3`) || !strings.Contains(text, `a\r\nb\r`) {
		t.Fatalf("unexpected result: %s", text)
	}
}

func TestGetWikiPageRejectsInvertedRange(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"title":"T","sub_url":"T","content_base64":"YQpi"}`))
	})
	_, err := GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "T", "start_line": float64(2), "end_line": float64(1)}))
	if err == nil {
		t.Fatal("expected inverted range error")
	}
}

func TestUpdateWithoutTitleReadsAndPreservesTitle(t *testing.T) {
	requests := 0
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"title":"Getting Started","sub_url":"Getting-Started","content_base64":""}`))
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		if !strings.Contains(string(body), `"title":"Getting Started"`) {
			t.Fatalf("title not preserved: %s", body)
		}
		_, _ = w.Write([]byte(`{"title":"Getting Started","sub_url":"Getting-Started"}`))
	})
	_, err := UpdateWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "Getting-Started", "content": "new"}))
	if err != nil || requests != 2 {
		t.Fatalf("requests=%d err=%v", requests, err)
	}
}
