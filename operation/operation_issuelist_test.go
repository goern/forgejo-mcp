// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests go through the server's own resource matcher rather than calling
// the handlers directly, because that matcher is where a whole class of defect
// lives: mcp-go builds an anchored regexp from the *registered* URI template
// string, so a template registered without its RFC 6570 query expansion matches
// only the bare URI. Every query-bearing read then fails with "resource not
// found" before any handler runs.
//
// The per-handler tests in operation/issue cannot see this. They construct a
// ReadResourceRequest themselves and invoke the handler directly, so they prove
// the handlers parse `state`, `labels`, `page` and `limit` correctly while the
// server refuses to route those URIs to them at all. That gap is the reason
// these live at dispatch level.

// issueListBackend serves the two endpoints the bounded list resources read,
// honouring `page` and `limit` so a query-bearing URI has an observable effect.
func issueListBackend(t *testing.T, totalIssues, totalComments int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if size < 1 {
			size = 10
		}
		offset := (page - 1) * size

		emit := func(total int, row func(i int) map[string]any) {
			rows := make([]any, 0, size)
			for i := offset + 1; i <= offset+size && i <= total; i++ {
				rows = append(rows, row(i))
			}
			w.Header().Set("X-Total-Count", strconv.Itoa(total))
			if offset+len(rows) < total {
				w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d&limit=%d>; rel="next"`, r.URL.Path, page+1, size))
			}
			_ = json.NewEncoder(w).Encode(rows)
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/comments"):
			emit(totalComments, func(i int) map[string]any {
				return map[string]any{
					"id":         i,
					"body":       fmt.Sprintf("comment %d", i),
					"user":       map[string]any{"login": "carol"},
					"created_at": "2026-08-03T09:00:00Z",
					"updated_at": "2026-08-03T09:00:00Z",
				}
			})
		case strings.HasSuffix(r.URL.Path, "/issues"):
			emit(totalIssues, func(i int) map[string]any {
				return map[string]any{
					"id":         i,
					"number":     i,
					"title":      fmt.Sprintf("issue %d", i),
					"body":       "a body that must never reach a list row",
					"state":      "open",
					"user":       map[string]any{"login": "alice"},
					"created_at": "2026-08-01T10:00:00Z",
					"updated_at": "2026-08-02T11:00:00Z",
				}
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	flag.URL = srv.URL
	flag.Token = "test"
	flag.UserAgent = "test"
	c, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("building test client: %v", err)
	}
	forgejo.SetClientForTesting(c)
}

// readResource dispatches a real resources/read envelope and returns the JSON
// payload of the first content block. A routing failure surfaces as a
// JSONRPCError, which is exactly the symptom being guarded against.
func readResource(t *testing.T, s *server.MCPServer, uri string) map[string]any {
	t.Helper()
	msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":%q}}`, uri)
	response := s.HandleMessage(context.Background(), []byte(msg))

	if errResp, isErr := response.(mcp.JSONRPCError); isErr {
		t.Fatalf("resources/read %s was not routed to a handler: %s (code %d)",
			uri, errResp.Error.Message, errResp.Error.Code)
	}
	rpc, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("resources/read %s returned an unexpected envelope: %#v", uri, response)
	}
	result, ok := rpc.Result.(mcp.ReadResourceResult)
	if !ok || len(result.Contents) == 0 {
		t.Fatalf("resources/read %s returned no contents: %#v", uri, rpc.Result)
	}
	text, ok := result.Contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("resources/read %s: first content block is not text: %#v", uri, result.Contents[0])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("resources/read %s: payload is not JSON: %v\n%s", uri, err, text.Text)
	}
	return payload
}

func rowCount(t *testing.T, payload map[string]any, key string) int {
	t.Helper()
	rows, ok := payload[key].([]any)
	if !ok {
		t.Fatalf("payload has no %q array: %#v", key, payload)
	}
	return len(rows)
}

func TestResourceDispatchRoutesIssueListTemplate(t *testing.T) {
	issueListBackend(t, 100, 69)
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)

	t.Run("bare URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/issues")
		// No limit given, so the resource's own ceiling applies.
		if got := rowCount(t, payload, "issues"); got != 30 {
			t.Fatalf("expected the default cap of 30 rows, got %d", got)
		}
	})

	t.Run("query-bearing URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/issues?limit=5")
		if got := rowCount(t, payload, "issues"); got != 5 {
			t.Fatalf("limit=5 did not take effect through dispatch: got %d rows", got)
		}
		if got, _ := payload["limit"].(float64); int(got) != 5 {
			t.Fatalf("payload did not echo the caller's limit: %v", payload["limit"])
		}
	})

	t.Run("query-bearing URI with filters and page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/issues?state=all&labels=security,%20triage&page=2&limit=5")
		if got := rowCount(t, payload, "issues"); got != 5 {
			t.Fatalf("expected 5 rows on page 2, got %d", got)
		}
		if got, _ := payload["state"].(string); got != "all" {
			t.Fatalf("state=all did not survive dispatch: %v", payload["state"])
		}
		if got, _ := payload["page"].(float64); int(got) != 2 {
			t.Fatalf("page=2 did not survive dispatch: %v", payload["page"])
		}
		labels, _ := payload["labels"].([]any)
		if len(labels) != 2 || labels[1] != "triage" {
			t.Fatalf("labels did not survive dispatch trimmed: %#v", payload["labels"])
		}
		// Page 2 at limit 5 must start after page 1's last row.
		rows, _ := payload["issues"].([]any)
		first, _ := rows[0].(map[string]any)
		if idx, _ := first["index"].(float64); int(idx) != 6 {
			t.Fatalf("page 2 did not start at row 6, got %v", first["index"])
		}
	})
}

func TestResourceDispatchRoutesCommentThreadTemplate(t *testing.T) {
	issueListBackend(t, 100, 69)
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)

	t.Run("bare URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/issue/42/comments")
		if got := rowCount(t, payload, "comments"); got != 30 {
			t.Fatalf("expected the default cap of 30 comments, got %d", got)
		}
	})

	t.Run("query-bearing URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/issue/42/comments?page=2&limit=5")
		if got := rowCount(t, payload, "comments"); got != 5 {
			t.Fatalf("limit=5 did not take effect through dispatch: got %d comments", got)
		}
		if got, _ := payload["page"].(float64); int(got) != 2 {
			t.Fatalf("page=2 did not survive dispatch: %v", payload["page"])
		}
	})

	t.Run("query-bearing URI on the pr kind", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/pr/7/comments?limit=3")
		if got := rowCount(t, payload, "comments"); got != 3 {
			t.Fatalf("limit=3 did not take effect for kind=pr: got %d comments", got)
		}
		if got, _ := payload["kind"].(string); got != "pr" {
			t.Fatalf("kind=pr did not survive dispatch: %v", payload["kind"])
		}
	})
}

// TestResourceDispatchRejectsUnknownIssueListURI keeps the query expansion from
// widening the match: adding {?…} must not turn the template into something
// that swallows neighbouring paths.
func TestResourceDispatchRejectsUnknownIssueListURI(t *testing.T) {
	issueListBackend(t, 10, 10)
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)

	for _, uri := range []string{
		"forgejo://repo/o/r/issues/extra",
		"forgejo://repo/o/r/issue/42/comments/9",
	} {
		msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":%q}}`, uri)
		response := s.HandleMessage(context.Background(), []byte(msg))
		if _, isErr := response.(mcp.JSONRPCError); !isErr {
			t.Fatalf("%s should not resolve to a bounded list resource, got %#v", uri, response)
		}
	}
}
