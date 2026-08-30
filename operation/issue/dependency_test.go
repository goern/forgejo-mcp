// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// newDependenciesBackend stubs the endpoints used by issue-dependency tools.
func newDependenciesBackend(t *testing.T) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"15.0.2+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, rawBody: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
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

func TestListIssueDependencies_SendsGetToDependencies(t *testing.T) {
	_, records := newDependenciesBackend(t)

	res, err := ListIssueDependenciesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListIssueDependenciesFn returned error: err=%v res=%+v", err, res)
	}

	if len(*records) == 0 {
		t.Fatal("expected request to backend")
	}
	last := (*records)[len(*records)-1]
	if last.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", last.method)
	}
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/dependencies"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}
	if !strings.Contains(last.query, "page=1") {
		t.Fatalf("expected default page=1 in query, got %s", last.query)
	}
	if !strings.Contains(last.query, "limit=20") {
		t.Fatalf("expected default limit=20 in query, got %s", last.query)
	}

	out := textOf(res)
	if !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
		t.Fatalf("expected response to echo page/limit, got %q", out)
	}
}

func TestListIssueDependents_SendsGetToBlocks(t *testing.T) {
	_, records := newDependenciesBackend(t)

	res, err := ListIssueDependentsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListIssueDependentsFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	if last.method != http.MethodGet {
		t.Fatalf("expected GET, got %s", last.method)
	}
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/blocks"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}
	if !strings.Contains(last.query, "page=1") || !strings.Contains(last.query, "limit=20") {
		t.Fatalf("expected default page/limit in query, got %s", last.query)
	}

	out := textOf(res)
	if !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
		t.Fatalf("expected response to echo page/limit, got %q", out)
	}
}

func TestAddIssueDependency_SendsPostWithIssueMeta(t *testing.T) {
	_, records := newDependenciesBackend(t)

	res, err := AddIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"depends_on_index": float64(7),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("AddIssueDependencyFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	if last.method != http.MethodPost {
		t.Fatalf("expected POST, got %s", last.method)
	}
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/dependencies"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}

	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	if payload["index"] != float64(7) {
		t.Fatalf("expected index=7, got %v", payload["index"])
	}
	if payload["owner"] != "goern" {
		t.Fatalf("expected owner=goern, got %v", payload["owner"])
	}
	if payload["repo"] != "forgejo-mcp" {
		t.Fatalf("expected repo=forgejo-mcp, got %v", payload["repo"])
	}
	if !strings.Contains(textOf(res), "now depends on") {
		t.Fatalf("expected success message, got %q", textOf(res))
	}
}

func TestAddIssueDependency_CrossRepoBodyShape(t *testing.T) {
	_, records := newDependenciesBackend(t)

	res, err := AddIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"depends_on_index": float64(7),
		"depends_on_owner": "other-org",
		"depends_on_repo":  "other-repo",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("AddIssueDependencyFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/dependencies"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}
	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	if payload["owner"] != "other-org" || payload["repo"] != "other-repo" || payload["index"] != float64(7) {
		t.Fatalf("expected cross-repo IssueMeta body, got %v", payload)
	}
}

func TestAddIssueDependency_SelfDependencyIsCaseInsensitive(t *testing.T) {
	_, records := newDependenciesBackend(t)

	// Forgejo treats owner and repository names case-insensitively, so a
	// differently-cased spelling names the same repository. Comparing the two
	// exactly let that spelling past the early check, which then reported a
	// generic server-side error instead of the clear one.
	_, err := AddIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(7),
		"depends_on_index": float64(7),
		"depends_on_owner": "Goern",
		"depends_on_repo":  "Forgejo-MCP",
	}))
	if err == nil {
		t.Fatal("a differently-cased spelling of the same repo was not caught as a self-dependency")
	}
	if !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Fatalf("wrong error for a case-differing self-dependency: %v", err)
	}
	if len(*records) > 0 {
		t.Fatalf("expected no HTTP request, got %d", len(*records))
	}
}

func TestAddIssueDependency_CrossRepoSameIndexAllowed(t *testing.T) {
	_, records := newDependenciesBackend(t)

	// Same index in a DIFFERENT repo is not a self-dependency.
	res, err := AddIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"depends_on_index": float64(42),
		"depends_on_repo":  "other-repo",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected cross-repo same-index to be allowed: err=%v res=%+v", err, res)
	}
	if len(*records) == 0 {
		t.Fatal("expected an HTTP request for cross-repo same-index dependency")
	}
}

func TestAddIssueDependency_SelfDependencyRejected(t *testing.T) {
	_, records := newDependenciesBackend(t)

	_, err := AddIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"depends_on_index": float64(42),
	}))
	if err == nil {
		t.Fatal("expected self-dependency to be rejected, got nil error")
	}
	if len(*records) > 0 {
		t.Fatalf("expected no HTTP request for self-dependency, got %d", len(*records))
	}
}

