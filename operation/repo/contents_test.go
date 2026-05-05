package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func setupContentsServer(t *testing.T, responseBody interface{}, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(responseBody)
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestListRepoContentsFn_ReturnsContents(t *testing.T) {
	fakeEntries := []map[string]interface{}{
		{"name": "README.md", "path": "README.md", "type": "file", "sha": "aaa111", "size": 42},
		{"name": "src", "path": "src", "type": "dir", "sha": "bbb222", "size": 0},
	}
	srv := setupContentsServer(t, fakeEntries, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner": "testowner",
		"repo":  "testrepo",
		"ref":   "main",
		"path":  "",
	})

	result, err := ListRepoContentsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListRepoContentsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ListRepoContentsFn returned tool error: %v", result.Content)
	}

	// Result should contain both entry names
	text := extractText(t, result)
	if !strings.Contains(text, "README.md") {
		t.Errorf("result missing README.md entry\n  got: %s", text)
	}
	if !strings.Contains(text, "src") {
		t.Errorf("result missing src entry\n  got: %s", text)
	}
}

func TestGetRepoTreeFn_ReturnsTree(t *testing.T) {
	fakeTree := map[string]interface{}{
		"sha": "abc123",
		"url": "http://example.com",
		"tree": []map[string]interface{}{
			{"path": "README.md", "mode": "100644", "type": "blob", "size": 42, "sha": "aaa111"},
			{"path": "src/main.go", "mode": "100644", "type": "blob", "size": 512, "sha": "ccc333"},
		},
		"truncated":   false,
		"page":        1,
		"total_count": 2,
	}
	srv := setupContentsServer(t, fakeTree, http.StatusOK)
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"owner":     "testowner",
		"repo":      "testrepo",
		"ref":       "main",
		"recursive": true,
	})

	result, err := GetRepoTreeFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetRepoTreeFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("GetRepoTreeFn returned tool error: %v", result.Content)
	}

	text := extractText(t, result)
	if !strings.Contains(text, "src/main.go") {
		t.Errorf("result missing src/main.go entry\n  got: %s", text)
	}
	if !strings.Contains(text, "README.md") {
		t.Errorf("result missing README.md entry\n  got: %s", text)
	}
}

// extractText pulls the string content from a CallToolResult.
func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("no text content in result")
	return ""
}
