// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestListRepoTopicsFn_Envelope(t *testing.T) {
	var gotMethod, gotPath string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/topics", func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"topics":["go","mcp"]}`))
		})
	})

	res, err := ListRepoTopicsFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method: got %q, want GET", gotMethod)
	}
	if gotPath != "/api/v1/repos/o/r/topics" {
		t.Fatalf("path: got %q", gotPath)
	}

	var envelope struct {
		Result listRepoTopicsResult `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	if envelope.Result.Page != 1 || envelope.Result.Limit != 100 {
		t.Fatalf("page/limit: got page=%d limit=%d", envelope.Result.Page, envelope.Result.Limit)
	}
	if envelope.Result.Count != 2 {
		t.Fatalf("count: got %d, want 2", envelope.Result.Count)
	}
	if len(envelope.Result.Topics) != 2 || envelope.Result.Topics[0] != "go" || envelope.Result.Topics[1] != "mcp" {
		t.Fatalf("topics: got %v", envelope.Result.Topics)
	}
}

func TestSetRepoTopicsFn_NormalizesCSV(t *testing.T) {
	var putBody []byte
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/topics", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			putBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		})
	})

	if _, err := SetRepoTopicsFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "topics": "go, MCP",
	})); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	var body struct {
		Topics []string `json:"topics"`
	}
	if err := json.Unmarshal(putBody, &body); err != nil {
		t.Fatalf("PUT body: %v raw=%s", err, putBody)
	}
	if len(body.Topics) != 2 || body.Topics[0] != "go" || body.Topics[1] != "mcp" {
		t.Fatalf("PUT topics: got %v, want [go mcp]", body.Topics)
	}
}

func TestSetRepoTopicsFn_EmptyClears(t *testing.T) {
	var putBody []byte
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/topics", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			putBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		})
	})

	if _, err := SetRepoTopicsFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "topics": "",
	})); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	var body struct {
		Topics []string `json:"topics"`
	}
	if err := json.Unmarshal(putBody, &body); err != nil {
		t.Fatalf("PUT body: %v raw=%s", err, putBody)
	}
	if body.Topics == nil {
		t.Fatal("topics must be [] not null")
	}
	if len(body.Topics) != 0 {
		t.Fatalf("PUT topics: got %v, want empty", body.Topics)
	}
}

func TestSetRepoTopicsFn_MissingKeyNoPut(t *testing.T) {
	records := newRepoBackend(t, func(_ *http.ServeMux) {})
	_, err := SetRepoTopicsFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err == nil {
		t.Fatal("expected error when topics key is missing")
	}
	for _, rec := range *records {
		if rec.method == http.MethodPut {
			t.Fatalf("PUT must not be sent without topics key, got %+v", rec)
		}
	}
}

func TestAddRepoTopicFn_PutPath(t *testing.T) {
	var gotMethod, gotPath string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/topics/ci", func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		})
	})

	if _, err := AddRepoTopicFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "topic": "ci",
	})); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method: got %q, want PUT", gotMethod)
	}
	if gotPath != "/api/v1/repos/o/r/topics/ci" {
		t.Fatalf("path: got %q", gotPath)
	}
}

func TestDeleteRepoTopicFn_DeletePath(t *testing.T) {
	var gotMethod, gotPath string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/topics/ci", func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		})
	})

	if _, err := DeleteRepoTopicFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "topic": "ci",
	})); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method: got %q, want DELETE", gotMethod)
	}
	if gotPath != "/api/v1/repos/o/r/topics/ci" {
		t.Fatalf("path: got %q", gotPath)
	}
}

func TestAddRepoTopicFn_InvalidNameNoRequest(t *testing.T) {
	records := newRepoBackend(t, func(_ *http.ServeMux) {})
	_, err := AddRepoTopicFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "topic": "Bad Name",
	}))
	if err == nil {
		t.Fatal("expected error for invalid topic name")
	}
	if len(*records) != 0 {
		t.Fatalf("expected zero upstream requests, got %+v", *records)
	}
}

func TestSetRepoTopicsFn_TooManyNoPut(t *testing.T) {
	records := newRepoBackend(t, func(_ *http.ServeMux) {})
	names := make([]string, 26)
	for i := range names {
		names[i] = fmt.Sprintf("t%d", i)
	}
	_, err := SetRepoTopicsFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "topics": strings.Join(names, ","),
	}))
	if err == nil {
		t.Fatal("expected error for 26 topics")
	}
	for _, rec := range *records {
		if rec.method == http.MethodPut {
			t.Fatalf("PUT must not be sent for 26 topics, got %+v", rec)
		}
	}
}

func TestGetRepoFn_IncludesTopics(t *testing.T) {
	var methods []string
	var paths []string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r/topics", func(w http.ResponseWriter, r *http.Request) {
			methods = append(methods, r.Method)
			paths = append(paths, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"topics":["go","mcp"]}`))
		})
		mux.HandleFunc("/api/v1/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			methods = append(methods, r.Method)
			paths = append(paths, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"r","website":"https://nesvet.dev"}`))
		})
	})

	res, err := GetRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("get failed: err=%v res=%+v", err, res)
	}
	if len(paths) != 2 {
		t.Fatalf("expected GetRepo and ListRepoTopics, got methods=%v paths=%v", methods, paths)
	}
	sawRepo, sawTopics := false, false
	for i, p := range paths {
		if p == "/api/v1/repos/o/r" && methods[i] == http.MethodGet {
			sawRepo = true
		}
		if p == "/api/v1/repos/o/r/topics" && methods[i] == http.MethodGet {
			sawTopics = true
		}
	}
	if !sawRepo || !sawTopics {
		t.Fatalf("missing GetRepo or ListRepoTopics: methods=%v paths=%v", methods, paths)
	}

	var envelope struct {
		Result map[string]any `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	raw, ok := envelope.Result["topics"]
	if !ok {
		t.Fatalf("topics missing from get_repo JSON: %s", extractText(t, res))
	}
	got, ok := raw.([]any)
	if !ok {
		t.Fatalf("topics type: %T", raw)
	}
	if len(got) != 2 || got[0] != "go" || got[1] != "mcp" {
		t.Fatalf("topics: got %v", got)
	}
}
