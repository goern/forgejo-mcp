package resource

import (
	"errors"
	"fmt"
	"testing"
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
