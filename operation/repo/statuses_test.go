// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGetCommitStatusesFn_ShortSHANoRequest(t *testing.T) {
	records := newRepoBackend(t, func(_ *http.ServeMux) {})
	_, err := GetCommitStatusesFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "sha": "abc123",
	}))
	if err == nil {
		t.Fatal("expected error for short sha")
	}
	if len(*records) != 0 {
		t.Errorf("expected zero upstream requests, got %+v", *records)
	}
}

func TestGetCommitStatusesFn_EmptyList(t *testing.T) {
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/commits/"+testSHA+"/statuses", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method: got %q, want GET", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		})
	})

	res, err := GetCommitStatusesFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "sha": testSHA,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}

	var envelope struct {
		Result getCommitStatusesResult `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	if envelope.Result.SHA != testSHA {
		t.Errorf("sha: got %q", envelope.Result.SHA)
	}
	if envelope.Result.Count != 0 {
		t.Errorf("count: got %d, want 0", envelope.Result.Count)
	}
	if envelope.Result.Statuses == nil {
		t.Fatal("statuses must be [] not null")
	}
	if len(envelope.Result.Statuses) != 0 {
		t.Errorf("statuses: got %v, want empty", envelope.Result.Statuses)
	}
	if envelope.Result.Page != 1 || envelope.Result.Limit != 30 {
		t.Errorf("page/limit: got page=%d limit=%d", envelope.Result.Page, envelope.Result.Limit)
	}
}

func TestGetCommitStatusesFn_SuccessPage(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/commits/"+testSHA+"/statuses", func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			gotQuery = r.URL.RawQuery
			if r.Method != http.MethodGet {
				t.Errorf("method: got %q, want GET", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{
					"id": 1,
					"status": "success",
					"target_url": "https://ci.example/1",
					"description": "ok",
					"context": "ci/woodpecker",
					"created_at": "2026-08-01T12:00:00Z"
				}
			]`))
		})
	})

	res, err := GetCommitStatusesFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "sha": testSHA,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method: got %q, want GET", gotMethod)
	}
	if gotPath != "/api/v1/repos/o/r/commits/"+testSHA+"/statuses" {
		t.Fatalf("path: got %q", gotPath)
	}
	if !strings.Contains(gotQuery, "page=1") {
		t.Errorf("query missing page=1: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=30") {
		t.Errorf("query missing limit=30: %q", gotQuery)
	}

	var envelope struct {
		Result getCommitStatusesResult `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	if envelope.Result.Count != 1 {
		t.Fatalf("count: got %d, want 1", envelope.Result.Count)
	}
	if len(envelope.Result.Statuses) != 1 {
		t.Fatalf("statuses: got %v", envelope.Result.Statuses)
	}
	item := envelope.Result.Statuses[0]
	if item.Context != "ci/woodpecker" {
		t.Errorf("context: got %q", item.Context)
	}
	if item.State != "success" {
		t.Errorf("state: got %q, want success", item.State)
	}
	if item.TargetURL != "https://ci.example/1" {
		t.Errorf("target_url: got %q", item.TargetURL)
	}
	if item.Description != "ok" {
		t.Errorf("description: got %q", item.Description)
	}
	text := extractText(t, res)
	if strings.Contains(text, `"status":"success"`) && !strings.Contains(text, `"state":"success"`) {
		t.Errorf("item must use state, not SDK status: %s", text)
	}
}

func TestGetCommitStatusesFn_NotFound(t *testing.T) {
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/commits/"+testSHA+"/statuses", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method: got %q, want GET", r.Method)
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		})
	})

	res, err := GetCommitStatusesFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "sha": testSHA,
	}))
	if err == nil {
		t.Fatalf("expected error for 404, got %v", res)
	}
}

func TestGetCommitStatusesFn_Page2(t *testing.T) {
	var gotQuery string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/commits/"+testSHA+"/statuses", func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			if r.Method != http.MethodGet {
				t.Errorf("method: got %q, want GET", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		})
	})

	res, err := GetCommitStatusesFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "sha": testSHA, "page": float64(2),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	if !strings.Contains(gotQuery, "page=2") {
		t.Errorf("query missing page=2: %q", gotQuery)
	}

	var envelope struct {
		Result getCommitStatusesResult `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	if envelope.Result.Page != 2 {
		t.Errorf("page: got %d, want 2", envelope.Result.Page)
	}
}
