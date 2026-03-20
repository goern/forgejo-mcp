package org

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestListOrgTeamsFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": 1, "name": "Owners", "permission": "owner"},
			{"id": 2, "name": "Developers", "permission": "write"},
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	result, err := ListOrgTeamsFn(context.Background(), req)
	if err != nil {
		t.Fatalf("ListOrgTeamsFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("ListOrgTeamsFn returned tool error")
	}
}

func TestListOrgTeamsFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{})
	_, err := ListOrgTeamsFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestCreateOrgTeamFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         5,
			"name":       "devs",
			"permission": "read",
		})
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"org":  "test-org",
		"name": "devs",
	})
	result, err := CreateOrgTeamFn(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateOrgTeamFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("CreateOrgTeamFn returned tool error")
	}
}

func TestCreateOrgTeamFn_MissingOrg(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"name": "devs",
	})
	_, err := CreateOrgTeamFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestCreateOrgTeamFn_MissingName(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"org": "test-org",
	})
	_, err := CreateOrgTeamFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestAddTeamMemberFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"team_id": float64(5),
		"user":    "testuser",
	})
	result, err := AddTeamMemberFn(context.Background(), req)
	if err != nil {
		t.Fatalf("AddTeamMemberFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("AddTeamMemberFn returned tool error")
	}
}

func TestAddTeamMemberFn_MissingTeamID(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"user": "testuser",
	})
	_, err := AddTeamMemberFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing team_id, got nil")
	}
}

func TestRemoveTeamMemberFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"team_id": float64(5),
		"user":    "testuser",
	})
	result, err := RemoveTeamMemberFn(context.Background(), req)
	if err != nil {
		t.Fatalf("RemoveTeamMemberFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("RemoveTeamMemberFn returned tool error")
	}
}

func TestAddTeamRepoFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"team_id": float64(5),
		"org":     "test-org",
		"repo":    "test-repo",
	})
	result, err := AddTeamRepoFn(context.Background(), req)
	if err != nil {
		t.Fatalf("AddTeamRepoFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("AddTeamRepoFn returned tool error")
	}
}

func TestAddTeamRepoFn_MissingTeamID(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"org":  "test-org",
		"repo": "test-repo",
	})
	_, err := AddTeamRepoFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing team_id, got nil")
	}
}

func TestRemoveTeamRepoFn_Success(t *testing.T) {
	srv := setupOrgMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	req := newCallToolRequest(map[string]interface{}{
		"team_id": float64(5),
		"org":     "test-org",
		"repo":    "test-repo",
	})
	result, err := RemoveTeamRepoFn(context.Background(), req)
	if err != nil {
		t.Fatalf("RemoveTeamRepoFn returned error: %v", err)
	}
	if result.IsError {
		t.Fatal("RemoveTeamRepoFn returned tool error")
	}
}

func TestRemoveTeamRepoFn_MissingRepo(t *testing.T) {
	req := newCallToolRequest(map[string]interface{}{
		"team_id": float64(5),
		"org":     "test-org",
	})
	_, err := RemoveTeamRepoFn(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing repo, got nil")
	}
}
