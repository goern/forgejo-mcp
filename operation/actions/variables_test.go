// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

type variableCapture struct {
	method string
	uri    string
	body   string
	hits   int
}

func setupVariableMockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *variableCapture) {
	t.Helper()
	capture := &variableCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.hits++
		capture.method = r.Method
		capture.uri = r.URL.RequestURI()
		if r.Body != nil {
			defer r.Body.Close()
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			capture.body = string(buf[:n])
		}
		if handler != nil {
			handler(w, r)
		}
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv, capture
}

func toolText(t *testing.T, result *mcp.CallToolResult, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("unexpected tool error: %#v", result)
	}
	return result.Content[0].(mcp.TextContent).Text
}

func TestListActionVariablesFn_RejectsCoordsBeforeHTTP(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP")
	})
	defer srv.Close()

	cases := []map[string]any{
		{},
		{"scope": "instance"},
		{"scope": "user", "owner": "o"},
		{"scope": "org"},
		{"scope": "org", "org": "acme", "repo": "r"},
		{"scope": "repo", "owner": "o"},
	}
	for _, args := range cases {
		_, err := ListActionVariablesFn(context.Background(), newCallToolRequest(args))
		if err == nil {
			t.Fatalf("args %v: expected error", args)
		}
	}
	if capture.hits != 0 {
		t.Fatalf("hits = %d, want 0", capture.hits)
	}
}

func TestListActionVariablesFn_OmitsValuesAndTotalCount(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "4")
		_, _ = w.Write([]byte(`[{"name":"TOKEN","data":"should-not-leak","owner_id":7,"repo_id":0}]`))
	})
	defer srv.Close()

	result, err := ListActionVariablesFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "user",
	}))
	text := toolText(t, result, err)
	if capture.uri != "/api/v1/user/actions/variables?limit=30&page=1" {
		t.Fatalf("uri %s", capture.uri)
	}
	if strings.Contains(text, "should-not-leak") || strings.Contains(text, `"data"`) {
		t.Fatalf("list leaked a value: %s", text)
	}
	var wrapped struct {
		Result listActionVariablesResult `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		t.Fatalf("json: %v %s", err, text)
	}
	got := wrapped.Result
	if got.Scope != "user" || got.Count != 1 || got.Page != 1 || got.Limit != 30 {
		t.Fatalf("envelope %#v", got)
	}
	if got.TotalCount == nil || *got.TotalCount != 4 {
		t.Fatalf("total_count %#v", got.TotalCount)
	}
	if len(got.Variables) != 1 || got.Variables[0].Name != "TOKEN" || got.Variables[0].OwnerID != 7 {
		t.Fatalf("rows %#v", got.Variables)
	}
}

func TestListActionVariablesFn_OmitsTotalCountWithoutHeader(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()

	result, err := ListActionVariablesFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "org", "org": "acme", "page": 2, "limit": 10,
	}))
	text := toolText(t, result, err)
	if capture.uri != "/api/v1/orgs/acme/actions/variables?limit=10&page=2" {
		t.Fatalf("uri %s", capture.uri)
	}
	if strings.Contains(text, "total_count") {
		t.Fatalf("total_count present without header: %s", text)
	}
}

func TestListActionVariablesFn_RejectsLimitAboveMax(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP")
	})
	defer srv.Close()

	_, err := ListActionVariablesFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "user", "limit": 51,
	}))
	if err == nil || !strings.Contains(err.Error(), "between 1 and 50") {
		t.Fatalf("error %v", err)
	}
	if capture.hits != 0 {
		t.Fatalf("hits = %d, want 0", capture.hits)
	}
}

func TestGetActionVariableFn_ReturnsData(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"TOKEN","data":"visible","owner_id":1,"repo_id":2}`))
	})
	defer srv.Close()

	result, err := GetActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "repo", "owner": "o", "repo": "r", "name": "TOKEN",
	}))
	text := toolText(t, result, err)
	if capture.uri != "/api/v1/repos/o/r/actions/variables/TOKEN" {
		t.Fatalf("uri %s", capture.uri)
	}
	if !strings.Contains(text, `"data":"visible"`) {
		t.Fatalf("get dropped the value: %s", text)
	}
}

func TestGetActionVariableFn_MissingIsError(t *testing.T) {
	srv, _ := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	})
	defer srv.Close()

	result, err := GetActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "user", "name": "MISSING",
	}))
	if err == nil {
		t.Fatalf("expected error, result %#v", result)
	}
	if strings.Contains(err.Error(), `"data"`) {
		t.Fatalf("missing name returned a value: %v", err)
	}
}

func TestCreateActionVariableFn_EmptyDataSendsNoRequest(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP")
	})
	defer srv.Close()

	_, err := CreateActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "user", "name": "TOKEN", "data": "",
	}))
	if err == nil || !strings.Contains(err.Error(), "data required") {
		t.Fatalf("error %v", err)
	}
	if capture.hits != 0 {
		t.Fatalf("hits = %d, want 0", capture.hits)
	}
}

func TestCreateActionVariableFn_SlashInNameSendsNoRequest(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected HTTP")
	})
	defer srv.Close()

	for _, name := range []string{"a/b", "a?b", "a#b"} {
		_, err := CreateActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
			"scope": "org", "org": "acme", "name": name, "data": "v",
		}))
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	if capture.hits != 0 {
		t.Fatalf("hits = %d uri %s", capture.hits, capture.uri)
	}
}

func TestCreateActionVariableFn_PostsValueAndDoesNotEchoIt(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	})
	defer srv.Close()

	result, err := CreateActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "repo", "owner": "o", "repo": "r", "name": "TOKEN", "data": "secret-value",
	}))
	text := toolText(t, result, err)
	if capture.uri != "/api/v1/repos/o/r/actions/variables/TOKEN" {
		t.Fatalf("uri %s", capture.uri)
	}
	if !strings.Contains(capture.body, `"value":"secret-value"`) {
		t.Fatalf("body %s", capture.body)
	}
	if strings.Contains(text, "secret-value") {
		t.Fatalf("create echoed the value: %s", text)
	}
	if !strings.Contains(text, `"status":"created"`) {
		t.Fatalf("status: %s", text)
	}
}

func TestUpdateActionVariableFn_Puts(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	result, err := UpdateActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "user", "name": "TOKEN", "data": "next",
	}))
	text := toolText(t, result, err)
	if capture.method != http.MethodPut || capture.uri != "/api/v1/user/actions/variables/TOKEN" {
		t.Fatalf("%s %s", capture.method, capture.uri)
	}
	if !strings.Contains(capture.body, `"value":"next"`) {
		t.Fatalf("body %s", capture.body)
	}
	if strings.Contains(text, "next") || !strings.Contains(text, `"status":"updated"`) {
		t.Fatalf("result %s", text)
	}
}

func TestDeleteActionVariableFn_Deletes(t *testing.T) {
	srv, capture := setupVariableMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	result, err := DeleteActionVariableFn(context.Background(), newCallToolRequest(map[string]any{
		"scope": "org", "org": "acme", "name": "TOKEN",
	}))
	text := toolText(t, result, err)
	if capture.uri != "/api/v1/orgs/acme/actions/variables/TOKEN" {
		t.Fatalf("uri %s", capture.uri)
	}
	if !strings.Contains(text, `"status":"deleted"`) {
		t.Fatalf("result %s", text)
	}
}
