package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeOwnerResourceRequest(owner string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://owner/" + owner,
		},
	}
}

// routingHandler routes /users/{login} and /orgs/{orgname} to separate handlers.
type routingHandler struct {
	userStatus int
	userBody   interface{}
	orgStatus  int
	orgBody    interface{}
}

func (h *routingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if strings.Contains(r.URL.Path, "/users/") {
		w.WriteHeader(h.userStatus)
		if h.userBody != nil {
			json.NewEncoder(w).Encode(h.userBody)
		}
		return
	}
	if strings.Contains(r.URL.Path, "/orgs/") {
		w.WriteHeader(h.orgStatus)
		if h.orgBody != nil {
			json.NewEncoder(w).Encode(h.orgBody)
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func setupOwnerMockServer(t *testing.T, h *routingHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestOwnerResourceHandler_HappyPath_User(t *testing.T) {
	h := &routingHandler{
		userStatus: http.StatusOK,
		userBody: map[string]interface{}{
			"login":           "alice",
			"full_name":       "Alice Smith",
			"html_url":        "https://codeberg.org/alice",
			"description":     "open source dev",
			"location":        "Berlin",
			"website":         "https://alice.dev",
			"created":         "2020-01-01T00:00:00Z",
			"followers_count": 10,
			"following_count": 5,
		},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("alice")
	contents, err := ownerResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	block := contents[0].(mcp.TextResourceContents)
	if block.MIMEType != "application/json" {
		t.Errorf("MIME: got %q, want application/json", block.MIMEType)
	}
	var payload ownerResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.Kind != "user" {
		t.Errorf("expected kind=user, got %q", payload.Kind)
	}
	if payload.Login != "alice" {
		t.Errorf("expected login=alice, got %q", payload.Login)
	}
}

func TestOwnerResourceHandler_HappyPath_OrgFallback(t *testing.T) {
	h := &routingHandler{
		userStatus: http.StatusNotFound,
		userBody:   map[string]string{"message": "user not found"},
		orgStatus:  http.StatusOK,
		orgBody: map[string]interface{}{
			"username":    "forgejo-org",
			"full_name":   "Forgejo Community",
			"description": "The Forgejo org",
			"location":    "Internet",
			"website":     "https://forgejo.org",
		},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("forgejo-org")
	contents, err := ownerResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	block := contents[0].(mcp.TextResourceContents)
	var payload ownerResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.Kind != "org" {
		t.Errorf("expected kind=org, got %q", payload.Kind)
	}
}

func TestOwnerResourceHandler_BothNotFound(t *testing.T) {
	h := &routingHandler{
		userStatus: http.StatusNotFound,
		userBody:   map[string]string{"message": "not found"},
		orgStatus:  http.StatusNotFound,
		orgBody:    map[string]string{"message": "not found"},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("nobody")
	_, err := ownerResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when both user and org return 404")
	}
}

func TestOwnerResourceHandler_UserForbidden(t *testing.T) {
	h := &routingHandler{
		userStatus: http.StatusForbidden,
		userBody:   map[string]string{"message": "Forbidden"},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("secretuser")
	_, err := ownerResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestOwnerResourceHandler_BothNotFound_Returns32003(t *testing.T) {
	h := &routingHandler{
		userStatus: http.StatusNotFound,
		userBody:   map[string]string{"message": "not found"},
		orgStatus:  http.StatusNotFound,
		orgBody:    map[string]string{"message": "not found"},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("ghost")
	_, err := ownerResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when both user and org return 404")
	}
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *resource.ResourceError, got %T: %v", err, err)
	}
	if resErr.Code != -32003 {
		t.Errorf("expected Code -32003, got %d", resErr.Code)
	}
	if !strings.Contains(resErr.Message, "owner not found") {
		t.Errorf("expected message to contain 'owner not found', got %q", resErr.Message)
	}
}

func TestOwnerResourceHandler_OrgTransportError_Surfaces_OrgErr(t *testing.T) {
	// User returns 404; org endpoint returns a 500 (non-404 error with body present).
	h := &routingHandler{
		userStatus: http.StatusNotFound,
		userBody:   map[string]string{"message": "not found"},
		orgStatus:  http.StatusInternalServerError,
		orgBody:    map[string]string{"message": "upstream timeout"},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("flaky-org")
	_, err := ownerResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when org returns 500")
	}
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *resource.ResourceError, got %T: %v", err, err)
	}
	// Must NOT be the original user 404 message — should contain org error signal.
	if strings.Contains(resErr.Message, "not found: ") && !strings.Contains(resErr.Message, "500") {
		t.Errorf("error appears to surface user 404 instead of org error: %q", resErr.Message)
	}
}

func TestOwnerResourceHandler_UserNilErr_OrgNotFound_Returns32003(t *testing.T) {
	// SDK edge: user endpoint returns 404 with empty body (u==nil, userErr may be nil
	// depending on SDK version). Org also 404 → must return explicit -32003, not (nil,nil).
	h := &routingHandler{
		userStatus: http.StatusNotFound,
		userBody:   nil, // empty body — SDK may return nil error
		orgStatus:  http.StatusNotFound,
		orgBody:    map[string]string{"message": "not found"},
	}
	srv := setupOwnerMockServer(t, h)
	defer srv.Close()

	req := makeOwnerResourceRequest("phantom")
	_, err := ownerResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, not (nil,nil), when both endpoints 404")
	}
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *resource.ResourceError, got %T: %v", err, err)
	}
	if resErr.Code != -32003 {
		t.Errorf("expected Code -32003, got %d", resErr.Code)
	}
}
