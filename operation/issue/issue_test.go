package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"

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

// newQueryBackend serves /api/v1/version for SDK startup and dispatches every
// other request to handler, recording each request's method/path/query. Unlike
// newPatchBackend, the caller's handler can inspect the query (e.g. page) and
// answer differently per request — needed to simulate the has_next probe,
// where page and page+1 must return different bodies.
//
// /api/v1/settings/api is served separately (not routed through handler, and
// not recorded in *records) and defaults to 404, so the instance ceiling is
// "unknown" and every existing caller of newQueryBackend keeps exercising the
// probe-based fallback path exactly as before. Use newQueryBackendWithSettings
// to test the known-ceiling path.
func newQueryBackend(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, records *[]recordedReq)) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	return newQueryBackendWithSettings(t, http.StatusNotFound, "", handler)
}

// newQueryBackendWithSettings is newQueryBackend with a configurable
// /api/v1/settings/api response, so tests can exercise the known-ceiling
// path. settingsStatus 0 defaults to 404 (ceiling unknown). The settings
// request itself is never appended to *records, matching how /api/v1/version
// is already excluded — both are SDK-internal plumbing, not the search
// request under test.
func newQueryBackendWithSettings(
	t *testing.T,
	settingsStatus int,
	settingsBody string,
	handler func(w http.ResponseWriter, r *http.Request, records *[]recordedReq),
) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	if settingsStatus == 0 {
		settingsStatus = http.StatusNotFound
	}
	if settingsBody == "" {
		settingsBody = `{"message":"not found"}`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/api/v1/settings/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(settingsStatus)
		_, _ = w.Write([]byte(settingsBody))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, rawBody: body})
		w.Header().Set("Content-Type", "application/json")
		handler(w, r, &records)
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
	// Resets the cached MaxResponseItems ceiling too, so one test's ceiling
	// (or lack thereof) never leaks into the next.
	forgejo.SetClientForTesting(c)
	return srv, &records
}

// knownCeilingBody is the /api/v1/settings/api response body used by tests
// that need a known instance pagination ceiling of 50.
const knownCeilingBody = `{"max_response_items":50,"default_paging_num":30,"default_git_trees_per_page":1000,"default_max_blob_size":10485760}`

// pageFromQuery extracts the "page" query parameter, defaulting to "1" when absent
// (matching the SDK's own default).
func pageFromQuery(r *http.Request) string {
	p := r.URL.Query().Get("page")
	if p == "" {
		return "1"
	}
	return p
}

func TestSearchIssues_OwnerFilterAndMultiRepo(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.URL.Query().Get("owner") != "goern" {
			t.Fatalf("expected owner query filter, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[
			{"id":1,"number":1,"title":"one","repository":{"full_name":"goern/repo-a"}},
			{"id":2,"number":2,"title":"two","repository":{"full_name":"goern/repo-b"}}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	text := textOf(res)
	if !strings.Contains(text, "repo-a") || !strings.Contains(text, "repo-b") {
		t.Fatalf("expected issues from multiple repos, got %s", text)
	}
	if len(*records) == 0 {
		t.Fatal("expected at least one request to backend")
	}
}

func TestSearchIssues_EnvelopeShape(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"},{"id":2,"number":2,"title":"two"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	var wrapper struct {
		Result struct {
			Issues  []map[string]any `json:"issues"`
			Page    int              `json:"page"`
			Limit   int              `json:"limit"`
			Count   int              `json:"count"`
			HasNext bool             `json:"has_next"`
		} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &wrapper); err != nil {
		t.Fatalf("invalid envelope JSON: %v\nbody: %s", err, textOf(res))
	}
	envelope := wrapper.Result
	if envelope.Page != 1 || envelope.Limit != 20 {
		t.Fatalf("expected default page=1 limit=20, got page=%d limit=%d", envelope.Page, envelope.Limit)
	}
	if envelope.Count != len(envelope.Issues) {
		t.Fatalf("count %d does not match issues length %d", envelope.Count, len(envelope.Issues))
	}
	if envelope.Count != 2 {
		t.Fatalf("expected 2 issues, got %d", envelope.Count)
	}
}

func TestSearchIssues_TotalCountFromHeader(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.Header().Set("X-Total-Count", "333")
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"},{"id":2,"number":2,"title":"two"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"total_count":333`) {
		t.Fatalf("expected total_count from X-Total-Count header, got %s", textOf(res))
	}
}

