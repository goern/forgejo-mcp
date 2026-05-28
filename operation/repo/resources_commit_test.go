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

const testSHA = "a3f1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"

func setupCommitMockServer(t *testing.T, statusCode int, body interface{}) *httptest.Server {
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

func makeCommitResourceRequest(sha string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/testowner/testrepo/commit/" + sha,
		},
	}
}

func TestCommitResourceHandler_HappyPath(t *testing.T) {
	fakeCommit := map[string]interface{}{
		"sha":      testSHA,
		"html_url": "https://codeberg.org/testowner/testrepo/commit/" + testSHA,
		"commit": map[string]interface{}{
			"message": "fix: resolve race condition\n\nDetailed explanation.",
			"author":  map[string]interface{}{"name": "Alice", "email": "alice@example.com", "date": "2024-01-01"},
		},
	}
	srv := setupCommitMockServer(t, http.StatusOK, fakeCommit)
	defer srv.Close()

	req := makeCommitResourceRequest(testSHA)
	contents, err := commitResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) < 2 {
		t.Fatalf("expected at least 2 content blocks (JSON + markdown), got %d", len(contents))
	}

	jsonBlock, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("first block must be TextResourceContents")
	}
	if jsonBlock.MIMEType != "application/json" {
		t.Errorf("first block MIME type: got %q, want application/json", jsonBlock.MIMEType)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(jsonBlock.Text), &decoded); err != nil {
		t.Errorf("first block is not valid JSON: %v", err)
	}

	mdBlock, ok := contents[1].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("second block must be TextResourceContents")
	}
	if mdBlock.MIMEType != "text/markdown" {
		t.Errorf("second block MIME type: got %q, want text/markdown", mdBlock.MIMEType)
	}
	if !strings.Contains(mdBlock.Text, "race condition") {
		t.Errorf("markdown sidecar missing commit message, got %q", mdBlock.Text)
	}
}

func TestCommitResourceHandler_ShortSHA(t *testing.T) {
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/testowner/testrepo/commit/abc123",
		},
	}
	_, err := commitResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for short sha")
	}
}

func TestCommitResourceHandler_NotFound(t *testing.T) {
	srv := setupCommitMockServer(t, http.StatusNotFound, map[string]string{"message": "commit not found"})
	defer srv.Close()

	req := makeCommitResourceRequest(testSHA)
	_, err := commitResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestCommitResourceHandler_SidecarPresent(t *testing.T) {
	fakeCommit := map[string]interface{}{
		"sha": testSHA,
		"commit": map[string]interface{}{
			"message": "chore: update deps",
		},
	}
	srv := setupCommitMockServer(t, http.StatusOK, fakeCommit)
	defer srv.Close()

	req := makeCommitResourceRequest(testSHA)
	contents, err := commitResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, c := range contents {
		if tc, ok := c.(mcp.TextResourceContents); ok && tc.MIMEType == "text/markdown" {
			found = true
		}
	}
	if !found {
		t.Error("expected text/markdown sidecar block to be present")
	}
}
