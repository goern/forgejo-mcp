package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeStatusResourceRequest(sha string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/testowner/testrepo/commit/" + sha + "/status",
		},
	}
}

func setupStatusMockServer(t *testing.T, statusCode int, body interface{}) *httptest.Server {
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

func makeStatusList(states ...string) []map[string]interface{} {
	result := make([]map[string]interface{}, len(states))
	for i, s := range states {
		result[i] = map[string]interface{}{
			"id":          i + 1,
			"status":      s,
			"context":     fmt.Sprintf("ci/test-%d", i),
			"description": "test context " + s,
			"target_url":  "https://ci.example.com/" + s,
			"created_at":  time.Now().Format(time.RFC3339),
			"updated_at":  time.Now().Format(time.RFC3339),
		}
	}
	return result
}

func TestStatusResourceHandler_HappyPath_UnderCap(t *testing.T) {
	statuses := makeStatusList("success", "success", "pending")
	srv := setupStatusMockServer(t, http.StatusOK, statuses)
	defer srv.Close()

	req := makeStatusResourceRequest(testSHA)
	contents, err := statusResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}

	block, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("content block must be TextResourceContents")
	}
	if block.MIMEType != "application/json" {
		t.Errorf("MIME type: got %q, want application/json", block.MIMEType)
	}

	var payload statusResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if payload.State != "pending" {
		t.Errorf("expected aggregate state=pending (has one pending), got %q", payload.State)
	}
	if payload.TotalCount != 3 {
		t.Errorf("expected total_count=3, got %d", payload.TotalCount)
	}
	if payload.Truncated {
		t.Error("expected truncated=false for 3 statuses under cap")
	}
	if len(payload.Statuses) != 3 {
		t.Errorf("expected 3 status items, got %d", len(payload.Statuses))
	}
}

func TestStatusResourceHandler_OverCap_Truncated(t *testing.T) {
	states := make([]string, 35)
	for i := range states {
		states[i] = "success"
	}
	statuses := makeStatusList(states...)
	srv := setupStatusMockServer(t, http.StatusOK, statuses)
	defer srv.Close()

	req := makeStatusResourceRequest(testSHA)
	contents, err := statusResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	block := contents[0].(mcp.TextResourceContents)
	var payload statusResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !payload.Truncated {
		t.Error("expected truncated=true for 35 statuses")
	}
	if len(payload.Statuses) != 30 {
		t.Errorf("expected 30 capped statuses, got %d", len(payload.Statuses))
	}
	if payload.ListTool != "get_commit_statuses" {
		t.Errorf("expected list_tool=get_commit_statuses, got %q", payload.ListTool)
	}
	if payload.TotalCount != 35 {
		t.Errorf("expected total_count=35, got %d", payload.TotalCount)
	}
}

func TestStatusResourceHandler_EmptyStatuses_StateUnknown(t *testing.T) {
	srv := setupStatusMockServer(t, http.StatusOK, []map[string]interface{}{})
	defer srv.Close()

	req := makeStatusResourceRequest(testSHA)
	contents, err := statusResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	block := contents[0].(mcp.TextResourceContents)
	var payload statusResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.State != "unknown" {
		t.Errorf("expected state=unknown for empty statuses, got %q", payload.State)
	}
}

func TestStatusResourceHandler_ShortSHA(t *testing.T) {
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/testowner/testrepo/commit/abc123/status",
		},
	}
	_, err := statusResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for short sha")
	}
}

func TestStatusResourceHandler_NotFound(t *testing.T) {
	srv := setupStatusMockServer(t, http.StatusNotFound, map[string]string{"message": "not found"})
	defer srv.Close()

	req := makeStatusResourceRequest(testSHA)
	_, err := statusResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestComputeAggregateState_AllSuccess(t *testing.T) {
	s := forgejo_sdk.StatusSuccess
	statuses := []*forgejo_sdk.Status{{State: s}, {State: s}}
	if got := computeAggregateState(statuses); got != "success" {
		t.Errorf("expected success, got %q", got)
	}
}

func TestComputeAggregateState_AnyFailure(t *testing.T) {
	statuses := []*forgejo_sdk.Status{
		{State: forgejo_sdk.StatusSuccess},
		{State: forgejo_sdk.StatusFailure},
	}
	if got := computeAggregateState(statuses); got != "failure" {
		t.Errorf("expected failure, got %q", got)
	}
}
