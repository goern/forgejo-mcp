package org

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

func setupOrgMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

func TestCreateOrgFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       1,
			"username": "test-org",
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"name": "test-org",
	})
	result, err := CreateOrgFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateOrgFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("CreateOrgFn returned tool error")
	}
}

func TestCreateOrgFn_MissingName(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := CreateOrgFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestGetOrgFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       1,
			"username": "test-org",
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	result, err := GetOrgFn(context.Background(), req)
	if err != nil {
		t.Fatalf("GetOrgFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("GetOrgFn returned tool error")
	}
}

func TestGetOrgFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := GetOrgFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestListMyOrgsFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "username": "org1"},
			{"id": 2, "username": "org2"},
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"page":  float64(1),
		"limit": float64(10),
	})
	result, err := ListMyOrgsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListMyOrgsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("ListMyOrgsFn returned tool error")
	}
}

func TestListUserOrgsFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "username": "org1"},
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"user": "testuser",
	})
	result, err := ListUserOrgsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListUserOrgsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("ListUserOrgsFn returned tool error")
	}
}

func TestListUserOrgsFn_MissingUser(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := ListUserOrgsFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing user, got nil")
	}
}

func TestEditOrgFn_Success(t *testing.T) {
	callCount := 0
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PATCH" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// GET for the follow-up GetOrg
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          1,
			"username":    "test-org",
			"description": "updated",
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org":         "test-org",
		"description": "updated",
	})
	result, err := EditOrgFn(context.Background(), req)
	if err != nil {
		t.Fatalf("EditOrgFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("EditOrgFn returned tool error")
	}
}

func TestEditOrgFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := EditOrgFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestDeleteOrgFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	result, err := DeleteOrgFn(context.Background(), req)
	if err != nil {
		t.Fatalf("DeleteOrgFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("DeleteOrgFn returned tool error")
	}
}

func TestDeleteOrgFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := DeleteOrgFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}
