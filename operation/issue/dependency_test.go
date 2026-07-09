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

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// newDependenciesBackend stubs the endpoints used by issue-dependency tools.
// Caller provides bodies/status for each path pattern. Defaults are 200 OK with an empty body.
func newDependenciesBackend(t *testing.T) (*httptest.Server, *[]recordedReq) {
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

func TestListIssueDependencies_SendsGet(t *testing.T) {
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
}

func TestAddIssueDependency_SendsPostWithBody(t *testing.T) {
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
	if payload["dependency_issue_index"] != float64(7) {
		t.Fatalf("expected dependency_issue_index=7, got %v", payload["dependency_issue_index"])
	}
	if !strings.Contains(textOf(res), "now depends on") {
		t.Fatalf("expected success message, got %q", textOf(res))
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

func TestRemoveIssueDependency_SendsDelete(t *testing.T) {
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
	want := "/api/v1/repos/goern/forgejo-mcp/issues/42/dependencies/7"
	if last.path != want {
		t.Fatalf("unexpected path: got %s want %s", last.path, want)
	}
	if !strings.Contains(textOf(res), "Removed dependency") {
		t.Fatalf("expected success message, got %q", textOf(res))
	}
}

func TestListIssueDependencies_DecodesIssueList(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{method: r.Method, path: r.URL.Path, rawBody: body})
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
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("ListIssueDependenciesFn returned error: err=%v res=%+v", err, res)
	}
	out := textOf(res)
	if !strings.Contains(out, `"number":7`) || !strings.Contains(out, `"title":"dependency"`) {
		t.Fatalf("expected decoded issue list, got %q", out)
	}
}

func TestListIssueDependencies_APIErrorSurfaces(t *testing.T) {
	records := make([]recordedReq, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
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
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
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
	if !strings.Contains(textOf(res), "[]") && !strings.Contains(textOf(res), "null") {
		t.Fatalf("expected empty result for 404, got %q", textOf(res))
	}
}

// Ensure CallToolRequest is used for the shared makeReq helper.
var _ mcp.CallToolRequest = mcp.CallToolRequest{}
