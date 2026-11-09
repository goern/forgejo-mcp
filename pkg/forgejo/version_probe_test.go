// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// signInForge stands in for a Forgejo instance whose /api/v1/version and
// /swagger.v1.json answers are set per test. It counts requests to each and
// requests that carried an Authorization header.
type signInForge struct {
	srv             *httptest.Server
	versionStatus   int
	swagger         http.HandlerFunc // nil: 404
	versionRequests atomic.Int64
	swaggerRequests atomic.Int64
	authorized      atomic.Int64
}

// newSignInForge starts the stub under prefix (a ROOT_URL sub-path, or "")
// and points flag.URL at it.
func newSignInForge(t *testing.T, prefix string, versionStatus int, swagger http.HandlerFunc) *signInForge {
	t.Helper()
	f := &signInForge{versionStatus: versionStatus, swagger: swagger}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			f.authorized.Add(1)
		}
		switch r.URL.Path {
		case prefix + "/api/v1/version":
			f.versionRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.versionStatus)
			if f.versionStatus == http.StatusOK {
				_, _ = w.Write([]byte(`{"version":"16.0.7+gitea-1.22.0"}`))
			} else {
				_, _ = w.Write([]byte(`{"message":"Only signed in user is allowed to call APIs."}`))
			}
		case prefix + "/swagger.v1.json":
			f.swaggerRequests.Add(1)
			if f.swagger == nil {
				http.NotFound(w, r)
				return
			}
			f.swagger(w, r)
		case prefix + "/user/login":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Sign in</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)

	prevURL, prevToken := flag.URL, flag.Token
	flag.URL = f.srv.URL + prefix
	t.Cleanup(func() {
		flag.URL, flag.Token = prevURL, prevToken
		SetServerVersion("")
	})
	return f
}

// swaggerDoc serves an OpenAPI document shaped like Forgejo's: a few keys
// before info, then info, then a large paths object.
func swaggerDoc(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		info := `{"title":"Forgejo API.","version":"` + version + `"}`
		if version == "" {
			info = `{"title":"Forgejo API."}`
		}
		_, _ = fmt.Fprintf(w, `{"consumes":["application/json"],"produces":["application/json"],"schemes":["https","http"],"swagger":"2.0","info":%s,"basePath":"/api/v1","paths":{"/version":{"get":{}}}}`, info)
	}
}

// Spec forgejo-version-detection, scenario "Version endpoint open".
func TestProbeVersionEndpointOpenSkipsSwagger(t *testing.T) {
	f := newSignInForge(t, "", http.StatusOK, swaggerDoc("99.0.0"))

	got, err := ProbeServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if got != "16.0.7+gitea-1.22.0" {
		t.Fatalf("version = %q, want 16.0.7+gitea-1.22.0", got)
	}
	if n := f.swaggerRequests.Load(); n != 0 {
		t.Fatalf("swagger requested %d times, want 0", n)
	}
}

// Spec forgejo-version-detection, scenario "Instance requires sign-in, swagger
// published", for both refusal statuses.
func TestProbeFallsBackToSwaggerOnSignIn(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := newSignInForge(t, "", status, swaggerDoc("15.0.11+gitea-1.22.0"))

			got, err := ProbeServerVersion(context.Background())
			if err != nil {
				t.Fatalf("ProbeServerVersion: %v", err)
			}
			if got != "15.0.11+gitea-1.22.0" {
				t.Fatalf("version = %q, want 15.0.11+gitea-1.22.0", got)
			}
			if f.versionRequests.Load() != 1 || f.swaggerRequests.Load() != 1 {
				t.Fatalf("requests: version %d, swagger %d; want 1 and 1", f.versionRequests.Load(), f.swaggerRequests.Load())
			}
		})
	}
}