// TestAddIssueDependency_MissingDependsOnIndexIsNotMisreportedAsSelfDependency
// guards the self-dependency check: index and depends_on_index must be
// validated BEFORE that check runs. Previously both were parsed with
// to.Float64's error discarded, so a missing/malformed depends_on_index
// silently coerced to 0 — indistinguishable from a genuine index=0 — and, if
// index also happened to be 0, would misreport as "an issue cannot depend on
// itself" instead of the real problem (a missing required argument).
func TestAddIssueDependency_MissingDependsOnIndexIsNotMisreportedAsSelfDependency(t *testing.T) {
	_, records := newDependenciesBackend(t)

	_, err := AddIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
		// depends_on_index intentionally omitted.
	}))
	if err == nil {
		t.Fatal("expected an error for missing depends_on_index")
	}
	if strings.Contains(err.Error(), "cannot depend on itself") {
		t.Fatalf("missing depends_on_index was misreported as a self-dependency: %v", err)
	}
	if len(*records) > 0 {
		t.Fatalf("expected no HTTP request for missing depends_on_index, got %d", len(*records))
	}
}

func TestRemoveIssueDependency_SendsDeleteWithIssueMeta(t *testing.T) {
	_, records := newDependenciesBackend(t)

	res, err := RemoveIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"dependency_index": float64(7),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("RemoveIssueDependencyFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	if last.method != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", last.method)
	}
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/dependencies"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}

	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	if payload["index"] != float64(7) {
		t.Fatalf("expected index=7, got %v", payload["index"])
	}
	if payload["owner"] != "goern" || payload["repo"] != "forgejo-mcp" {
		t.Fatalf("expected owner/repo to match repo, got %v", payload)
	}
	if !strings.Contains(textOf(res), "Removed dependency") {
		t.Fatalf("expected success message, got %q", textOf(res))
	}
}

// TestRemoveIssueDependency_CrossRepoBodyShape mirrors
// TestAddIssueDependency_CrossRepoBodyShape for the DELETE body: the
// dependency_owner/dependency_repo arguments must resolve into the
// IssueMeta request body, not the target issue's own owner/repo.
func TestRemoveIssueDependency_CrossRepoBodyShape(t *testing.T) {
	_, records := newDependenciesBackend(t)

	res, err := RemoveIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"dependency_index": float64(7),
		"dependency_owner": "other-org",
		"dependency_repo":  "other-repo",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("RemoveIssueDependencyFn returned error: err=%v res=%+v", err, res)
	}

	last := (*records)[len(*records)-1]
	if last.method != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", last.method)
	}
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/dependencies"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}
	var payload map[string]any
	if err := json.Unmarshal(last.rawBody, &payload); err != nil {
		t.Fatalf("invalid JSON body: %v\nbody: %s", err, last.rawBody)
	}
	if payload["owner"] != "other-org" || payload["repo"] != "other-repo" || payload["index"] != float64(7) {
		t.Fatalf("expected cross-repo IssueMeta body, got %v", payload)
	}
	if !strings.Contains(textOf(res), "other-org/other-repo#7") {
		t.Fatalf("expected success message to name the cross-repo dependency, got %q", textOf(res))
	}
}

func TestListIssueDependencies_DecodesIssueList(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"15.0.2+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, rawBody: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"number":7,"title":"dependency"}]`))
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

	res, err := ListIssueDependenciesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
		"page":  float64(2),
		"limit": float64(5),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListIssueDependenciesFn returned error: err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"number":7`) || !strings.Contains(out, `"title":"dependency"`) {
		t.Fatalf("expected decoded issue list, got %q", out)
	}
	if !strings.Contains(out, `"page":2`) || !strings.Contains(out, `"limit":5`) {
		t.Fatalf("expected response to echo requested page/limit, got %q", out)
	}
	last := records[len(records)-1]
	if !strings.Contains(last.query, "page=2") || !strings.Contains(last.query, "limit=5") {
		t.Fatalf("expected query params page=2&limit=5, got %s", last.query)
	}
}

func TestListIssueDependencies_APIErrorSurfaces(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"15.0.2+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"issue dependencies are disabled"}`))
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

	_, err = ListIssueDependenciesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
	}))
	if err == nil {
		t.Fatal("expected API error to surface, got nil")
	}
}

func TestListIssueDependencies_404IsEmpty(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"15.0.2+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
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

	res, err := ListIssueDependenciesFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected 404 to be treated as empty list, got err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"issues":[]`) {
		t.Fatalf("expected empty result for 404, got %q", out)
	}
	if !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
		t.Fatalf("expected response to echo default page/limit, got %q", out)
	}
}

func TestListIssueDependents_404IsEmpty(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"15.0.2+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
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

	res, err := ListIssueDependentsFn(context.Background(), makeReq(map[string]any{
		"owner": "goern",
		"repo":  "forgejo-mcp",
		"index": float64(42),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("expected 404 to be treated as empty list, got err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"issues":[]`) {
		t.Fatalf("expected empty result for 404, got %q", out)
	}
	if !strings.Contains(out, `"page":1`) || !strings.Contains(out, `"limit":20`) {
		t.Fatalf("expected response to echo default page/limit, got %q", out)
	}
}

func TestRemoveIssueDependency_APIErrorSurfaces(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"15.0.2+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"issue dependencies are disabled"}`))
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

	_, err = RemoveIssueDependencyFn(context.Background(), makeReq(map[string]any{
		"owner":            "goern",
		"repo":             "forgejo-mcp",
		"index":            float64(42),
		"dependency_index": float64(7),
	}))
	if err == nil {
		t.Fatal("expected API error to surface, got nil")
	}
}

// Ensure CallToolRequest is used for the shared makeReq helper.
var _ mcp.CallToolRequest = mcp.CallToolRequest{}