// A confirmed-zero total and an unknown total are different answers, and
// `omitempty` on the *int must not collapse them: X-Total-Count: 0 has to
// marshal as "total_count":0, while a missing header drops the key entirely
// (asserted by TestSearchIssues_TotalCountOmittedWhenHeaderAbsent below).
func TestSearchIssues_TotalCountZeroIsEmittedNotOmitted(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
		w.Header().Set("X-Total-Count", "0")
		_, _ = w.Write([]byte(`[]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"total_count":0`) {
		t.Fatalf("expected a confirmed-zero total_count to be emitted, got %s", textOf(res))
	}
}

func TestSearchIssues_TotalCountOmittedWhenHeaderAbsent(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if strings.Contains(textOf(res), "total_count") {
		t.Fatalf("expected total_count to be omitted when header is absent, got %s", textOf(res))
	}
}

func TestSearchIssues_TotalCountOmittedWhenHeaderUnparsable(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.Header().Set("X-Total-Count", "not-a-number")
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if strings.Contains(textOf(res), "total_count") {
		t.Fatalf("expected total_count to be omitted for a garbage header, got %s", textOf(res))
	}
}

func TestSearchIssues_HasNextTrueWhenProbeReturnsRows(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[{"id":3,"number":3,"title":"three"}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(1)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"has_next":true`) {
		t.Fatalf("expected has_next true, got %s", textOf(res))
	}
	// Current page + probe page.
	if len(*records) != 2 {
		t.Fatalf("expected 2 requests (page + probe), got %d", len(*records))
	}
}

func TestSearchIssues_HasNextFalseWhenProbeReturnsNoRows(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"has_next":false`) {
		t.Fatalf("expected has_next false, got %s", textOf(res))
	}
}

func TestSearchIssues_HasNextFalseOnEmptyPage(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
		_, _ = w.Write([]byte(`[]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"has_next":false`) || !strings.Contains(textOf(res), `"count":0`) {
		t.Fatalf("expected empty page with has_next false, got %s", textOf(res))
	}
	// No probe should be made when the page itself is empty.
	if len(*records) != 1 {
		t.Fatalf("expected exactly 1 request (no probe on empty page), got %d", len(*records))
	}
}

func TestSearchIssues_DefaultPagingAndClamping(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.URL.Query().Get("limit") != "20" || r.URL.Query().Get("page") != "1" {
			t.Fatalf("expected clamped page=1 limit=20, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "page": float64(0), "limit": float64(-5),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"page":1`) || !strings.Contains(textOf(res), `"limit":20`) {
		t.Fatalf("expected reported page=1 limit=20, got %s", textOf(res))
	}
	if len(*records) == 0 {
		t.Fatal("expected at least one request")
	}
}

func TestSearchIssues_CallerRaisedLimit(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(50)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"limit":50`) {
		t.Fatalf("expected reported limit=50, got %s", textOf(res))
	}
	if len(*records) == 0 {
		t.Fatal("expected at least one request")
	}
}

func TestSearchIssues_MissingOwnerErrorsWithoutUpstreamCall(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
		_, _ = w.Write([]byte(`[]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{}))
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("expected error naming owner, got %v", err)
	}
	if len(*records) != 0 {
		t.Fatalf("expected no upstream request, got %d", len(*records))
	}

	res, err = SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": ""}))
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("expected error naming owner for empty owner, got %v", err)
	}
	if len(*records) != 0 {
		t.Fatalf("expected no upstream request for empty owner, got %d", len(*records))
	}
}

