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
	query   string
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
			query:   r.URL.RawQuery,
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

// newLabelsBackend serves /api/v1/repos/{owner}/{repo}/labels and
// /api/v1/orgs/{owner}/labels with caller-supplied status codes and bodies.
// Caller passes maps keyed by exact path. Status defaults to 200 if 0.
func newLabelsBackend(
	t *testing.T,
	repoLabelsBody string, repoStatus int,
	orgLabelsBody string, orgStatus int,
) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/repos/") && strings.HasSuffix(r.URL.Path, "/labels"):
			if repoStatus == 0 {
				repoStatus = http.StatusOK
			}
			w.WriteHeader(repoStatus)
			_, _ = w.Write([]byte(repoLabelsBody))
		case strings.HasPrefix(r.URL.Path, "/api/v1/orgs/") && strings.HasSuffix(r.URL.Path, "/labels"):
			if orgStatus == 0 {
				orgStatus = http.StatusOK
			}
			w.WriteHeader(orgStatus)
			_, _ = w.Write([]byte(orgLabelsBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

func TestListOrgLabels_Success(t *testing.T) {
	_, records := newLabelsBackend(t,
		"", 0,
		`[{"id":1,"name":"bug","color":"ff0000","description":"a bug","url":""},{"id":2,"name":"enh","color":"00ff00","description":"","url":""}]`, http.StatusOK,
	)
	res, err := ListOrgLabelsFn(context.Background(), makeReq(map[string]any{"org": "codeberg"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListOrgLabelsFn err: %v res=%+v", err, res)
	}
	if len(*records) == 0 {
		t.Fatal("expected request to backend")
	}
	last := (*records)[len(*records)-1]
	if last.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", last.method)
	}
	if !strings.HasPrefix(last.path, "/api/v1/orgs/codeberg/labels") {
		t.Fatalf("unexpected path: %s", last.path)
	}
	// Result body should serialize each entry with scope=org.
	if !strings.Contains(textOf(res), `"scope":"org"`) {
		t.Fatalf("expected scope=org in result, got %q", textOf(res))
	}
}

func TestListOrgLabels_404IsEmpty(t *testing.T) {
	_, _ = newLabelsBackend(t, "", 0, ``, http.StatusNotFound)
	res, err := ListOrgLabelsFn(context.Background(), makeReq(map[string]any{"org": "ghost"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected success on 404, got err=%v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), "[]") && !strings.Contains(textOf(res), "null") {
		t.Fatalf("expected empty result for 404, got %q", textOf(res))
	}
}

func TestListOrgLabels_UnauthorizedSurfaces(t *testing.T) {
	_, _ = newLabelsBackend(t, "", 0, ``, http.StatusUnauthorized)
	_, err := ListOrgLabelsFn(context.Background(), makeReq(map[string]any{"org": "codeberg"}))
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}

func TestListRepoLabels_MergeWithOrgLabels(t *testing.T) {
	_, _ = newLabelsBackend(t,
		`[{"id":10,"name":"good-first-issue","color":"7057ff","description":"","url":""}]`, http.StatusOK,
		`[{"id":20,"name":"security","color":"d73a4a","description":"","url":""}]`, http.StatusOK,
	)
	res, err := ListRepoLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "codeberg-org", "repo": "project",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoLabelsFn err: %v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"scope":"repo"`) || !strings.Contains(out, `"scope":"org"`) {
		t.Fatalf("expected both scopes in result, got %q", out)
	}
	if !strings.Contains(out, `"good-first-issue"`) || !strings.Contains(out, `"security"`) {
		t.Fatalf("expected both label names in merged result, got %q", out)
	}
}

func TestListRepoLabels_IncludeOrgFalseSkipsOrgCall(t *testing.T) {
	_, records := newLabelsBackend(t,
		`[{"id":10,"name":"good-first-issue","color":"7057ff","description":"","url":""}]`, http.StatusOK,
		`[{"id":20,"name":"security","color":"d73a4a","description":"","url":""}]`, http.StatusOK,
	)
	res, err := ListRepoLabelsFn(context.Background(), makeReq(map[string]any{
		"owner":              "codeberg-org",
		"repo":               "project",
		"include_org_labels": false,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoLabelsFn err: %v res=%+v", err, res)
	}
	for _, r := range *records {
		if strings.HasPrefix(r.path, "/api/v1/orgs/") {
			t.Fatalf("did not expect org-labels request, got %s %s", r.method, r.path)
		}
	}
	out := textOf(res)
	if strings.Contains(out, `"scope":"org"`) || strings.Contains(out, `"security"`) {
		t.Fatalf("expected no org-scoped labels in result, got %q", out)
	}
}

func TestListRepoLabels_UserOwnerOrgEndpoint404(t *testing.T) {
	_, _ = newLabelsBackend(t,
		`[{"id":10,"name":"bug","color":"ff0000","description":"","url":""}]`, http.StatusOK,
		``, http.StatusNotFound,
	)
	res, err := ListRepoLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "alice", "repo": "project",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected success when org endpoint 404s, got err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"scope":"repo"`) {
		t.Fatalf("expected repo-scoped label in result, got %q", out)
	}
	if strings.Contains(out, `"scope":"org"`) {
		t.Fatalf("expected no org-scoped labels when org endpoint 404s, got %q", out)
	}
}

// textOf flattens a CallToolResult into a single string for substring assertions.
func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
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
