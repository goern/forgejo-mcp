package pull

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// prRoutingHandler routes /pulls/{index}, /issues/{index}/comments, and /pulls/{index}/reviews.
type prRoutingHandler struct {
	prStatus       int
	prBody         interface{}
	commentsStatus int
	commentsBody   interface{}
	reviewsStatus  int
	reviewsBody    interface{}
}

func (h *prRoutingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case strings.Contains(path, "/reviews"):
		w.WriteHeader(h.reviewsStatus)
		if h.reviewsBody != nil {
			json.NewEncoder(w).Encode(h.reviewsBody)
		}
	case strings.Contains(path, "/issues/") && strings.Contains(path, "/comments"):
		w.WriteHeader(h.commentsStatus)
		if h.commentsBody != nil {
			json.NewEncoder(w).Encode(h.commentsBody)
		}
	case strings.Contains(path, "/pulls/"):
		w.WriteHeader(h.prStatus)
		if h.prBody != nil {
			json.NewEncoder(w).Encode(h.prBody)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func setupPRMockServer(t *testing.T, h *prRoutingHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func makePRResourceRequest(index int) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: fmt.Sprintf("forgejo://repo/testowner/testrepo/pr/%d", index),
		},
	}
}

func fakePR(merged bool) map[string]interface{} {
	now := time.Now().Format(time.RFC3339)
	pr := map[string]interface{}{
		"id":         1,
		"number":     7,
		"title":      "feat: add resources",
		"body":       "This PR adds MCP resource support.",
		"state":      "open",
		"merged":     merged,
		"mergeable":  true,
		"comments":   2,
		"user":       map[string]interface{}{"login": "alice"},
		"labels":     []interface{}{},
		"assignees":  []interface{}{},
		"created_at": now,
		"updated_at": now,
		"head": map[string]interface{}{
			"label": "alice:feature",
			"ref":   "feature",
			"sha":   "abc123",
		},
		"base": map[string]interface{}{
			"label": "main",
			"ref":   "main",
			"sha":   "def456",
		},
	}
	if merged {
		pr["state"] = "closed"
		pr["merged_at"] = now
	}
	return pr
}

func fakeReviews(n int) []map[string]interface{} {
	result := make([]map[string]interface{}, n)
	for i := range result {
		result[i] = map[string]interface{}{
			"id":           i + 1,
			"state":        "approved",
			"body":         fmt.Sprintf("review body %d", i+1),
			"user":         map[string]interface{}{"login": "reviewer"},
			"submitted_at": time.Now().Format(time.RFC3339),
		}
	}
	return result
}

func TestPRResourceHandler_OpenPR_HappyPath(t *testing.T) {
	h := &prRoutingHandler{
		prStatus:       http.StatusOK,
		prBody:         fakePR(false),
		commentsStatus: http.StatusOK,
		commentsBody:   fakeReviews(2), // reuse shape; comments just need id/body/user/created_at
		reviewsStatus:  http.StatusOK,
		reviewsBody:    fakeReviews(1),
	}
	srv := setupPRMockServer(t, h)
	defer srv.Close()

	req := makePRResourceRequest(7)
	contents, err := prResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(contents))
	}

	block := contents[0].(mcp.TextResourceContents)
	if block.MIMEType != "application/json" {
		t.Errorf("MIME: got %q, want application/json", block.MIMEType)
	}
	var payload prResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.State != "open" {
		t.Errorf("expected state=open, got %q", payload.State)
	}
	if !payload.Mergeable {
		t.Error("expected mergeable=true")
	}
	if payload.CommentsTruncated {
		t.Error("expected comments not truncated")
	}

	mdBlock := contents[1].(mcp.TextResourceContents)
	if mdBlock.MIMEType != "text/markdown" {
		t.Errorf("sidecar MIME: got %q, want text/markdown", mdBlock.MIMEType)
	}
	if !strings.Contains(mdBlock.Text, "feat: add resources") {
		t.Errorf("sidecar missing title, got %q", mdBlock.Text)
	}
}