func TestSearchIssues_FilterPassThrough(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		q := r.URL.Query()
		if q.Get("state") != "closed" {
			t.Fatalf("expected state=closed, got %s", r.URL.RawQuery)
		}
		if q.Get("labels") != "bug,triage" {
			t.Fatalf("expected labels passthrough, got %s", r.URL.RawQuery)
		}
		if q.Get("q") != "crash" {
			t.Fatalf("expected q passthrough, got %s", r.URL.RawQuery)
		}
		// type is unconditionally emitted by the SDK, but empty when omitted.
		if !q.Has("type") || q.Get("type") != "" {
			t.Fatalf("expected type present and empty, got %s", r.URL.RawQuery)
		}
		// Filters the caller omitted must carry no value at all.
		for _, name := range []string{"created_by", "assigned_by", "mentioned_by", "milestones", "team"} {
			if q.Has(name) {
				t.Fatalf("expected %s to be absent, got %s", name, r.URL.RawQuery)
			}
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{
		"owner":  "goern",
		"state":  "closed",
		"labels": "bug,triage",
		"q":      "crash",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if len(*records) == 0 {
		t.Fatal("expected at least one request")
	}
}

// TestSearchIssues_AllOptionalFiltersPassThrough covers the remaining
// pass-through filters (type, milestones, created_by, assigned_by,
// mentioned_by), including the "excluding pull requests" scenario.
func TestSearchIssues_AllOptionalFiltersPassThrough(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		q := r.URL.Query()
		if q.Get("type") != "issues" {
			t.Fatalf("expected type=issues, got %s", r.URL.RawQuery)
		}
		if q.Get("milestones") != "v1,v2" {
			t.Fatalf("expected milestones passthrough, got %s", r.URL.RawQuery)
		}
		if q.Get("created_by") != "alice" {
			t.Fatalf("expected created_by passthrough, got %s", r.URL.RawQuery)
		}
		if q.Get("assigned_by") != "bob" {
			t.Fatalf("expected assigned_by passthrough, got %s", r.URL.RawQuery)
		}
		if q.Get("mentioned_by") != "carol" {
			t.Fatalf("expected mentioned_by passthrough, got %s", r.URL.RawQuery)
		}
		// The tool never sets opt.Team, so the SDK's "team" bug (design
		// Decision 5) cannot fire here even though MentionedBy is set.
		if q.Has("team") {
			t.Fatalf("expected no team key since the tool never sets opt.Team, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one","pull_request":null}]`))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{
		"owner":        "goern",
		"type":         "issues",
		"milestones":   "v1,v2",
		"created_by":   "alice",
		"assigned_by":  "bob",
		"mentioned_by": "carol",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	if len(*records) == 0 {
		t.Fatal("expected at least one request")
	}
}

// TestSearchIssues_ClampedPageStillSignalsContinuation is the clamping
// falsifier from battle-test.md: an instance silently clamping limit=100
// down to 50 results must still report has_next true when a further page
// exists. The count == limit heuristic gets this wrong; the same-limit probe
// does not.
//
// This scenario is now specifically the UNKNOWN-ceiling fallback path: when
// the instance ceiling is known, a limit that would be clamped is rejected
// before the upstream call (see TestSearchIssues_LimitAboveKnownCeilingRejected),
// so this exact mismatch cannot occur there. newQueryBackend's settings
// endpoint 404s by default, so this test still exercises "ceiling unknown"
// without any change.
func TestSearchIssues_ClampedPageStillSignalsContinuation(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			// Further matching issues exist beyond the clamped page.
			_, _ = w.Write([]byte(`[{"id":51,"number":51,"title":"extra"}]`))
			return
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("expected requested limit=100, got %s", r.URL.RawQuery)
		}
		// Simulate the instance clamping at MAX_RESPONSE_ITEMS=50: 50 issues
		// come back even though the caller asked for 100.
		issues := make([]string, 0, 50)
		for i := 1; i <= 50; i++ {
			issues = append(issues, fmt.Sprintf(`{"id":%d,"number":%d,"title":"issue-%d"}`, i, i, i))
		}
		_, _ = w.Write([]byte("[" + strings.Join(issues, ",") + "]"))
	})

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(100)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	text := textOf(res)
	if !strings.Contains(text, `"count":50`) {
		t.Fatalf("expected count=50 (clamped), got %s", text)
	}
	if !strings.Contains(text, `"has_next":true`) {
		t.Fatalf("count==limit heuristic would report false here; probe must report has_next true, got %s", text)
	}
	if len(*records) != 2 {
		t.Fatalf("expected page request + probe, got %d", len(*records))
	}
}

// TestSearchIssues_LimitAboveKnownCeilingRejected is the case chris420's
// review (PR #458 issuecomment-5533) argued for: when the ceiling is
// discoverable, an over-ceiling limit must be rejected before the upstream
// call, not silently truncated with a misleading envelope. The error must
// name both the requested limit and the ceiling so an LLM caller can
// self-correct the retry.
func TestSearchIssues_LimitAboveKnownCeilingRejected(t *testing.T) {
	_, records := newQueryBackendWithSettings(t, http.StatusOK, knownCeilingBody,
		func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
			t.Fatal("no upstream search request expected when limit exceeds the known ceiling")
		},
	)

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(100)}))
	if res != nil {
		t.Fatalf("expected nil result on rejection, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected an error when limit exceeds the known ceiling")
	}
	if !strings.Contains(err.Error(), "100") || !strings.Contains(err.Error(), "50") {
		t.Fatalf("expected error naming both the requested limit (100) and the ceiling (50), got %v", err)
	}
	if len(*records) != 0 {
		t.Fatalf("expected no upstream search request, got %d", len(*records))
	}
}

// TestSearchIssues_LimitEqualsKnownCeilingAccepted asserts the boundary:
// limit == ceiling is accepted, not rejected.
func TestSearchIssues_LimitEqualsKnownCeilingAccepted(t *testing.T) {
	_, records := newQueryBackendWithSettings(t, http.StatusOK, knownCeilingBody,
		func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
			if r.URL.Query().Get("limit") != "50" {
				t.Fatalf("expected limit=50, got %s", r.URL.RawQuery)
			}
			issues := make([]string, 0, 50)
			for i := 1; i <= 50; i++ {
				issues = append(issues, fmt.Sprintf(`{"id":%d,"number":%d,"title":"issue-%d"}`, i, i, i))
			}
			_, _ = w.Write([]byte("[" + strings.Join(issues, ",") + "]"))
		},
	)

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(50)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	text := textOf(res)
	if !strings.Contains(text, `"limit":50`) {
		t.Fatalf("expected reported limit=50, got %s", text)
	}
	if !strings.Contains(text, `"count":50`) {
		t.Fatalf("expected count=50, got %s", text)
	}
	// Full page under a known, honored ceiling: count == limit is sound here
	// (see design.md Decision 3, revised), so has_next must be true without
	// a probe.
	if !strings.Contains(text, `"has_next":true`) {
		t.Fatalf("expected has_next=true for a full page under a known ceiling, got %s", text)
	}
	if len(*records) != 1 {
		t.Fatalf("expected exactly 1 request (no probe when ceiling is known), got %d", len(*records))
	}
}

// TestSearchIssues_LimitBelowKnownCeilingAccepted covers the ordinary
// below-ceiling case and its no-probe, short-page has_next=false semantics.
func TestSearchIssues_LimitBelowKnownCeilingAccepted(t *testing.T) {
	_, records := newQueryBackendWithSettings(t, http.StatusOK, knownCeilingBody,
		func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
			if r.URL.Query().Get("limit") != "10" {
				t.Fatalf("expected limit=10, got %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
		},
	)

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(10)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	text := textOf(res)
	if !strings.Contains(text, `"limit":10`) {
		t.Fatalf("expected reported limit=10, got %s", text)
	}
	if !strings.Contains(text, `"has_next":false`) {
		t.Fatalf("expected has_next=false for a short page (count < limit), got %s", text)
	}
	if len(*records) != 1 {
		t.Fatalf("expected exactly 1 request (no probe when ceiling is known), got %d", len(*records))
	}
}

// TestSearchIssues_EffectiveLimitReported is the direct regression test for
// the bug chris420 found: issue.go used to report `Limit: int(limit)` (the
// raw requested value) in the envelope regardless of what was actually sent
// upstream. Under this design, an over-ceiling request is rejected outright
// (see TestSearchIssues_LimitAboveKnownCeilingRejected), so the specific
// "limit=100 reported, 50 rows returned" mismatch can no longer reach a
// successful envelope at all — this test pins that down for the accepted
// case: the envelope's `limit` must equal the value actually sent upstream,
// sourced from the same variable, not recomputed independently.
func TestSearchIssues_EffectiveLimitReported(t *testing.T) {
	_, records := newQueryBackendWithSettings(t, http.StatusOK, knownCeilingBody,
		func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
			// The instance ceiling is 50; the request below asks for 50 and
			// must be honored and reported as 50, never mutated en route.
			if r.URL.Query().Get("limit") != "50" {
				t.Fatalf("expected effective limit 50 sent upstream, got %s", r.URL.RawQuery)
			}
			issues := make([]string, 0, 50)
			for i := 1; i <= 50; i++ {
				issues = append(issues, fmt.Sprintf(`{"id":%d,"number":%d,"title":"issue-%d"}`, i, i, i))
			}
			_, _ = w.Write([]byte("[" + strings.Join(issues, ",") + "]"))
		},
	)

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(50)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("SearchIssuesFn err: %v res=%+v", err, res)
	}
	var wrapper struct {
		Result struct {
			Limit int `json:"limit"`
			Count int `json:"count"`
		} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &wrapper); err != nil {
		t.Fatalf("invalid envelope JSON: %v\nbody: %s", err, textOf(res))
	}
	if wrapper.Result.Limit != 50 {
		t.Fatalf("expected envelope limit=50 (the effective, actually-sent limit), got %d", wrapper.Result.Limit)
	}
	if len(*records) != 1 {
		t.Fatalf("expected exactly 1 upstream request, got %d", len(*records))
	}
}

// TestSearchIssues_SettingsFailureNeverBecomesZeroCeiling asserts that a
// broken /api/v1/settings/api endpoint (500, not merely 403/404) is treated
// as "ceiling unknown" and falls back to the probe, rather than being
// misread as a ceiling of 0 — a zero ceiling would reject every request,
// including this one for a modest limit of 5.
func TestSearchIssues_SettingsFailureNeverBecomesZeroCeiling(t *testing.T) {
	_, records := newQueryBackendWithSettings(t, http.StatusInternalServerError, `{"message":"boom"}`,
		func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
			if pageFromQuery(r) == "2" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
		},
	)

	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "limit": float64(5)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected success (ceiling unknown, not zero), got err=%v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"limit":5`) {
		t.Fatalf("expected reported limit=5, got %s", textOf(res))
	}
	// Ceiling unknown -> fallback probe still runs: page + probe.
	if len(*records) != 2 {
		t.Fatalf("expected page request + probe (ceiling unknown fallback), got %d", len(*records))
	}
}

func TestSearchIssues_ClientErrorPropagates(t *testing.T) {
	// No backend at all: forgejo.Client construction against an unreachable
	// URL surfaces as an error before any ListIssues call. Clear the singleton
	// first so this holds whatever order -shuffle picks.
	forgejo.ResetClientForTesting()
	t.Cleanup(forgejo.ResetClientForTesting)
	flag.URL = "http://127.0.0.1:0"
	flag.Token = "tkn"
	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected error when client cannot be built")
	}
}

