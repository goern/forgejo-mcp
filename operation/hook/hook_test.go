// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

func newHookCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func setupHookMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func hookToolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	content, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatal("not text content")
	}
	return content.Text
}

func TestListRepoHooksFn_CountAndPageEcho(t *testing.T) {
	setupHookMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/hooks") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "type": "forgejo", "active": true, "config": map[string]string{"url": "https://example.test/hook"}},
		})
	})

	res, err := ListRepoHooksFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "page": float64(1), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	text := hookToolText(t, res)
	if !strings.Contains(text, `"count":1`) || !strings.Contains(text, `"page":1`) {
		t.Fatalf("expected count+page echo in result, got: %s", text)
	}
}

func TestListRepoHooksFn_TotalCountFromHeader(t *testing.T) {
	setupHookMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "4")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "type": "forgejo", "active": true, "config": map[string]string{}},
		})
	})

	res, err := ListRepoHooksFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "page": float64(1), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); !strings.Contains(text, `"total_count":4`) {
		t.Fatalf("expected total_count from X-Total-Count header, got: %s", text)
	}
}

// Confirmed-zero and unknown are different answers: X-Total-Count: 0 must
// marshal as "total_count":0 rather than being swallowed by `omitempty`,
// while a missing header drops the key (the sibling test below).
func TestListRepoHooksFn_TotalCountZeroIsEmittedNotOmitted(t *testing.T) {
	setupHookMockServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "0")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	res, err := ListRepoHooksFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "page": float64(1), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); !strings.Contains(text, `"total_count":0`) {
		t.Fatalf("expected a confirmed-zero total_count to be emitted, got: %s", text)
	}
}

func TestListRepoHooksFn_TotalCountOmittedWhenHeaderAbsent(t *testing.T) {
	setupHookMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "type": "forgejo", "active": true, "config": map[string]string{}},
		})
	})

	res, err := ListRepoHooksFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "page": float64(1), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); strings.Contains(text, "total_count") {
		t.Fatalf("expected total_count to be omitted, got: %s", text)
	}
}

func TestListRepoHooksFn_TotalCountOmittedWhenHeaderUnparsable(t *testing.T) {
	setupHookMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "garbage")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "type": "forgejo", "active": true, "config": map[string]string{}},
		})
	})

	res, err := ListRepoHooksFn(context.Background(), newHookCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "page": float64(1), "limit": float64(30),
	}))
	if err != nil {
		t.Fatalf("ListRepoHooksFn err: %v", err)
	}
	if text := hookToolText(t, res); strings.Contains(text, "total_count") {
		t.Fatalf("expected total_count to be omitted for a garbage header, got: %s", text)
	}
}
