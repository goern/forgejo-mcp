// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

func TestWithinInstance(t *testing.T) {
	cases := []struct {
		base, u string
		want    bool
	}{
		{"https://forge.example", "/attachments/u", true},
		{"https://forge.example", "attachments/u", true},
		{"https://forge.example", "https://forge.example/attachments/u", true},
		{"https://forge.example", "HTTPS://Forge.Example/attachments/u", true},
		{"https://forge.example", "https://forge.example:443/attachments/u", true},
		{"https://forge.example:443/", "https://forge.example/x", true},
		{"http://forge.example", "http://forge.example:80/x", true},
		{"https://forge.example", "https://evil.example/attachments/u", false},
		{"https://forge.example", "https://evil.forge.example/x", false},
		{"https://forge.example", "https://forge.example.evil/x", false},
		{"https://forge.example", "http://forge.example/x", false},
		{"https://forge.example", "https://forge.example:8443/x", false},
		{"https://forge.example", "//evil.example/x", false},
		{"https://forge.example", "https://user:pw@forge.example/x", false},
		{"https://forge.example", "ftp://forge.example/x", false},
		{"https://forge.example", "https://forge.example/a/../../x", false},
		{"https://h.example/forgejo", "https://h.example/forgejo/attachments/u", true},
		{"https://h.example/forgejo/", "https://h.example/forgejo", true},
		{"https://h.example/forgejo", "https://h.example/other/x", false},
		{"https://h.example/forgejo", "https://h.example/forgejo-evil/x", false},
		{"https://h.example/forgejo", "https://h.example/forgejo/../other", false},
		{"https://h.example/forgejo", "https://h.example/forgejo/%2e%2e/other", false},
		{"", "https://forge.example/x", false},
	}
	defer func(old string) { flag.URL = old }(flag.URL)
	for _, c := range cases {
		flag.URL = c.base
		if got := WithinInstance(c.u); got != c.want {
			t.Errorf("base %q, url %q: got %v, want %v", c.base, c.u, got, c.want)
		}
	}
}

// foreignHost records every request it receives and the Authorization header
// each carried. It stands in for the attacker's server.
type foreignHost struct {
	srv   *httptest.Server
	hits  atomic.Int32
	mu    sync.Mutex
	auths []string
}

func newForeignHost(t *testing.T) *foreignHost {
	t.Helper()
	f := &foreignHost{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.mu.Lock()
		f.auths = append(f.auths, r.Header.Get("Authorization"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("foreign"))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func TestDoRaw_ForeignAbsoluteURL_NoRequest(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusTeapot)
	})
	foreign := newForeignHost(t)

	_, _, err := DoRaw(context.Background(), foreign.srv.URL+"/steal")
	if !errors.Is(err, ErrCrossOrigin) {
		t.Fatalf("expected ErrCrossOrigin, got %v", err)
	}
	if n := foreign.hits.Load(); n != 0 {
		t.Fatalf("foreign host received %d request(s); want none", n)
	}
}

// The raw JSON helpers attach the token too, so they refuse the same way.
func TestDoJSON_ForeignAbsoluteURL_NoRequest(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) {
		w.WriteHeader(http.StatusTeapot)
	})
	foreign := newForeignHost(t)

	err := DoJSON(context.Background(), http.MethodGet, foreign.srv.URL+"/api/v1/x", nil, nil)
	if !errors.Is(err, ErrCrossOrigin) {
		t.Fatalf("expected ErrCrossOrigin, got %v", err)
	}
	if n := foreign.hits.Load(); n != 0 {
		t.Fatalf("foreign host received %d request(s); want none", n)
	}
}

// Both test servers listen on 127.0.0.1, so the redirect changes only the
// port — a case net/http forwards Authorization on by itself.
func TestDoRaw_RedirectOffInstance_DropsToken(t *testing.T) {
	foreign := newForeignHost(t)
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		http.Redirect(w, r, foreign.srv.URL+"/presigned", http.StatusFound)
	})

	body, _, err := DoRaw(context.Background(), "/attachments/u")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(body) != "foreign" {
		t.Fatalf("redirect should still be followed; body %q", body)
	}
	foreign.mu.Lock()
	defer foreign.mu.Unlock()
	if len(foreign.auths) != 1 || foreign.auths[0] != "" {
		t.Fatalf("token forwarded across origin: %q", foreign.auths)
	}
}

func TestDoRaw_RedirectWithinInstance_KeepsToken(t *testing.T) {
	var finalAuth atomic.Value
	newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		if r.URL.Path == "/attachments/old" {
			http.Redirect(w, r, "/attachments/new", http.StatusFound)
			return
		}
		finalAuth.Store(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("ok"))
	})

	if _, _, err := DoRaw(context.Background(), "/attachments/old"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, _ := finalAuth.Load().(string); got != "token test-token" {
		t.Fatalf("same-origin redirect lost the token: %q", got)
	}
}