func TestSearchIssues_ListIssuesErrorPropagates(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})
	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
	if err == nil || !strings.Contains(err.Error(), "search issues err") {
		t.Fatalf("expected wrapped search issues error, got %v", err)
	}
}

func TestSearchIssues_ProbeErrorPropagates(t *testing.T) {
	_, _ = newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if pageFromQuery(r) == "2" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})
	res, err := SearchIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern"}))
	if res != nil {
		t.Fatalf("expected nil result on error, got %+v", res)
	}
	if err == nil || !strings.Contains(err.Error(), "probe next issues page err") {
		t.Fatalf("expected wrapped probe error, got %v", err)
	}
}

func TestListRepoIssues_EmptyRepoNamesSearchIssues(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
		_, _ = w.Write([]byte(`[]`))
	})
	res, err := ListRepoIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "repo": ""}))
	if res != nil {
		t.Fatalf("expected nil result for empty repo, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected error for empty repo")
	}
	text := err.Error()
	if !strings.Contains(text, "repo") {
		t.Fatalf("expected error to name repo, got %s", text)
	}
	if !strings.Contains(text, "search_issues") {
		t.Fatalf("expected error to mention search_issues, got %s", text)
	}
	if strings.Contains(text, "path segment") {
		t.Fatalf("expected no SDK-internal wording, got %s", text)
	}
	if len(*records) != 0 {
		t.Fatalf("expected no upstream request, got %d", len(*records))
	}
}

