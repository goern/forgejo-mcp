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

// These tests exist because the label templates were registered as bare paths
// while their descriptions, the README table and the AGENTS.md table all
// advertised the {?page,limit} form. mcp-go matches a read against an anchored
// regexp built from the registered string, so every query-bearing read failed
// with "resource not found" before any handler ran — page and limit were
// unreachable, and so, in turn, was the paging defect underneath them.
//
// The handler tests in operation/issue cannot see this: they build their own
// ReadResourceRequest and never cross the matcher. Only a real resources/read
// envelope through server.HandleMessage does.

// labelListBackend serves both label endpoints, honouring page and limit so a
// query-bearing URI has an observable effect rather than merely being accepted.
func labelListBackend(t *testing.T, total int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/labels") {
			http.NotFound(w, r)
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if size < 1 {
			size = 10
		}
		offset := (page - 1) * size

		rows := make([]any, 0, size)
		for i := offset + 1; i <= offset+size && i <= total; i++ {
			rows = append(rows, map[string]any{
				"id":          i,
				"name":        fmt.Sprintf("label-%d", i),
				"color":       "aabbcc",
				"description": "",
				"url":         fmt.Sprintf("https://example.test/labels/%d", i),
			})
		}
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		if offset+len(rows) < total {
			w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d&limit=%d>; rel="next"`, r.URL.Path, page+1, size))
		}
		_ = json.NewEncoder(w).Encode(rows)
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

// firstLabelID reports the id of the first row, so a paged read can be shown to
// start where it should rather than merely returning the right number of rows.
func firstLabelID(t *testing.T, payload map[string]any) int {
	t.Helper()
	rows, ok := payload["labels"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("payload has no label rows: %#v", payload)
	}
	row, _ := rows[0].(map[string]any)
	id, _ := row["id"].(float64)
	return int(id)
}

func TestResourceDispatchRoutesRepoLabelsTemplate(t *testing.T) {
	labelListBackend(t, 100)
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)

	t.Run("bare URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/labels")
		if got := rowCount(t, payload, "labels"); got != 30 {
			t.Fatalf("expected the default cap of 30 rows, got %d", got)
		}
	})

	t.Run("query-bearing URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/labels?limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("limit=5 did not take effect through dispatch: got %d rows", got)
		}
	})

	t.Run("query-bearing URI with page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/labels?page=2&limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("expected 5 rows on page 2, got %d", got)
		}
		// Page 2 at limit 5 must start after page 1's last row — the whole
		// point of the paging fix.
		if got := firstLabelID(t, payload); got != 6 {
			t.Fatalf("page 2 did not start at row 6, got %d", got)
		}
	})
}

func TestResourceDispatchRoutesOrgLabelsTemplate(t *testing.T) {
	labelListBackend(t, 100)
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)

	t.Run("bare URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://org/o/labels")
		if got := rowCount(t, payload, "labels"); got != 30 {
			t.Fatalf("expected the default cap of 30 rows, got %d", got)
		}
	})

	t.Run("query-bearing URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://org/o/labels?limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("limit=5 did not take effect through dispatch: got %d rows", got)
		}
	})

	t.Run("query-bearing URI with page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://org/o/labels?page=2&limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("expected 5 rows on page 2, got %d", got)
		}
		if got := firstLabelID(t, payload); got != 6 {
			t.Fatalf("page 2 did not start at row 6, got %d", got)
		}
	})
}

// TestResourceDispatchRejectsUnknownLabelListURI keeps the query expansion from
// widening the match: adding {?…} must not turn the template into something
// that swallows neighbouring paths. Without this, "register everything as a
// prefix" would pass the tests above.
func TestResourceDispatchRejectsUnknownLabelListURI(t *testing.T) {
	labelListBackend(t, 10)
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s)

	for _, uri := range []string{
		"forgejo://repo/o/r/labels/extra",
		"forgejo://org/o/labels/extra",
	} {
		msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":%q}}`, uri)
		response := s.HandleMessage(context.Background(), []byte(msg))
		if _, isErr := response.(mcp.JSONRPCError); !isErr {
			t.Fatalf("%s should not resolve to a bounded label list resource, got %#v", uri, response)
		}
	}
}
