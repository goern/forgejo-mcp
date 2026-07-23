// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
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

func TestCreateWikiPageToolDocumentsSubpages(t *testing.T) {
	description := CreateWikiPageTool.Description
	for _, required := range []string{"Parent/Child", "flat naming convention", "no parent-child relationship", "does not create a parent page automatically", "returned page_name"} {
		if !strings.Contains(description, required) {
			t.Fatalf("create tool description does not document %q: %s", required, description)
		}
	}
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

func TestGetWikiPageTotalLinesMatchesForCRLFAndLF(t *testing.T) {
	bodies := []string{"a\r\nb\r\n", "a\nb\n"}
	request := 0
	wikiServer(t, func(w http.ResponseWriter, _ *http.Request) {
		content := bodies[request]
		request++
		_, _ = w.Write([]byte(`{"title":"T","sub_url":"T","content_base64":"` + base64.StdEncoding.EncodeToString([]byte(content)) + `"}`))
	})
	for _, content := range bodies {
		result, err := GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "T"}))
		if err != nil {
			t.Fatal(err)
		}
		if text := toolText(t, result); !strings.Contains(text, `"total_lines":3`) {
			t.Fatalf("body %q did not report three lines: %s", content, text)
		}
	}
}

func TestListWikiPagesIsBoundedAndUsesStableShape(t *testing.T) {
	requests := 0
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("limit") != "2" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`[
				{"title":"One","sub_url":"One","content_base64":"must-not-leak"},
				{"title":"Child","sub_url":"Guides%2FChild"}]`))
			return
		}
		if r.URL.Query().Get("page") != "3" {
			t.Fatalf("unexpected probe query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"title":"Extra","sub_url":"Extra"}]`))
	})
	result, err := ListWikiPagesFn(context.Background(), wikiRequest(map[string]any{
		"owner": "o", "repo": "r", "page": float64(2), "limit": float64(2),
	}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"page_name":"Guides%2FChild"`) || !strings.Contains(text, `"sub_url":"Guides%2FChild"`) || !strings.Contains(text, `"has_next":true`) {
		t.Fatalf("unexpected result: %s", text)
	}
	if strings.Contains(text, "content_base64") || strings.Contains(text, "Extra") {
		t.Fatalf("unbounded or unsafe result: %s", text)
	}
	if requests != 2 {
		t.Fatalf("expected current-page read plus next-page probe, got %d requests", requests)
	}
}

func TestListWikiPages404ReturnsEmptyArray(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	result, err := ListWikiPagesFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r"}))
	if err != nil {
		t.Fatal(err)
	}
	if text := toolText(t, result); !strings.Contains(text, `"pages":[]`) {
		t.Fatalf("empty list must be [], got %s", text)
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

func TestGetWikiPageRejectsInvalidBase64(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"title":"T","sub_url":"T","content_base64":"%%%"}`))
	})
	_, err := GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "T"}))
	if err == nil || !strings.Contains(err.Error(), "decode wiki page content") {
		t.Fatalf("expected explicit base64 error, got %v", err)
	}
}

func TestGetWikiRevisionsIsBoundedAndFlattensAuthor(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "1" {
			t.Fatalf("expected stable page size, got %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"commits":[{"sha":"a","author":{"name":"alice","email":"private@example.test"},"message":"one"}],"count":2}`))
			return
		}
		_, _ = w.Write([]byte(`{"commits":[{"sha":"b","author":{"name":"bob"},"message":"two"}],"count":2}`))
	})
	result, err := GetWikiRevisionsFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "T", "limit": float64(1)}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"author":"alice"`) || !strings.Contains(text, `"has_next":true`) || strings.Contains(text, "private@example.test") || strings.Contains(text, `"sha":"b"`) {
		t.Fatalf("unexpected result: %s", text)
	}
}

func TestGetWikiRevisions404IsError(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	_, err := GetWikiRevisionsFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "missing"}))
	if !errors.Is(err, forgejo.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCreateWikiPageReturnsSafeStableShape(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["content_base64"] != base64.StdEncoding.EncodeToString([]byte("secret markdown")) || !strings.Contains(body["message"], "My Page") {
			t.Fatalf("unexpected body: %v", body)
		}
		_, _ = w.Write([]byte(`{"title":"My Page","sub_url":"My+Page.-","content_base64":"must-not-leak","last_commit":{"sha":"abc"}}`))
	})
	result, err := CreateWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "title": "My Page", "content": "secret markdown"}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"page_name":"My+Page.-"`) || !strings.Contains(text, `"commit_sha":"abc"`) || strings.Contains(text, "content_base64") || strings.Contains(text, "must-not-leak") {
		t.Fatalf("unsafe or unstable result: %s", text)
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
		_, _ = w.Write([]byte(`{"title":"Getting Started","sub_url":"Getting-Started","content_base64":"must-not-leak","last_commit":{"sha":"updated"}}`))
	})
	result, err := UpdateWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "Getting-Started", "content": "new"}))
	if err != nil || requests != 2 {
		t.Fatalf("requests=%d err=%v", requests, err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"page_name":"Getting-Started"`) || !strings.Contains(text, `"commit_sha":"updated"`) || strings.Contains(text, "content_base64") {
		t.Fatalf("unsafe or unstable result: %s", text)
	}
}

func TestDeleteWikiPageReportsSuccess(t *testing.T) {
	wikiServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	result, err := DeleteWikiPageFn(context.Background(), wikiRequest(map[string]any{"owner": "o", "repo": "r", "page_name": "Guides%2FSetup"}))
	if err != nil {
		t.Fatal(err)
	}
	if text := toolText(t, result); !strings.Contains(text, `"deleted":true`) || !strings.Contains(text, `"page_name":"Guides%2FSetup"`) {
		t.Fatalf("unexpected result: %s", text)
	}
}
