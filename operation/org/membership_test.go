package org

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListOrgMembersFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "login": "user1"},
			{"id": 2, "login": "user2"},
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	result, err := ListOrgMembersFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListOrgMembersFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("ListOrgMembersFn returned tool error")
	}
}

func TestListOrgMembersFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := ListOrgMembersFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestCheckOrgMembershipFn_IsMember(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org":  "test-org",
		"user": "testuser",
	})
	result, err := CheckOrgMembershipFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckOrgMembershipFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("CheckOrgMembershipFn returned tool error")
	}
}

func TestCheckOrgMembershipFn_NotMember(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org":  "test-org",
		"user": "outsider",
	})
	result, err := CheckOrgMembershipFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckOrgMembershipFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("CheckOrgMembershipFn returned tool error")
	}
}

func TestCheckOrgMembershipFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"user": "testuser",
	})
	_, err := CheckOrgMembershipFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestCheckOrgMembershipFn_MissingUser(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	_, err := CheckOrgMembershipFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing user, got nil")
	}
}

func TestRemoveOrgMemberFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org":  "test-org",
		"user": "testuser",
	})
	result, err := RemoveOrgMemberFn(context.Background(), req)
	if err != nil {
		t.Fatalf("RemoveOrgMemberFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("RemoveOrgMemberFn returned tool error")
	}
}

func TestRemoveOrgMemberFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"user": "testuser",
	})
	_, err := RemoveOrgMemberFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestRemoveOrgMemberFn_MissingUser(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	_, err := RemoveOrgMemberFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing user, got nil")
	}
}
