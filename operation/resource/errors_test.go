package resource

import (
	"errors"
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
