package issue

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

type recordedReq struct {
	method  string
	path    string
	rawBody []byte
}

func newPatchBackend(t *testing.T, respBody string) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 2)
	// Serve the SDK's startup version probe so NewClient succeeds.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			rawBody: body,
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "tkn"
	flag.UserAgent = "test"

	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("failed to build SDK client for test: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return srv, &records
}

func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func TestUpdateIssue_AssigneeSingular(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42}`)

	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":    "goern",
		"repo":     "forgejo-mcp",
		"index":    float64(42),
		"assignee": "goern",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("UpdateIssueFn returned error: err=%v res=%+v", err, res)
	}

	if len(*records) == 0 {
		t.Fatal("expected at least one HTTP request to backend")
	}
	last := (*records)[len(*records)-1]
	if last.method != http.MethodPatch {
		t.Fatalf("expected PATCH, got %s", last.method)
	}
	if !strings.Contains(last.path, "/issues/42") {
		t.Fatalf("unexpected path: %s", last.path)
	}

	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	assignees, ok := payload["assignees"].([]any)
	if !ok {
		t.Fatalf("assignees field missing or wrong type: %T %v", payload["assignees"], payload["assignees"])
	}
	if len(assignees) != 1 || assignees[0] != "goern" {
		t.Fatalf("expected assignees=[goern], got %v", assignees)
	}
}

func TestUpdateIssue_AssigneesCSV(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42}`)

	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":     "goern",
		"repo":      "forgejo-mcp",
		"index":     float64(42),
		"assignees": "alice, bob ,carol",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("UpdateIssueFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	assignees, _ := payload["assignees"].([]any)
	want := []string{"alice", "bob", "carol"}
	if len(assignees) != len(want) {
		t.Fatalf("len mismatch: got %v want %v", assignees, want)
	}
	for i, v := range want {
		if assignees[i] != v {
			t.Fatalf("assignees[%d]: got %v want %s", i, assignees[i], v)
		}
	}
}

func TestUpdateIssue_AssigneesEmptyClears(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42}`)

	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":     "goern",
		"repo":      "forgejo-mcp",
		"index":     float64(42),
		"assignees": "",
		"assignee":  "ignored-because-assignees-wins",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("UpdateIssueFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	if !strings.Contains(string(last.rawBody), `"assignees":[]`) {
		t.Fatalf("expected empty assignees array in body, got: %s", last.rawBody)
	}
}

func TestUpdateIssue_NoAssigneeFieldsOmitsAssignees(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42}`)

	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
		"title": "new title",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("UpdateIssueFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	if v, ok := payload["assignees"]; ok && v != nil {
		// Acceptable for SDK to emit "assignees":null since Assignees is a slice; reject only non-null arrays.
		if arr, isArr := v.([]any); isArr && len(arr) > 0 {
			t.Fatalf("expected no assignees set, got %v", v)
		}
	}
}