func TestListRepoIssues_EmptyOwnerErrorsAndWellFormedCallUnchanged(t *testing.T) {
	_, records := newQueryBackend(t, func(w http.ResponseWriter, _ *http.Request, _ *[]recordedReq) {
		_, _ = w.Write([]byte(`[{"id":1,"number":1,"title":"one"}]`))
	})

	res, err := ListRepoIssuesFn(context.Background(), makeReq(map[string]any{"owner": "", "repo": "forgejo-mcp"}))
	if res != nil {
		t.Fatalf("expected nil result for empty owner, got %+v", res)
	}
	if err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("expected error naming owner for empty owner, got %v", err)
	}
	if len(*records) != 0 {
		t.Fatalf("expected no upstream request for empty owner, got %d", len(*records))
	}

	res, err = ListRepoIssuesFn(context.Background(), makeReq(map[string]any{"owner": "goern", "repo": "forgejo-mcp"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("well-formed call should be unaffected, got err=%v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"title":"one"`) {
		t.Fatalf("expected issue payload unchanged, got %s", textOf(res))
	}
	if len(*records) != 1 {
		t.Fatalf("expected exactly one upstream request for well-formed call, got %d", len(*records))
	}
}

// The tests below cover the remaining simple wrapper handlers so package
// coverage reflects the state of the code this change touches, not just the
// two functions changed directly.

func TestGetIssueByIndexFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42,"title":"hello"}`)
	res, err := GetIssueByIndexFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("GetIssueByIndexFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"title":"hello"`) {
		t.Fatalf("unexpected result: %s", textOf(res))
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestCreateIssueFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":1,"title":"new issue"}`)
	res, err := CreateIssueFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "title": "new issue", "body": "body text",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("CreateIssueFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"title":"new issue"`) {
		t.Fatalf("unexpected result: %s", textOf(res))
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestCreateIssueCommentFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"body":"a comment"}`)
	res, err := CreateIssueCommentFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "body": "a comment",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("CreateIssueCommentFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"body":"a comment"`) {
		t.Fatalf("unexpected result: %s", textOf(res))
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestAddIssueLabelsFn(t *testing.T) {
	// AddIssueLabels (POST .../labels) returns a label list; the trailing
	// GetIssue call returns the refreshed issue. Distinguish by method.
	_, records := newQueryBackend(t, func(w http.ResponseWriter, r *http.Request, _ *[]recordedReq) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`[{"id":5},{"id":6}]`))
			return
		}
		_, _ = w.Write([]byte(`{"id":1,"number":1,"labels":[{"id":5},{"id":6}]}`))
	})
	res, err := AddIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "5,6",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("AddIssueLabelsFn err: %v res=%+v", err, res)
	}
	// Two calls: AddIssueLabels then GetIssue to return the refreshed issue.
	if len(*records) != 2 {
		t.Fatalf("expected two requests, got %d", len(*records))
	}
}