func TestPRResourceHandler_MergedPR(t *testing.T) {
	h := &prRoutingHandler{
		prStatus:       http.StatusOK,
		prBody:         fakePR(true),
		commentsStatus: http.StatusOK,
		commentsBody:   []interface{}{},
		reviewsStatus:  http.StatusOK,
		reviewsBody:    []interface{}{},
	}
	srv := setupPRMockServer(t, h)
	defer srv.Close()

	req := makePRResourceRequest(7)
	contents, err := prResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload prResourcePayload
	json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload)
	if payload.State != "merged" {
		t.Errorf("expected state=merged, got %q", payload.State)
	}
	if payload.MergedAt == "" {
		t.Error("expected merged_at to be set")
	}
}

func TestPRResourceHandler_OverCapComments(t *testing.T) {
	comments := make([]map[string]interface{}, resource.EmbeddedListCap+1)
	for i := range comments {
		comments[i] = map[string]interface{}{
			"id":         i + 1,
			"body":       "comment",
			"user":       map[string]interface{}{"login": "bob"},
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		}
	}
	h := &prRoutingHandler{
		prStatus:       http.StatusOK,
		prBody:         fakePR(false),
		commentsStatus: http.StatusOK,
		commentsBody:   comments,
		reviewsStatus:  http.StatusOK,
		reviewsBody:    []interface{}{},
	}
	srv := setupPRMockServer(t, h)
	defer srv.Close()

	req := makePRResourceRequest(7)
	contents, err := prResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload prResourcePayload
	json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload)
	if !payload.CommentsTruncated {
		t.Error("expected comments_truncated=true")
	}
	if payload.CommentsListTool != "list_issue_comments" {
		t.Errorf("expected comments_list_tool=list_issue_comments, got %q", payload.CommentsListTool)
	}
	if len(payload.RecentComments) != resource.EmbeddedListCap {
		t.Errorf("expected %d capped comments, got %d", resource.EmbeddedListCap, len(payload.RecentComments))
	}
}

func TestPRResourceHandler_OverCapReviews(t *testing.T) {
	h := &prRoutingHandler{
		prStatus:       http.StatusOK,
		prBody:         fakePR(false),
		commentsStatus: http.StatusOK,
		commentsBody:   []interface{}{},
		reviewsStatus:  http.StatusOK,
		reviewsBody:    fakeReviews(35),
	}
	srv := setupPRMockServer(t, h)
	defer srv.Close()

	req := makePRResourceRequest(7)
	contents, err := prResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload prResourcePayload
	json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload)
	if !payload.ReviewsTruncated {
		t.Error("expected reviews_truncated=true")
	}
	if payload.ReviewsListTool != "list_pull_reviews" {
		t.Errorf("expected reviews_list_tool=list_pull_reviews, got %q", payload.ReviewsListTool)
	}
	if len(payload.RecentReviews) != 30 {
		t.Errorf("expected 30 capped reviews, got %d", len(payload.RecentReviews))
	}
}

func TestPRResourceHandler_NonNumericIndex(t *testing.T) {
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/testowner/testrepo/pr/abc",
		},
	}
	_, err := prResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-numeric index")
	}
	if re, ok := err.(*resource.ResourceError); ok && re.Code != -32602 {
		t.Errorf("expected -32602, got %d", re.Code)
	}
}

func TestPRResourceHandler_NotFound(t *testing.T) {
	h := &prRoutingHandler{
		prStatus: http.StatusNotFound,
		prBody:   map[string]string{"message": "not found"},
	}
	srv := setupPRMockServer(t, h)
	defer srv.Close()

	req := makePRResourceRequest(999)
	_, err := prResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if re, ok := err.(*resource.ResourceError); ok && re.Code != -32003 {
		t.Errorf("expected -32003, got %d", re.Code)
	}
}
