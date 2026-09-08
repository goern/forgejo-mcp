// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

type recordedReq struct {
	method  string
	path    string
	rawBody []byte
}

func newRepoBackend(t *testing.T, muxFn func(*http.ServeMux)) *[]recordedReq {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	if muxFn != nil {
		muxFn(mux)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			rawBody: body,
		})
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "tkn"
	flag.UserAgent = "test"
	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("sdk client: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return &records
}

func TestEditRepoFn_DescriptionOnlyOmitsPrivate(t *testing.T) {
	var patchBody []byte
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"r","description":"only desc"}`))
		})
	})

	res, err := EditRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "description": "only desc",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("edit failed: err=%v res=%+v", err, res)
	}
	var body map[string]any
	if err := json.Unmarshal(patchBody, &body); err != nil {
		t.Fatalf("PATCH body: %v", err)
	}
	if body["description"] != "only desc" {
		t.Fatalf("description: got %v", body["description"])
	}
	if _, ok := body["private"]; ok {
		t.Fatalf("private must be omitted from description-only PATCH, body=%v", body)
	}
	if _, ok := body["archived"]; ok {
		t.Fatalf("archived must be omitted from description-only PATCH, body=%v", body)
	}
}

func TestEditRepoFn_PrivateFalseSent(t *testing.T) {
	var patchBody []byte
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"r","private":false}`))
		})
	})

	if _, err := EditRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "private": false,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(patchBody, &body); err != nil {
		t.Fatalf("PATCH body: %v", err)
	}
	v, ok := body["private"]
	if !ok {
		t.Fatal("private missing from PATCH body")
	}
	if v != false {
		t.Fatalf("private: got %v (%T), want false", v, v)
	}
}

func TestEditRepoFn_ArchivedTrueSent(t *testing.T) {
	var patchBody []byte
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"r","archived":true}`))
		})
	})

	if _, err := EditRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "archived": true,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(patchBody, &body); err != nil {
		t.Fatalf("PATCH body: %v", err)
	}
	v, ok := body["archived"]
	if !ok {
		t.Fatal("archived missing from PATCH body")
	}
	if v != true {
		t.Fatalf("archived: got %v (%T), want true", v, v)
	}
}

func TestEditRepoFn_EmptyEditRejected(t *testing.T) {
	records := newRepoBackend(t, func(_ *http.ServeMux) {})
	_, err := EditRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err == nil {
		t.Fatal("expected error when no setting fields are provided")
	}
	for _, rec := range *records {
		if rec.method == http.MethodPatch {
			t.Fatalf("PATCH must not be sent for empty edit, got %+v", rec)
		}
	}
}

func TestEditRepoFn_EmptyDescriptionClears(t *testing.T) {
	var patchBody []byte
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"r","description":""}`))
		})
	})

	if _, err := EditRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r", "description": "",
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(patchBody, &body); err != nil {
		t.Fatalf("PATCH body: %v", err)
	}
	v, ok := body["description"]
	if !ok {
		t.Fatal("description missing from PATCH body")
	}
	if v != "" {
		t.Fatalf("description: got %v, want empty string", v)
	}
}

func TestGetRepoFn_ReturnsWebsite(t *testing.T) {
	var gotMethod, gotPath string
	newRepoBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/o/r", func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"r","description":"d","website":"https://nesvet.dev"}`))
		})
	})

	res, err := GetRepoFn(context.Background(), newCallToolRequest(map[string]any{
		"owner": "o", "repo": "r",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("get failed: err=%v res=%+v", err, res)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method: got %q, want GET", gotMethod)
	}
	if gotPath != "/api/v1/repos/o/r" {
		t.Fatalf("path: got %q", gotPath)
	}
	text := extractText(t, res)
	if !strings.Contains(text, `"website":"https://nesvet.dev"`) && !strings.Contains(text, `"website": "https://nesvet.dev"`) {
		t.Fatalf("expected website in result, got %s", text)
	}
}