func TestAddIssueLabelsFn_InvalidLabelID(t *testing.T) {
	_, _ = newPatchBackend(t, `{"id":1}`)
	res, err := AddIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "not-a-number",
	}))
	if res != nil {
		t.Fatalf("expected nil result on invalid label ID, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected error for non-numeric label ID")
	}
}

func TestRemoveIssueLabelsFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":1,"labels":[]}`)
	res, err := RemoveIssueLabelsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "labels": "5",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("RemoveIssueLabelsFn err: %v res=%+v", err, res)
	}
	// Two calls: DeleteIssueLabel then GetIssue to return the refreshed issue.
	if len(*records) != 2 {
		t.Fatalf("expected two requests, got %d", len(*records))
	}
}

func TestIssueStateChangeFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":1,"state":"closed"}`)
	res, err := IssueStateChangeFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "state": "closed",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("IssueStateChangeFn err: %v res=%+v", err, res)
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}

	res, err = IssueStateChangeFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "state": "bogus",
	}))
	if res != nil {
		t.Fatalf("expected nil result for invalid state, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestListIssueCommentsFn(t *testing.T) {
	_, records := newPatchBackend(t, `[{"id":1,"body":"c1"},{"id":2,"body":"c2"}]`)
	res, err := ListIssueCommentsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1),
		"since": "2024-01-01T00:00:00Z", "before": "2024-06-01T00:00:00Z",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListIssueCommentsFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"body":"c1"`) {
		t.Fatalf("unexpected result: %s", textOf(res))
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestListIssueCommentsFn_InvalidSince(t *testing.T) {
	_, _ = newPatchBackend(t, `[]`)
	res, err := ListIssueCommentsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "index": float64(1), "since": "not-a-time",
	}))
	if res != nil {
		t.Fatalf("expected nil result for invalid since, got %+v", res)
	}
	if err == nil {
		t.Fatal("expected error for invalid since time")
	}
}

func TestGetIssueCommentFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":9,"body":"a comment"}`)
	res, err := GetIssueCommentFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "comment_id": float64(9),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("GetIssueCommentFn err: %v res=%+v", err, res)
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestEditIssueCommentFn(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":9,"body":"updated"}`)
	res, err := EditIssueCommentFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "comment_id": float64(9), "body": "updated",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("EditIssueCommentFn err: %v res=%+v", err, res)
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestDeleteIssueCommentFn(t *testing.T) {
	_, records := newPatchBackend(t, ``)
	res, err := DeleteIssueCommentFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp", "comment_id": float64(9),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("DeleteIssueCommentFn err: %v res=%+v", err, res)
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

func TestListRepoMilestonesFn(t *testing.T) {
	_, records := newPatchBackend(t, `[{"id":1,"title":"v1"}]`)
	res, err := ListRepoMilestonesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern", "repo": "forgejo-mcp",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoMilestonesFn err: %v res=%+v", err, res)
	}
	if !strings.Contains(textOf(res), `"title":"v1"`) {
		t.Fatalf("unexpected result: %s", textOf(res))
	}
	if len(*records) != 1 {
		t.Fatalf("expected one request, got %d", len(*records))
	}
}

