package resource

import (
	"errors"
	"fmt"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

func TestMapForgejoError_403(t *testing.T) {
	err := MapForgejoError("forgejo://repo/a/b", errors.New("403 Forbidden"))
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32002 {
		t.Errorf("expected code -32002 for 403, got %d", err.Code)
	}
}

func TestMapForgejoError_404(t *testing.T) {
	err := MapForgejoError("forgejo://repo/a/b", errors.New("404 Not Found"))
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32003 {
		t.Errorf("expected code -32003 for 404, got %d", err.Code)
	}
}

func TestMapForgejoError_Nil(t *testing.T) {
	err := MapForgejoError("forgejo://repo/a/b", nil)
	if err != nil {
		t.Errorf("expected nil for nil input, got %v", err)
	}
}

func TestMapForgejoError_InvalidKind(t *testing.T) {
	// The real parser now wraps with ErrInvalidParams; simulate that here.
	wrapped := fmt.Errorf("%w: invalid URI \"forgejo://repo/a/b/wiki/1/comment/5\": kind must be 'issue' or 'pr', got \"wiki\"", ErrInvalidParams)
	err := MapForgejoError("forgejo://repo/a/b/wiki/1/comment/5", wrapped)
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32602 {
		t.Errorf("expected code -32602 for invalid kind, got %d", err.Code)
	}
}

func TestMapForgejoError_ErrInvalidParams_Sentinel(t *testing.T) {
	// Errors wrapping ErrInvalidParams must map to -32602 regardless of message content.
	wrapped := fmt.Errorf("%w: invalid URI \"forgejo://repo/x/y/commit/short\": sha must be exactly 40 hex characters, got 5", ErrInvalidParams)
	err := MapForgejoError("forgejo://repo/x/y/commit/short", wrapped)
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32602 {
		t.Errorf("expected code -32602 for ErrInvalidParams sentinel, got %d", err.Code)
	}
}

func TestMapForgejoError_ErrInvalidParams_404InMessage(t *testing.T) {
	// A URI containing "404" in its body must still map to -32602 when wrapped with ErrInvalidParams,
	// not to -32003 (the substring-match path must not fire before the sentinel check).
	wrapped := fmt.Errorf("%w: invalid URI containing 404 in path", ErrInvalidParams)
	err := MapForgejoError("forgejo://repo/a/b/issue/404abc", wrapped)
	if err == nil {
		t.Fatal("expected non-nil ResourceError")
	}
	if err.Code != -32602 {
		t.Errorf("expected code -32602 (sentinel beats 404 substring), got %d", err.Code)
	}
}

// Error text carries the request URL. A port, owner, or page name that contains
// "403" or "404" must not decide the classification — the typed status does.
// Regression for the flake on main 6c8199b: an httptest port of 40373 turned a
// wiki 404 into access denied.
func TestMapForgejoError_StatusBeatsDigitsInURL(t *testing.T) {
	const u = "http://127.0.0.1:40373/api/v1/repos/o403/r404/wiki/page/p"
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"HTTPError 404 with 403 in URL", &forgejo.HTTPError{StatusCode: 404, Status: "404 Not Found", Method: "GET", URL: u}, -32003},
		{"HTTPError 403 with 404 in URL", &forgejo.HTTPError{StatusCode: 403, Status: "403 Forbidden", Method: "GET", URL: u}, -32002},
		{"HTTPError 500 with both in URL", &forgejo.HTTPError{StatusCode: 500, Status: "500 Internal Server Error", Method: "GET", URL: u}, -32603},
		{"wrapped HTTPError 404", fmt.Errorf("get wiki page: %w", &forgejo.HTTPError{StatusCode: 404, Status: "404 Not Found", Method: "GET", URL: u}), -32003},
		{"WithStatus 404", WithStatus(404, errors.New("GET "+u+": user does not exist")), -32003},
		{"WithStatus 403", WithStatus(403, errors.New("GET "+u)), -32002},
		{"WithStatus 502", WithStatus(502, errors.New("GET "+u+": 404 Not Found upstream")), -32603},
		{"no status, digits only in URL", errors.New(`Get "` + u + `": dial tcp: connection refused`), -32603},
		{"no status, ErrNotFound sentinel", fmt.Errorf("lookup %s: %w", u, forgejo.ErrNotFound), -32003},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapForgejoError("forgejo://repo/o403/r404/wiki/p", tc.err)
			if got.Code != tc.want {
				t.Errorf("code = %d, want %d (msg %q)", got.Code, tc.want, got.Message)
			}
		})
	}
}

func TestWithStatus_Nil(t *testing.T) {
	if err := WithStatus(404, nil); err != nil {
		t.Errorf("WithStatus(404, nil) = %v, want nil", err)
	}
}

func TestWithStatus_KeepsLegacyMessage(t *testing.T) {
	inner := errors.New("boom")
	err := WithStatus(404, inner)
	if err.Error() != "404 boom" {
		t.Errorf("Error() = %q, want %q", err.Error(), "404 boom")
	}
	if !errors.Is(err, inner) {
		t.Error("WithStatus must unwrap to the SDK error")
	}
}
