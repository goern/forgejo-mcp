// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// startupForge stands in for Forgejo at startup. /api/v1/version answers
// anonStatus without a credential and 200 with one; /swagger.v1.json is
// served only when swaggerVersion is set.
type startupForge struct {
	url             string
	anonStatus      int
	swaggerVersion  string
	versionRequests atomic.Int64
	swaggerRequests atomic.Int64
	authorized      atomic.Int64
}

func newStartupForge(t *testing.T, anonStatus int, swaggerVersion string) *startupForge {
	t.Helper()
	f := &startupForge{anonStatus: anonStatus, swaggerVersion: swaggerVersion}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorized := r.Header.Get("Authorization") != ""
		if authorized {
			f.authorized.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/version":
			f.versionRequests.Add(1)
			if !authorized && f.anonStatus != http.StatusOK {
				w.WriteHeader(f.anonStatus)
				_, _ = w.Write([]byte(`{"message":"Only signed in user is allowed to call APIs."}`))
				return
			}
			_, _ = w.Write([]byte(`{"version":"16.0.7+gitea-1.22.0"}`))
		case "/swagger.v1.json":
			f.swaggerRequests.Add(1)
			if f.swaggerVersion == "" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"swagger":"2.0","info":{"version":"` + f.swaggerVersion + `"},"paths":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	f.url = srv.URL
	return f
}

// withStartupConfig points the server at forgeURL in passthrough mode with
// the given operator token and version setting, and restores everything after.
func withStartupConfig(t *testing.T, forgeURL, token, version string) {
	t.Helper()
	prevURL, prevToken, prevVersion := flag.URL, flag.Token, flag.ForgejoVersion
	forgejo.ResetClientForTesting()
	t.Cleanup(func() {
		flag.URL, flag.Token, flag.ForgejoVersion = prevURL, prevToken, prevVersion
		forgejo.SetServerVersion("")
		forgejo.ResetClientForTesting()
	})
	flag.URL, flag.Token, flag.ForgejoVersion = forgeURL, token, version
	forgejo.SetServerVersion("")
}

// Spec stateless-http-auth, scenario "Forgejo requires sign-in", on both
// network transports and for both refusal statuses.
func TestStartupTokenlessStartsWhenForgejoRequiresSignIn(t *testing.T) {
	for _, transport := range []string{"http", "sse"} {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			t.Run(transport+" "+http.StatusText(status), func(t *testing.T) {
				f := newStartupForge(t, status, "")
				withStartupConfig(t, f.url, "", "")

				if err := checkForgejoAtStartup(context.Background(), transport); err != nil {
					t.Fatalf("startup refused: %v", err)
				}
				if f.swaggerRequests.Load() != 1 {
					t.Fatal("the swagger fallback was not tried")
				}
				if f.authorized.Load() != 0 {
					t.Fatal("a startup request carried a credential")
				}
			})
		}
	}
}

// Spec stateless-http-auth, scenario "Version read through the fallback".
func TestStartupTokenlessReadsTheVersionFromSwagger(t *testing.T) {
	f := newStartupForge(t, http.StatusForbidden, "16.0.7+gitea-1.22.0")
	withStartupConfig(t, f.url, "", "")

	if err := checkForgejoAtStartup(context.Background(), "http"); err != nil {
		t.Fatalf("startup refused: %v", err)
	}
	if f.swaggerRequests.Load() != 1 {
		t.Fatal("swagger was not read")
	}
}

// Spec stateless-http-auth, scenario "Wrong URL still fails fast".
func TestStartupTokenlessRefusesAForgejoItCannotUse(t *testing.T) {
	t.Run("404", func(t *testing.T) {
		f := newStartupForge(t, http.StatusNotFound, "16.0.7")
		withStartupConfig(t, f.url, "", "")
		err := checkForgejoAtStartup(context.Background(), "http")
		if err == nil || !strings.Contains(err.Error(), "connection test failed") {
			t.Fatalf("err = %v, want a refusal", err)
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		dead := srv.URL
		srv.Close()
		withStartupConfig(t, dead, "", "")
		if err := checkForgejoAtStartup(context.Background(), "sse"); err == nil {
			t.Fatal("startup accepted an unreachable Forgejo")
		}
	})
}

// Spec stateless-http-auth, scenario "Operator token configured": the
// authenticated test still runs, and its failure is fatal.
func TestStartupWithATokenKeepsTheAuthenticatedCheck(t *testing.T) {
	f := newStartupForge(t, http.StatusForbidden, "16.0.7")
	withStartupConfig(t, f.url, "operator-test-value", "")

	if err := checkForgejoAtStartup(context.Background(), "http"); err != nil {
		t.Fatalf("startup refused: %v", err)
	}
	if f.authorized.Load() == 0 {
		t.Fatal("the connection test did not authenticate")
	}
	if f.swaggerRequests.Load() != 0 {
		t.Fatal("the anonymous probe ran although a token is configured")
	}

	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	withStartupConfig(t, deadURL, "operator-test-value", "")
	if err := checkForgejoAtStartup(context.Background(), "http"); err == nil {
		t.Fatal("a failed authenticated check did not refuse startup")
	}
}

// Spec stateless-http-auth, scenario "stdio is unchanged".
func TestStartupStdioWithoutTokenStillFailsOnSignIn(t *testing.T) {
	f := newStartupForge(t, http.StatusForbidden, "16.0.7")
	withStartupConfig(t, f.url, "", "")

	err := checkForgejoAtStartup(context.Background(), "stdio")
	if err == nil || !strings.Contains(err.Error(), "connection test failed") {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if f.swaggerRequests.Load() != 0 {
		t.Fatal("stdio used the anonymous probe")
	}
}

// Spec forgejo-version-detection, scenario "Setting given", in passthrough:
// no probe, and the version reaches the SDK.
func TestStartupWithTheVersionSettingSkipsTheProbe(t *testing.T) {
	f := newStartupForge(t, http.StatusForbidden, "")
	withStartupConfig(t, f.url, "", "16.0.7")

	if err := checkForgejoAtStartup(context.Background(), "http"); err != nil {
		t.Fatalf("startup refused: %v", err)
	}
	if n := f.versionRequests.Load() + f.swaggerRequests.Load(); n != 0 {
		t.Fatalf("%d version requests, want 0", n)
	}
	// An SDK client built now must not ask for the version itself.
	ctx := context.WithValue(context.Background(), forgejo.TokenContextKey, "caller-test-value")
	if _, err := forgejo.Client(ctx); err != nil {
		t.Fatalf("Client: %v", err)
	}
	if n := f.versionRequests.Load(); n != 0 {
		t.Fatalf("the SDK requested the version %d times, want 0", n)
	}
}