// --- daikon#93: due_date read/write/sort ---

func TestUpdateIssue_DueDateSet(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42}`)

	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":    "goern",
		"repo":     "forgejo-mcp",
		"index":    float64(42),
		"due_date": "2026-08-20T00:00:00Z",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("UpdateIssueFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	dueDate, ok := payload["due_date"].(string)
	if !ok || !strings.HasPrefix(dueDate, "2026-08-20") {
		t.Fatalf("expected due_date=2026-08-20..., got %v", payload["due_date"])
	}
	if v, ok := payload["unset_due_date"]; ok && v != nil {
		t.Fatalf("expected no unset_due_date field, got %v", v)
	}
}

func TestUpdateIssue_DueDateClear(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"number":42}`)

	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":          "goern",
		"repo":           "forgejo-mcp",
		"index":          float64(42),
		"clear_due_date": true,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("UpdateIssueFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	if v, ok := payload["unset_due_date"].(bool); !ok || !v {
		t.Fatalf("expected unset_due_date=true, got %v", payload["unset_due_date"])
	}
	if v, ok := payload["due_date"]; ok && v != nil {
		t.Fatalf("expected no due_date field when clearing, got %v", v)
	}
}

func TestUpdateIssue_DueDateSetAndClearConflict(t *testing.T) {
	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":          "goern",
		"repo":           "forgejo-mcp",
		"index":          float64(42),
		"due_date":       "2026-08-20T00:00:00Z",
		"clear_due_date": true,
	}))
	if err == nil {
		t.Fatalf("expected error when due_date and clear_due_date both set, got res=%+v", res)
	}
}

func TestUpdateIssue_DueDateInvalidFormat(t *testing.T) {
	res, err := UpdateIssueFn(context.Background(), makeReq(map[string]any{
		"owner":    "goern",
		"repo":     "forgejo-mcp",
		"index":    float64(42),
		"due_date": "not-a-date",
	}))
	if err == nil {
		t.Fatalf("expected error for invalid due_date format, got res=%+v", res)
	}
}

func TestGetIssueByIndex_DueDatePassesThrough(t *testing.T) {
	srv, _ := newPatchBackend(t, `{"id":1,"number":42,"due_date":"2026-08-20T23:59:59Z"}`)
	_ = srv

	res, err := GetIssueByIndexFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("GetIssueByIndexFn returned error: err=%v res=%+v", err, res)
	}
	text := textResultString(t, res)
	if !strings.Contains(text, "2026-08-20") {
		t.Fatalf("expected due_date to pass through in response, got: %s", text)
	}
}