// Spec forgejo-version-detection, scenario "Instance requires sign-in, swagger
// disabled". Forgejo with ENABLE_SWAGGER=false answers /swagger.v1.json with a
// 303 to an HTML page.
func TestProbeReportsSignInRequiredWithoutSwagger(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"303 to HTML": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/swagger.v1.json")+"/user/login", http.StatusSeeOther)
		},
		"404":      nil,
		"non-JSON": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html></html>")) },
		"JSON array": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`["info"]`))
		},
		"no info":            func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"swagger":"2.0"}`)) },
		"empty info.version": swaggerDoc(""),
	}
	for name, swagger := range cases {
		t.Run(name, func(t *testing.T) {
			newSignInForge(t, "", http.StatusForbidden, swagger)

			got, err := ProbeServerVersion(context.Background())
			if !errors.Is(err, ErrSignInRequired) {
				t.Fatalf("ProbeServerVersion = %q, %v; want ErrSignInRequired", got, err)
			}
		})
	}
}

// Spec forgejo-version-detection, scenario "Forgejo not reachable or not
// Forgejo".
func TestProbeDoesNotTrySwaggerWhenForgejoIsNotUsable(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f := newSignInForge(t, "", status, swaggerDoc("16.0.7"))

			_, err := ProbeServerVersion(context.Background())
			if err == nil || errors.Is(err, ErrSignInRequired) {
				t.Fatalf("err = %v, want a failure that is not ErrSignInRequired", err)
			}
			if n := f.swaggerRequests.Load(); n != 0 {
				t.Fatalf("swagger requested %d times, want 0", n)
			}
		})
	}

	t.Run("connection refused", func(t *testing.T) {
		f := newSignInForge(t, "", http.StatusOK, nil)
		f.srv.Close()

		_, err := ProbeServerVersion(context.Background())
		if err == nil || errors.Is(err, ErrSignInRequired) {
			t.Fatalf("err = %v, want a failure that is not ErrSignInRequired", err)
		}
	})
}

// Spec forgejo-version-detection, scenario "No credential is sent".
func TestProbeSendsNoCredentialOnEitherRequest(t *testing.T) {
	f := newSignInForge(t, "", http.StatusForbidden, swaggerDoc("16.0.7"))
	flag.Token = decoyOperatorCredential

	if _, err := ProbeServerVersion(context.Background()); err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if f.swaggerRequests.Load() != 1 {
		t.Fatal("swagger was not requested")
	}
	if n := f.authorized.Load(); n != 0 {
		t.Fatalf("%d requests carried an Authorization header, want 0", n)
	}
}

// Spec forgejo-version-detection, scenario "Oversized swagger document".
func TestProbeBoundsTheSwaggerRead(t *testing.T) {
	var written atomic.Int64
	huge := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"paths":{"x":"`))
		chunk := []byte(strings.Repeat("a", 64<<10))
		// Twice the bound, then the info the probe must never reach.
		for i := 0; i < 2*maxSwaggerBody/len(chunk); i++ {
			n, err := w.Write(chunk)
			written.Add(int64(n))
			if err != nil {
				return
			}
		}
		_, _ = w.Write([]byte(`"},"info":{"version":"16.0.7"}}`))
	}
	newSignInForge(t, "", http.StatusForbidden, huge)

	got, err := ProbeServerVersion(context.Background())
	if !errors.Is(err, ErrSignInRequired) {
		t.Fatalf("ProbeServerVersion = %q, %v; want ErrSignInRequired", got, err)
	}
}

// A ROOT_URL sub-path applies to swagger too: Forgejo serves it under the
// instance root, not under /api/v1.
func TestProbeFindsSwaggerUnderASubPath(t *testing.T) {
	f := newSignInForge(t, "/forgejo", http.StatusForbidden, swaggerDoc("16.0.7"))

	got, err := ProbeServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if got != "16.0.7" || f.swaggerRequests.Load() != 1 {
		t.Fatalf("version = %q, swagger requests %d; want 16.0.7 and 1", got, f.swaggerRequests.Load())
	}
}
