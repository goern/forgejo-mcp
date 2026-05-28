package issue

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

// issueRoutingHandler routes /issues/{index} and /issues/{index}/comments to
// separate canned responses, plus /issues/comments/{id} for GetIssueComment.
type issueRoutingHandler struct {
	issueStatus    int
	issueBody      interface{}
	commentsStatus int
	commentsBody   interface{}
	commentStatus  int
	commentBody    interface{}
}

func (h *issueRoutingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case strings.Contains(path, "/issues/comments/"):
		w.WriteHeader(h.commentStatus)
		if h.commentBody != nil {
			json.NewEncoder(w).Encode(h.commentBody)
		}
	case strings.Contains(path, "/comments"):
		w.WriteHeader(h.commentsStatus)
		if h.commentsBody != nil {
			json.NewEncoder(w).Encode(h.commentsBody)
		}
	case strings.Contains(path, "/issues/"):
		w.WriteHeader(h.issueStatus)
		if h.issueBody != nil {
			json.NewEncoder(w).Encode(h.issueBody)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func setupIssueMockServer(t *testing.T, h *issueRoutingHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func makeIssueResourceRequest(owner, repo string, index int) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: fmt.Sprintf("forgejo://repo/%s/%s/issue/%d", owner, repo, index),
		},
	}
}

func makeCommentResourceRequest(owner, repo, kind string, index, id int) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: fmt.Sprintf("forgejo://repo/%s/%s/%s/%d/comment/%d", owner, repo, kind, index, id),
		},
	}
}

func fakeIssue() map[string]interface{} {
	return map[string]interface{}{
		"id":     1,
		"number": 42,
		"title":  "Fix the bug",
		"body":   "There is a bug that needs fixing.",
		"state":  "open",
		"user": map[string]interface{}{
			"login": "alice",
		},
		"labels":     []interface{}{},
		"assignees":  []interface{}{},
		"comments":   3,
		"created_at": time.Now().Format(time.RFC3339),
		"updated_at": time.Now().Format(time.RFC3339),
	}
}

func fakeComments(n int) []map[string]interface{} {
	result := make([]map[string]interface{}, n)
	for i := range result {
		result[i] = map[string]interface{}{
			"id":         i + 1,
			"body":       fmt.Sprintf("comment body %d", i+1),
			"user":       map[string]interface{}{"login": "bob"},
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		}
	}
	return result
}

func TestIssueResourceHandler_HappyPath(t *testing.T) {
	h := &issueRoutingHandler{
		issueStatus:    http.StatusOK,
		issueBody:      fakeIssue(),
		commentsStatus: http.StatusOK,
		commentsBody:   fakeComments(3),
	}
	srv := setupIssueMockServer(t, h)
	defer srv.Close()

	req := makeIssueResourceRequest("goern", "forgejo-mcp", 42)
	contents, err := issueResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 content blocks (JSON + markdown), got %d", len(contents))
	}

	jsonBlock := contents[0].(mcp.TextResourceContents)
	if jsonBlock.MIMEType != "application/json" {
		t.Errorf("first block MIME: got %q, want application/json", jsonBlock.MIMEType)
	}
	var payload issueResourcePayload
	if err := json.Unmarshal([]byte(jsonBlock.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.Title != "Fix the bug" {
		t.Errorf("expected title='Fix the bug', got %q", payload.Title)
	}
	if payload.Truncated {
		t.Error("expected truncated=false for 3 comments")
	}

	mdBlock := contents[1].(mcp.TextResourceContents)
	if mdBlock.MIMEType != "text/markdown" {
		t.Errorf("second block MIME: got %q, want text/markdown", mdBlock.MIMEType)
	}
	if !strings.Contains(mdBlock.Text, "Fix the bug") {
		t.Errorf("markdown sidecar missing title, got %q", mdBlock.Text)
	}
}

func TestIssueResourceHandler_OverCap_Truncated(t *testing.T) {
	h := &issueRoutingHandler{
		issueStatus:    http.StatusOK,
		issueBody:      fakeIssue(),
		commentsStatus: http.StatusOK,
		commentsBody:   fakeComments(35),
	}
	srv := setupIssueMockServer(t, h)
	defer srv.Close()

	req := makeIssueResourceRequest("goern", "forgejo-mcp", 42)
	contents, err := issueResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload issueResourcePayload
	json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload)
	if !payload.Truncated {
		t.Error("expected truncated=true for 35 comments")
	}
	if len(payload.RecentComments) != 30 {
		t.Errorf("expected 30 recent comments, got %d", len(payload.RecentComments))
	}
	if payload.ListTool != "list_issue_comments" {
		t.Errorf("expected list_tool=list_issue_comments, got %q", payload.ListTool)
	}
}

func TestIssueResourceHandler_404(t *testing.T) {
	h := &issueRoutingHandler{
		issueStatus: http.StatusNotFound,
		issueBody:   map[string]string{"message": "not found"},
	}
	srv := setupIssueMockServer(t, h)
	defer srv.Close()

	req := makeIssueResourceRequest("goern", "forgejo-mcp", 99)
	_, err := issueResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if re, ok := err.(*resource.ResourceError); ok && re.Code != -32003 {
		t.Errorf("expected -32003, got %d", re.Code)
	}
}

func TestIssueResourceHandler_NonNumericIndex(t *testing.T) {
	// non-numeric index is caught at parse time → -32603 (internal) because
	// "index must be numeric" maps to -32602 via MapForgejoError
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/goern/forgejo-mcp/issue/abc",
		},
	}
	_, err := issueResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-numeric index")
	}
	if re, ok := err.(*resource.ResourceError); ok {
		if re.Code != -32602 {
			t.Errorf("expected -32602 for non-numeric index, got %d", re.Code)
		}
	}
}

func TestCommentResourceHandler_UnknownKind(t *testing.T) {
	req := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/goern/forgejo-mcp/wiki/1/comment/5",
		},
	}
	_, err := commentResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if re, ok := err.(*resource.ResourceError); ok {
		if re.Code != -32602 {
			t.Errorf("expected -32602 for unknown kind, got %d", re.Code)
		}
	}
}

func TestCommentResourceHandler_NotFound(t *testing.T) {
	h := &issueRoutingHandler{
		commentStatus: http.StatusNotFound,
		commentBody:   map[string]string{"message": "comment not found"},
	}
	srv := setupIssueMockServer(t, h)
	defer srv.Close()

	req := makeCommentResourceRequest("goern", "forgejo-mcp", "issue", 42, 999)
	_, err := commentResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404 comment")
	}
	if re, ok := err.(*resource.ResourceError); ok && re.Code != -32003 {
		t.Errorf("expected -32003, got %d", re.Code)
	}
}