func TestListRepoIssues_SortNearDueDate(t *testing.T) {
	srv, records := newPatchBackend(t, `[{"id":1,"number":42,"due_date":"2026-08-20T23:59:59Z"}]`)
	_ = srv

	res, err := ListRepoIssuesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"sort":  "nearduedate",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoIssuesFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	if last.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", last.method)
	}
	if !strings.Contains(last.query, "sort=nearduedate") {
		t.Fatalf("expected sort=nearduedate in query, got: %s", last.query)
	}
	text := textResultString(t, res)
	if !strings.Contains(text, "2026-08-20") {
		t.Fatalf("expected due_date to pass through in response, got: %s", text)
	}
}

// newNotFoundBackend serves 404 for every API path except the SDK's startup
// version probe, i.e. it stands in for "the repository does not exist".
func newNotFoundBackend(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"repository does not exist","url":"` +
			`https://example.invalid/api/swagger"}`))
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
	return srv
}

// TestListRepoIssues_MissingRepoErrorsWithAndWithoutSort pins the property
// that made the sort branch worth a second look: the two code paths must agree
// about a repository that does not exist. /repos/{owner}/{repo}/issues returns
// 200 [] when a repo simply has no issues, so a 404 there means the repo is
// missing and must surface as an error — not as an empty list — regardless of
// whether the caller passed `sort`.
func TestListRepoIssues_MissingRepoErrorsWithAndWithoutSort(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"without sort", map[string]any{"owner": "nope", "repo": "nope"}},
		{"with sort", map[string]any{"owner": "nope", "repo": "nope", "sort": "nearduedate"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			newNotFoundBackend(t)

			res, err := ListRepoIssuesFn(context.Background(), makeReq(tc.args))
			if err == nil && (res == nil || !res.IsError) {
				t.Fatalf("missing repository must surface as an error, got err=%v res=%+v", err, res)
			}
		})
	}
}

// TestListRepoIssues_SortPathEscapesSegments guards the raw-HTTP path built by
// the sort branch: this is the first raw site to append a query string to a
// hand-built path, so an unescaped "?" in repo would merge into the query and
// swallow the /issues segment rather than 404.
func TestListRepoIssues_SortPathEscapesSegments(t *testing.T) {
	// The request target has to be read raw: http.Request.URL.Path is the
	// *decoded* form, so %2F reads back as "/" there and would hide exactly
	// the escaping this test exists to prove. RequestURI is the untouched
	// wire value, and a bare handler (no ServeMux) keeps it unrewritten.
	var requestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/version" {
			_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
			return
		}
		requestURI = r.RequestURI
		_, _ = w.Write([]byte(`[]`))
	}))
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

	res, err := ListRepoIssuesFn(context.Background(), makeReq(map[string]any{
		"owner": "o/x/../../admin",
		"repo":  "r?state=all&",
		"sort":  "nearduedate",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListRepoIssuesFn returned error: err=%v res=%+v", err, res)
	}

	rawPath, rawQuery, _ := strings.Cut(requestURI, "?")
	if !strings.HasSuffix(rawPath, "/issues") {
		t.Fatalf("the %q in repo merged into the query and swallowed /issues; request target = %q", "?", requestURI)
	}
	if strings.Contains(rawPath, "/x/") || strings.Contains(rawPath, "/../") {
		t.Fatalf("path traversal was not contained; request target = %q", requestURI)
	}
	if !strings.Contains(rawQuery, "sort=nearduedate") {
		t.Fatalf("expected sort=nearduedate in query, got request target = %q", requestURI)
	}
}

func TestListRepoIssues_SortInvalidRejected(t *testing.T) {
	res, err := ListRepoIssuesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"sort":  "not-a-real-sort",
	}))
	if err == nil {
		t.Fatalf("expected error for invalid sort value, got res=%+v", res)
	}
}

// textResultString extracts the text payload from an mcp.CallToolResult
// built by pkg/to.TextResult (a single text content block).
func textResultString(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatalf("result has no content")
	}
	tc, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("result content is not text: %+v", res.Content[0])
	}
	return tc.Text
}
