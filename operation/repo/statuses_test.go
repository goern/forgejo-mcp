// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
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
	if envelope.Result.TotalCount != nil {
		t.Errorf("total_count must be omitted when the header is absent, got %d", *envelope.Result.TotalCount)
	}
	if text := extractText(t, res); strings.Contains(text, "total_count") {
		t.Errorf("total_count must be omitted when the header is absent: %s", text)
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
	if item.CreatedAt != "2026-08-01T12:00:00Z" {
		t.Errorf("created_at: got %q", item.CreatedAt)
	}
	text := extractText(t, res)
	if !strings.Contains(text, `"state":"success"`) {
		t.Errorf("item must use state: %s", text)
	}
	if strings.Contains(text, `"status":"success"`) {
		t.Errorf("item must not use SDK status: %s", text)
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

func TestGetCommitStatusesFn_LimitClamp(t *testing.T) {
	cases := []struct {
		name  string
		limit float64
		query string
		echo  int
	}{
		{name: "over_max", limit: 999, query: "limit=50", echo: 50},
		{name: "under_min", limit: 0, query: "limit=30", echo: 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				"owner": "o", "repo": "r", "sha": testSHA, "limit": tc.limit,
			}))
			if err != nil || res == nil || res.IsError {
				t.Fatalf("list failed: err=%v res=%+v", err, res)
			}
			if !strings.Contains(gotQuery, tc.query) {
				t.Errorf("query missing %s: %q", tc.query, gotQuery)
			}

			var envelope struct {
				Result getCommitStatusesResult `json:"Result"`
			}
			if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
				t.Fatalf("result JSON: %v", err)
			}
			if envelope.Result.Limit != tc.echo {
				t.Errorf("limit: got %d, want %d", envelope.Result.Limit, tc.echo)
			}
		})
	}
}

func TestGetCommitStatusesFn_TotalCountFromHeader(t *testing.T) {
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/commits/"+testSHA+"/statuses", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method: got %q, want GET", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set(forgejo.TotalCountHeader, "4")
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
	if envelope.Result.TotalCount == nil || *envelope.Result.TotalCount != 4 {
		t.Fatalf("total_count: %+v", envelope.Result.TotalCount)
	}
}

func TestGetCommitStatusesFn_TotalCountZeroIsEmitted(t *testing.T) {
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/commits/"+testSHA+"/statuses", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method: got %q, want GET", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set(forgejo.TotalCountHeader, "0")
			_, _ = w.Write([]byte(`[]`))
		})
	})

	res, err := GetCommitStatusesFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "sha": testSHA,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	text := extractText(t, res)
	if !strings.Contains(text, `"total_count":0`) {
		t.Fatalf("expected a confirmed-zero total_count to be emitted, got: %s", text)
	}
}
