package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"

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
	return serveStatusHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	})
}

func serveStatusHandler(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	oldURL, oldToken := flag.URL, flag.Token
	t.Cleanup(func() { flag.URL, flag.Token = oldURL, oldToken })
	flag.URL, flag.Token = srv.URL, "tkn"
	return srv
}

// combinedStatus is the /commits/{sha}/status envelope. Its total_count is the
// page length, as on a real Forgejo; the true count is in X-Total-Count.
func combinedStatus(statuses []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"state": "", "total_count": len(statuses), "statuses": statuses}
}

func readStatusPayload(t *testing.T, sha string) statusResourcePayload {
	t.Helper()
	contents, err := statusResourceHandler(context.Background(), makeStatusResourceRequest(sha))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload statusResourcePayload
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return payload
}

// The PaC sequence: a run posts pending then success under one context. Both
// rows stay in /statuses forever; only /status drops the superseded one.
func TestStatusResourceHandler_SupersededPendingIsNotCounted(t *testing.T) {
	const statusPath = "/api/v1/repos/testowner/testrepo/commits/" + testSHA
	srv := serveStatusHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case statusPath + "/status":
			if got := r.URL.Query().Get("limit"); got != fmt.Sprint(resource.EmbeddedListCap+1) {
				t.Errorf("limit = %q, want cap+1", got)
			}
			json.NewEncoder(w).Encode(combinedStatus(makeStatusList("success")))
		case statusPath + "/statuses":
			json.NewEncoder(w).Encode(makeStatusList("pending", "success"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer srv.Close()

	payload := readStatusPayload(t, testSHA)

	if payload.State != "success" {
		t.Errorf("state = %q, want success", payload.State)
	}
	if payload.TotalCount != 1 || len(payload.Statuses) != 1 {
		t.Errorf("total_count = %d with %d rows, want 1 and 1", payload.TotalCount, len(payload.Statuses))
	}
}

func TestStatusResourceHandler_TruncatedAllGreenIsNotSuccess(t *testing.T) {
	states := make([]string, resource.EmbeddedListCap+1)
	for i := range states {
		states[i] = "success"
	}
	srv := setupStatusMockServer(t, http.StatusOK, combinedStatus(makeStatusList(states...)))
	defer srv.Close()

	if got := readStatusPayload(t, testSHA).State; got != "unknown" {
		t.Errorf("state = %q, want unknown: unseen contexts may have failed", got)
	}
}

func TestStatusResourceHandler_TruncatedFailureStands(t *testing.T) {
	states := make([]string, resource.EmbeddedListCap+1)
	for i := range states {
		states[i] = "success"
	}
	states[0] = "failure"
	srv := setupStatusMockServer(t, http.StatusOK, combinedStatus(makeStatusList(states...)))
	defer srv.Close()

	if got := readStatusPayload(t, testSHA).State; got != "failure" {
		t.Errorf("state = %q, want failure", got)
	}
}

// An instance whose page size is below our limit returns fewer rows than exist,
// so only the header shows the truncation.
func TestStatusResourceHandler_HeaderTotalBeyondPageTruncates(t *testing.T) {
	srv := serveStatusHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "40")
		json.NewEncoder(w).Encode(combinedStatus(makeStatusList("success", "success")))
	})
	defer srv.Close()

	payload := readStatusPayload(t, testSHA)

	if payload.TotalCount != 40 {
		t.Errorf("total_count = %d, want 40", payload.TotalCount)
	}
	if !payload.Truncated || payload.ListTool != "get_commit_statuses" {
		t.Errorf("truncated = %v, list_tool = %q", payload.Truncated, payload.ListTool)
	}
	if payload.State != "unknown" {
		t.Errorf("state = %q, want unknown", payload.State)
	}
}

func TestStatusResourceHandler_EmptyBodyIsUnknown(t *testing.T) {
	srv := setupStatusMockServer(t, http.StatusOK, nil)
	defer srv.Close()

	if got := readStatusPayload(t, testSHA).State; got != "unknown" {
		t.Errorf("state = %q, want unknown", got)
	}
}

func TestStatusResourceHandler_ServerErrorIgnoresDigitsInSHA(t *testing.T) {
	const shaWith403 = "403403403403403403403403403403403403403a"
	srv := setupStatusMockServer(t, http.StatusInternalServerError, map[string]string{"message": "boom"})
	defer srv.Close()

	_, err := statusResourceHandler(context.Background(), makeStatusResourceRequest(shaWith403))

	var re *resource.ResourceError
	if !errors.As(err, &re) || re.Code != -32603 {
		t.Fatalf("err = %v, want a -32603 ResourceError", err)
	}
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
	srv := setupStatusMockServer(t, http.StatusOK, combinedStatus(statuses))
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
	// Use exactly EmbeddedListCap+1 items: the handler asks for limit=cap+1, so
	// that is the most a real server returns when there are more than cap
	// contexts. The mock does not enforce the limit itself.
	capPlus1 := resource.EmbeddedListCap + 1
	states := make([]string, capPlus1)
	for i := range states {
		states[i] = "success"
	}
	statuses := makeStatusList(states...)
	srv := setupStatusMockServer(t, http.StatusOK, combinedStatus(statuses))
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
		t.Errorf("expected truncated=true for %d statuses (EmbeddedListCap+1)", capPlus1)
	}
	if len(payload.Statuses) != resource.EmbeddedListCap {
		t.Errorf("expected %d capped statuses, got %d", resource.EmbeddedListCap, len(payload.Statuses))
	}
	if payload.ListTool != "get_commit_statuses" {
		t.Errorf("expected list_tool=get_commit_statuses, got %q", payload.ListTool)
	}
	if payload.TotalCount != capPlus1 {
		t.Errorf("expected total_count=%d, got %d", capPlus1, payload.TotalCount)
	}
}

func TestStatusResourceHandler_EmptyStatuses_StateUnknown(t *testing.T) {
	// Forgejo's answer for a commit with no statuses: 200, null list, empty state.
	srv := setupStatusMockServer(t, http.StatusOK, map[string]interface{}{"state": "", "total_count": 0, "statuses": nil})
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

func TestComputeAggregateState_NilOnlyIsUnknown(t *testing.T) {
	if got := computeAggregateState([]*forgejo_sdk.Status{nil, nil}); got != "unknown" {
		t.Errorf("got %q, want unknown", got)
	}
}

func TestToStatusItem_Nil(t *testing.T) {
	item, ok := toStatusItem(nil)
	if ok {
		t.Fatalf("nil must be skipped, got %+v", item)
	}
	if item != (statusItem{}) {
		t.Errorf("zero item: got %+v", item)
	}
}

func TestToStatusItem_MapsFields(t *testing.T) {
	created := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	got, ok := toStatusItem(&forgejo_sdk.Status{
		Context:     "ci/woodpecker",
		State:       forgejo_sdk.StatusSuccess,
		TargetURL:   "https://ci.example/1",
		Description: "ok",
		Created:     created,
	})
	if !ok {
		t.Fatal("expected ok")
	}
	if got.Context != "ci/woodpecker" || got.State != "success" || got.TargetURL != "https://ci.example/1" || got.Description != "ok" {
		t.Errorf("item: %+v", got)
	}
	if got.CreatedAt != "2026-08-01T12:00:00Z" {
		t.Errorf("created_at: got %q", got.CreatedAt)
	}
}
