// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// milestoneHandler is labelHandler plus capture of the request body so tests
// can assert on the exact PATCH/POST payload.
func milestoneHandler(t *testing.T, method, path string, status int, body any, captured *[]byte) func(*http.ServeMux) {
	t.Helper()
	return func(mux *http.ServeMux) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if captured != nil {
				*captured, _ = io.ReadAll(r.Body)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if body != nil {
				json.NewEncoder(w).Encode(body)
			}
		})
	}
}

func TestCreateMilestoneFn_HappyPath(t *testing.T) {
	var got []byte
	ms := forgejo_sdk.Milestone{ID: 7, Title: "Phase 1", State: "open"}
	newLabelBackend(t, milestoneHandler(t, http.MethodPost, "/api/v1/repos/owner/repo/milestones", http.StatusCreated, ms, &got))

	res, err := CreateMilestoneFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "title": "Phase 1", "description": "first", "due_on": "2026-08-20T00:00:00Z",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("unexpected error: err=%v res=%+v", err, res)
	}
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("bad payload: %v", err)
	}
	if payload["title"] != "Phase 1" || payload["description"] != "first" || payload["state"] != "" && payload["state"] != "open" {
		t.Errorf("unexpected payload: %s", got)
	}
	if !strings.HasPrefix(payload["due_on"].(string), "2026-08-20") {
		t.Errorf("due_on not forwarded: %s", got)
	}
}

func TestCreateMilestoneFn_Rejects(t *testing.T) {
	newLabelBackend(t, func(*http.ServeMux) {})
	cases := []map[string]any{
		{"owner": "owner", "repo": "repo"},                                         // missing title
		{"owner": "owner", "repo": "repo", "title": "x", "state": "bogus"},         // bad state
		{"owner": "owner", "repo": "repo", "title": "x", "due_on": "next tuesday"}, // bad date
	}
	for _, args := range cases {
		if _, err := CreateMilestoneFn(context.Background(), makeReq(args)); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestGetMilestoneFn(t *testing.T) {
	ms := forgejo_sdk.Milestone{ID: 3, Title: "v1"}
	newLabelBackend(t, milestoneHandler(t, http.MethodGet, "/api/v1/repos/owner/repo/milestones/3", http.StatusOK, ms, nil))
	res, err := GetMilestoneFn(context.Background(), makeReq(map[string]any{"owner": "owner", "repo": "repo", "id": float64(3)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("unexpected error: err=%v res=%+v", err, res)
	}
}

func TestEditMilestoneFn_PartialPatch(t *testing.T) {
	var got []byte
	ms := forgejo_sdk.Milestone{ID: 3, Title: "v1", State: "closed"}
	newLabelBackend(t, milestoneHandler(t, http.MethodPatch, "/api/v1/repos/owner/repo/milestones/3", http.StatusOK, ms, &got))

	res, err := EditMilestoneFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(3), "state": "closed",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("unexpected error: err=%v res=%+v", err, res)
	}
	var payload map[string]any
	_ = json.Unmarshal(got, &payload)
	// Only state is set; title stays empty and description/due_on stay null so the server leaves them alone.
	if payload["state"] != "closed" || payload["title"] != "" || payload["description"] != nil || payload["due_on"] != nil {
		t.Errorf("unexpected payload: %s", got)
	}
}

func TestEditMilestoneFn_Rejects(t *testing.T) {
	newLabelBackend(t, func(*http.ServeMux) {})
	cases := []map[string]any{
		{"owner": "owner", "repo": "repo", "id": float64(3)}, // nothing to change
		{"owner": "owner", "repo": "repo", "id": float64(3), "state": "bogus"},
	}
	for _, args := range cases {
		if _, err := EditMilestoneFn(context.Background(), makeReq(args)); err == nil {
			t.Errorf("expected error for %v", args)
		}
	}
}

func TestDeleteMilestoneFn(t *testing.T) {
	newLabelBackend(t, milestoneHandler(t, http.MethodDelete, "/api/v1/repos/owner/repo/milestones/3", http.StatusNoContent, nil, nil))
	res, err := DeleteMilestoneFn(context.Background(), makeReq(map[string]any{"owner": "owner", "repo": "repo", "id": float64(3)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("unexpected error: err=%v res=%+v", err, res)
	}
}
