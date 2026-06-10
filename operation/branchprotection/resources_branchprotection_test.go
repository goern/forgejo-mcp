package branchprotection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func setupBPResourceServer(t *testing.T, status int, body interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func readResourceReq(uri string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}}
}

func resourceText(t *testing.T, contents []mcp.ResourceContents) string {
	t.Helper()
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	tc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}
	return tc.Text
}

func TestBranchProtectionsResource_HappyPath(t *testing.T) {
	srv := setupBPResourceServer(t, http.StatusOK, []map[string]interface{}{
		{"rule_name": "main", "branch_name": "main", "required_approvals": 1},
		{"rule_name": "release/*", "branch_name": "release/*"},
	})
	defer srv.Close()

	contents, err := branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload bpCollectionPayload
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TotalCount != 2 || len(payload.BranchProtections) != 2 {
		t.Errorf("expected 2 rules, got total=%d shown=%d", payload.TotalCount, len(payload.BranchProtections))
	}
	if payload.Truncated {
		t.Error("did not expect truncation for 2 rules")
	}
}

func TestBranchProtectionsResource_Truncation(t *testing.T) {
	rules := make([]map[string]interface{}, resource.EmbeddedListCap+1)
	for i := range rules {
		rules[i] = map[string]interface{}{"rule_name": fmt.Sprintf("rule-%d", i), "branch_name": fmt.Sprintf("b-%d", i)}
	}
	srv := setupBPResourceServer(t, http.StatusOK, rules)
	defer srv.Close()

	contents, err := branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload bpCollectionPayload
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if !payload.Truncated {
		t.Error("expected truncation when over EmbeddedListCap")
	}
	if len(payload.BranchProtections) != resource.EmbeddedListCap {
		t.Errorf("expected %d shown rules, got %d", resource.EmbeddedListCap, len(payload.BranchProtections))
	}
	if payload.ListTool != "list_branch_protections" {
		t.Errorf("expected list_tool sentinel, got %q", payload.ListTool)
	}
}

func TestBranchProtectionsResource_NotFound(t *testing.T) {
	srv := setupBPResourceServer(t, http.StatusNotFound, map[string]string{"message": "Not Found"})
	defer srv.Close()

	_, err := branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/missing/branch_protections"))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var re *resource.ResourceError
	ok := errors.As(err, &re)
	if !ok {
		t.Fatalf("expected *resource.ResourceError, got %T", err)
	}
	if re.Code != -32003 {
		t.Errorf("expected code -32003 (not found), got %d", re.Code)
	}
}

func TestBranchProtectionResource_HappyPath(t *testing.T) {
	srv := setupBPResourceServer(t, http.StatusOK, map[string]interface{}{
		"rule_name": "main", "branch_name": "main",
		"status_check_contexts": []string{"ci/build"}, "required_approvals": 2,
	})
	defer srv.Close()

	contents, err := branchProtectionResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protection/main"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var bp forgejo_sdk.BranchProtection
	if err := json.Unmarshal([]byte(resourceText(t, contents)), &bp); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if bp.RuleName != "main" || bp.RequiredApprovals != 2 || len(bp.StatusCheckContexts) != 1 {
		t.Errorf("unexpected protection state: %+v", bp)
	}
}

func TestBranchProtectionResource_MalformedURI(t *testing.T) {
	// No server needed: parse fails before any client call.
	_, err := branchProtectionResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protection"))
	if err == nil {
		t.Fatal("expected error for malformed URI (missing rule)")
	}
	var re *resource.ResourceError
	ok := errors.As(err, &re)
	if !ok {
		t.Fatalf("expected *resource.ResourceError, got %T", err)
	}
	if re.Code != -32602 {
		t.Errorf("expected code -32602 (invalid params), got %d", re.Code)
	}
}
