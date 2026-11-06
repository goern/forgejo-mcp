// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

const forksPath = "/api/v1/repos/o/r/forks"

// newForkBackend is newRepoBackend on the production HTTP client: redirect
// handling lives there, and newRepoBackend's client does not have it.
func newForkBackend(t *testing.T, muxFn func(*http.ServeMux)) {
	t.Helper()
	newRepoBackend(t, muxFn)
	forgejo.ResetClientForTesting()
	t.Cleanup(forgejo.ResetClientForTesting)
}

func callFork(t *testing.T) (*mcp.CallToolResult, error) {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"user": "o", "repo": "r"}
	return ForkRepoFn(context.Background(), req)
}

func TestForkRepoFn_ReturnsFork(t *testing.T) {
	newForkBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc(forksPath, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":7,"full_name":"me/r","html_url":"https://forge/me/r","fork":true}`))
		})
	})

	res, err := callFork(t)
	if err != nil {
		t.Fatalf("ForkRepoFn: %v", err)
	}
	var got struct {
		Result struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
			Fork     bool   `json:"fork"`
		} `json:"Result"`
	}
	text := res.Content[0].(mcp.TextContent).Text
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("decode %q: %v", text, err)
	}
	if got.Result.ID != 7 || got.Result.FullName != "me/r" || got.Result.HTMLURL != "https://forge/me/r" || !got.Result.Fork {
		t.Errorf("result = %s", text)
	}
}

// The reported failure: a renamed repository answers 301, and the POST must
// not arrive at the fork listing as a GET.
func TestForkRepoFn_RefusesRedirect(t *testing.T) {
	var targetHits int
	newForkBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc(forksPath, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/api/v1/repos/o/renamed/forks", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/api/v1/repos/o/renamed/forks", func(w http.ResponseWriter, _ *http.Request) {
			targetHits++
			_, _ = w.Write([]byte(`[]`))
		})
	})

	_, err := callFork(t)
	if !errors.Is(err, forgejo.ErrRedirectRefused) {
		t.Fatalf("err = %v, want ErrRedirectRefused", err)
	}
	if targetHits != 0 {
		t.Errorf("redirect target received %d requests, want 0", targetHits)
	}
	for _, want := range []string{"301 Moved Permanently", flag.URL + forksPath, "/api/v1/repos/o/renamed/forks"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
}

func TestForkRepoFn_ErrorCarriesStatusAndURL(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"forge error", http.StatusConflict, `{"message":"repository is already forked"}`, "409 Conflict"},
		{"body is not a repository", http.StatusOK, `[]`, "200 OK"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newForkBackend(t, func(mux *http.ServeMux) {
				mux.HandleFunc(forksPath, func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
				})
			})

			_, err := callFork(t)
			if err == nil {
				t.Fatal("ForkRepoFn succeeded, want an error")
			}
			for _, want := range []string{tc.want, "POST " + flag.URL + forksPath} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}
