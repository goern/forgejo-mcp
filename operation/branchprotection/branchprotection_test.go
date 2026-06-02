package branchprotection

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func setupBPMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestListBranchProtectionsFn(t *testing.T) {
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/branch_protections") {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"rule_name": "main", "branch_name": "main", "enable_status_check": true, "status_check_contexts": []string{"ci/build"}, "required_approvals": 1},
		})
	})
	defer srv.Close()

	res, err := ListBranchProtectionsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "page": float64(1), "limit": float64(50),
	}))
	if err != nil {
		t.Fatalf("ListBranchProtectionsFn err: %v", err)
	}
	text := toolText(t, res)
	if !strings.Contains(text, `"count":1`) || !strings.Contains(text, `"page":1`) {
		t.Errorf("expected count+page echo in result, got: %s", text)
	}
	if !strings.Contains(text, "ci/build") {
		t.Errorf("expected rule contexts in result, got: %s", text)
	}
}

func TestGetBranchProtectionFn_OK(t *testing.T) {
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/branch_protections/main") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"rule_name": "main", "branch_name": "main"})
	})
	defer srv.Close()

	res, err := GetBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "rule": "main",
	}))
	if err != nil {
		t.Fatalf("GetBranchProtectionFn err: %v", err)
	}
	if !strings.Contains(toolText(t, res), `"rule_name":"main"`) {
		t.Errorf("expected rule in result")
	}
}

func TestGetBranchProtectionFn_NotFound(t *testing.T) {
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	})
	defer srv.Close()

	_, err := GetBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "rule": "missing",
	}))
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestCreateBranchProtectionFn_StatusCheckRoundTrip(t *testing.T) {
	var body map[string]interface{}
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"rule_name": "main", "branch_name": "main",
			"enable_status_check": true, "status_check_contexts": []string{"ci/build", "ci/test"},
		})
	})
	defer srv.Close()

	res, err := CreateBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "branch_name": "main",
		"enable_status_check": true, "status_check_contexts": "ci/build, ci/test",
	}))
	if err != nil {
		t.Fatalf("CreateBranchProtectionFn err: %v", err)
	}
	if body["enable_status_check"] != true {
		t.Errorf("expected enable_status_check true in request body, got %v", body["enable_status_check"])
	}
	got := toStrings(body["status_check_contexts"])
	if len(got) != 2 || got[0] != "ci/build" || got[1] != "ci/test" {
		t.Errorf("status_check_contexts not round-tripped: %v", got)
	}
	if !strings.Contains(toolText(t, res), "ci/test") {
		t.Errorf("expected contexts echoed in result")
	}
}

func TestCreateBranchProtectionFn_MissingBranchName(t *testing.T) {
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("Forgejo must not be called when branch_name is missing (got %s %s)", r.Method, r.URL.Path)
	})
	defer srv.Close()

	_, err := CreateBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp",
	}))
	if err == nil {
		t.Fatal("expected error when branch_name is missing")
	}
}

func TestEditBranchProtectionFn_OnlyPassedFields(t *testing.T) {
	var raw []byte
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		raw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"rule_name": "main", "required_approvals": 2})
	})
	defer srv.Close()

	_, err := EditBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "rule": "main", "required_approvals": float64(2),
	}))
	if err != nil {
		t.Fatalf("EditBranchProtectionFn err: %v", err)
	}
	// Decode into a typed-agnostic map to inspect what was sent.
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if string(body["required_approvals"]) != "2" {
		t.Errorf("expected required_approvals=2, got %s", string(body["required_approvals"]))
	}
	// A field the caller did not pass must serialize as null (leave-unchanged),
	// NEVER as false (which would silently relax protection).
	if v, ok := body["enable_status_check"]; ok && string(v) != "null" {
		t.Errorf("unpassed enable_status_check must be null, got %s", string(v))
	}
}

func TestEditBranchProtectionFn_ContextsRoundTrip(t *testing.T) {
	var body map[string]interface{}
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		rawb, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(rawb, &body)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"rule_name": "main"})
	})
	defer srv.Close()

	_, err := EditBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "rule": "main",
		"status_check_contexts": "ci/build,ci/lint",
	}))
	if err != nil {
		t.Fatalf("EditBranchProtectionFn err: %v", err)
	}
	got := toStrings(body["status_check_contexts"])
	if len(got) != 2 || got[0] != "ci/build" || got[1] != "ci/lint" {
		t.Errorf("status_check_contexts not round-tripped: %v", got)
	}
}

func TestDeleteBranchProtectionFn_OK(t *testing.T) {
	called := false
	srv := setupBPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/branch_protections/main") {
			called = true
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	res, err := DeleteBranchProtectionFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "goern", "repo": "forgejo-mcp", "rule": "main",
	}))
	if err != nil {
		t.Fatalf("DeleteBranchProtectionFn err: %v", err)
	}
	if !called {
		t.Error("expected DELETE to be issued for the rule")
	}
	if !strings.Contains(toolText(t, res), "Deleted branch protection rule") {
		t.Errorf("expected delete confirmation, got: %s", toolText(t, res))
	}
}

func TestSplitContexts(t *testing.T) {
	got := splitContexts("ci/build, ci/test ,, ci/lint")
	want := []string{"ci/build", "ci/test", "ci/lint"}
	if len(got) != len(want) {
		t.Fatalf("splitContexts: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitContexts[%d]=%q want %q", i, got[i], want[i])
		}
	}
	if n := len(splitContexts("")); n != 0 {
		t.Errorf("empty string should split to 0 contexts, got %d", n)
	}
}

// toolText extracts the text payload from a tool result.
func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatal("nil tool result")
	}
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatal("no text content in tool result")
	return ""
}

// toStrings coerces a decoded JSON array (any) to []string.
func toStrings(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
