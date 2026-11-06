// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

const (
	movedPath  = "/api/v1/repos/old/r/forks"
	targetPath = "/api/v1/repos/new/r/forks"
)

// redirectingForge answers movedPath with a redirect to targetPath, the way
// Forgejo answers a renamed repository. The target answers GET with the array
// Forgejo returns for a fork listing, and anything else with one repository.
// It records how many requests reached the target, and the last method and
// body seen there.
type redirectingForge struct {
	srv    *httptest.Server
	hits   atomic.Int32
	method atomic.Value
	body   atomic.Value
}

func newRedirectingForge(t *testing.T, status int) *redirectingForge {
	t.Helper()
	f := &redirectingForge{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	mux.HandleFunc(movedPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetPath, status)
	})
	mux.HandleFunc(targetPath, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.hits.Add(1)
		f.method.Store(r.Method)
		f.body.Store(string(body))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`{"full_name":"me/r"}`))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	useForge(t, f.srv.URL)
	return f
}

// useForge points the package at url and rebuilds the SDK client, so a test
// goes through the production HTTP clients.
func useForge(t *testing.T, url string) {
	t.Helper()
	savedURL, savedToken := flag.URL, flag.Token
	flag.URL, flag.Token = url, "test-token"
	ResetClientForTesting()
	t.Cleanup(func() {
		flag.URL, flag.Token = savedURL, savedToken
		ResetClientForTesting()
		SetServerVersion("")
	})
}

func sdkClient(t *testing.T) *forgejo_sdk.Client {
	t.Helper()
	c, err := Client(context.Background())
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	return c
}

func assertRefused(t *testing.T, f *redirectingForge, err error, method string, status int) {
	t.Helper()
	if !errors.Is(err, ErrRedirectRefused) {
		t.Fatalf("err = %v, want ErrRedirectRefused", err)
	}
	if got := f.hits.Load(); got != 0 {
		t.Errorf("redirect target received %d requests, want 0", got)
	}
	for _, want := range []string{
		method + " " + f.srv.URL + movedPath,
		fmt.Sprintf("%d %s", status, http.StatusText(status)),
		f.srv.URL + targetPath,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
}

var methodChangingRedirects = []int{
	http.StatusMovedPermanently,
	http.StatusFound,
	http.StatusSeeOther,
}

func TestRawClientRefusesRedirectThatChangesMethod(t *testing.T) {
	for _, status := range methodChangingRedirects {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			t.Run(fmt.Sprintf("%s_%d", method, status), func(t *testing.T) {
				f := newRedirectingForge(t, status)
				var out []any
				err := DoJSON(context.Background(), method, "/repos/old/r/forks", map[string]string{"a": "b"}, &out)
				assertRefused(t, f, err, method, status)
			})
		}
	}
}

func TestSDKClientRefusesRedirectThatChangesMethod(t *testing.T) {
	for _, status := range methodChangingRedirects {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newRedirectingForge(t, status)
			_, _, err := sdkClient(t).CreateFork("old", "r", forgejo_sdk.CreateForkOption{})
			assertRefused(t, f, err, http.MethodPost, status)
		})
	}
}

func TestPerRequestSDKClientRefusesRedirectThatChangesMethod(t *testing.T) {
	f := newRedirectingForge(t, http.StatusMovedPermanently)
	ctx := WithToken(context.Background(), "caller-token")
	c, err := Client(ctx)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	_, _, err = c.CreateFork("old", "r", forgejo_sdk.CreateForkOption{})
	assertRefused(t, f, err, http.MethodPost, http.StatusMovedPermanently)
}

func TestMultipartRefusesRedirectThatChangesMethod(t *testing.T) {
	f := newRedirectingForge(t, http.StatusFound)
	err := DoMultipart(context.Background(), http.MethodPost, "/repos/old/r/forks",
		"attachment", "a.txt", "text/plain", strings.NewReader("data"), nil)
	assertRefused(t, f, err, http.MethodPost, http.StatusFound)
}

// A 307 or 308 keeps the method and body, so the write reaches its target.
func TestWriteFollowsMethodPreservingRedirect(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(fmt.Sprintf("raw_%d", status), func(t *testing.T) {
			f := newRedirectingForge(t, status)
			var out struct {
				FullName string `json:"full_name"`
			}
			if err := DoJSON(context.Background(), http.MethodPost, "/repos/old/r/forks", map[string]string{"a": "b"}, &out); err != nil {
				t.Fatalf("DoJSON: %v", err)
			}
			if out.FullName != "me/r" {
				t.Errorf("full_name = %q", out.FullName)
			}
			if got := f.method.Load(); got != http.MethodPost {
				t.Errorf("target saw %v, want POST", got)
			}
			if got := f.body.Load(); got != `{"a":"b"}` {
				t.Errorf("target body = %v", got)
			}
		})
		t.Run(fmt.Sprintf("sdk_%d", status), func(t *testing.T) {
			f := newRedirectingForge(t, status)
			fork, _, err := sdkClient(t).CreateFork("old", "r", forgejo_sdk.CreateForkOption{})
			if err != nil {
				t.Fatalf("CreateFork: %v", err)
			}
			if fork.FullName != "me/r" {
				t.Errorf("full_name = %q", fork.FullName)
			}
			if got := f.method.Load(); got != http.MethodPost {
				t.Errorf("target saw %v, want POST", got)
			}
		})
	}
}

// A streamed multipart body cannot be replayed, so net/http does not follow
// the redirect and the caller gets the 307 as an error, not a lost upload.
func TestMultipartReportsUnreplayableRedirect(t *testing.T) {
	f := newRedirectingForge(t, http.StatusTemporaryRedirect)
	err := DoMultipart(context.Background(), http.MethodPost, "/repos/old/r/forks",
		"attachment", "a.txt", "text/plain", strings.NewReader("data"), nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("err = %v, want HTTPError 307", err)
	}
	if got := f.hits.Load(); got != 0 {
		t.Errorf("redirect target received %d requests, want 0", got)
	}
}

// A 307 that was followed does not excuse a later 301.
func TestRefusesMethodChangeLaterInRedirectChain(t *testing.T) {
	f := newRedirectingForge(t, http.StatusMovedPermanently)
	first := "/api/v1/repos/older/r/forks"
	mux := http.NewServeMux()
	mux.HandleFunc(first, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.srv.URL+movedPath, http.StatusTemporaryRedirect)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL

	err := DoJSON(context.Background(), http.MethodPost, "/repos/older/r/forks", map[string]string{"a": "b"}, nil)
	assertRefused(t, f, err, http.MethodPost, http.StatusMovedPermanently)
}

func TestReadsFollowRedirects(t *testing.T) {
	for _, status := range methodChangingRedirects {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newRedirectingForge(t, status)
			var out []any
			if err := DoJSON(context.Background(), http.MethodGet, "/repos/old/r/forks", nil, &out); err != nil {
				t.Fatalf("DoJSON: %v", err)
			}
			if got := f.hits.Load(); got != 1 {
				t.Errorf("redirect target received %d requests, want 1", got)
			}
		})
	}
}

// A custom CheckRedirect replaces net/http's limit, so a loop must still end.
func TestRedirectLoopStops(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) > 2*maxRedirects {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	useForge(t, srv.URL)

	err := DoJSON(context.Background(), http.MethodGet, "/loop", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("err = %v, want the redirect limit", err)
	}
	if got := hits.Load(); got != maxRedirects {
		t.Errorf("server received %d requests, want %d", got, maxRedirects)
	}
}
