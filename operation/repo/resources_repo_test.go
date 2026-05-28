package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func setupRepoMockServer(t *testing.T, statusCode int, body interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func makeRepoResourceRequest(owner, repo string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/" + owner + "/" + repo,
		},
	}
}

func TestRepoResourceHandler_HappyPath(t *testing.T) {
	fakeRepo := map[string]interface{}{
		"id":        1,
		"name":      "forgejo-mcp",
		"full_name": "goern/forgejo-mcp",
		"owner": map[string]interface{}{
			"login": "goern",
		},
		"description":       "MCP server for Forgejo",
		"html_url":          "https://codeberg.org/goern/forgejo-mcp",
		"default_branch":    "main",
		"fork":              false,
		"archived":          false,
		"private":           false,
		"stars_count":       42,
		"forks_count":       7,
		"watchers_count":    15,
		"open_issues_count": 3,
		"open_pr_counter":   1,
		"size":              512,
		"has_issues":        true,
		"has_wiki":          false,
		"has_pull_requests": true,
	}
	srv := setupRepoMockServer(t, http.StatusOK, fakeRepo)
	defer srv.Close()

	req := makeRepoResourceRequest("goern", "forgejo-mcp")
	contents, err := repoResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}

	block, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("content must be TextResourceContents")
	}
	if block.MIMEType != "application/json" {
		t.Errorf("MIME type: got %q, want application/json", block.MIMEType)
	}

	var payload repoResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if payload.Name != "forgejo-mcp" {
		t.Errorf("expected name=forgejo-mcp, got %q", payload.Name)
	}
	if payload.StarsCount != 42 {
		t.Errorf("expected stars=42, got %d", payload.StarsCount)
	}
	if payload.HasIssues != true {
		t.Error("expected has_issues=true")
	}
}

func TestRepoResourceHandler_403(t *testing.T) {
	srv := setupRepoMockServer(t, http.StatusForbidden, map[string]string{"message": "Forbidden"})
	defer srv.Close()

	req := makeRepoResourceRequest("goern", "private-repo")
	_, err := repoResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if re, ok := err.(*resource.ResourceError); ok {
		if re.Code != -32002 {
			t.Errorf("expected code -32002, got %d", re.Code)
		}
	}
}

func TestRepoResourceHandler_404(t *testing.T) {
	srv := setupRepoMockServer(t, http.StatusNotFound, map[string]string{"message": "Not Found"})
	defer srv.Close()

	req := makeRepoResourceRequest("goern", "no-such-repo")
	_, err := repoResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if re, ok := err.(*resource.ResourceError); ok {
		if re.Code != -32003 {
			t.Errorf("expected code -32003, got %d", re.Code)
		}
	}
}
