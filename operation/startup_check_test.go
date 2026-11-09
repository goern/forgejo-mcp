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
	"go.uber.org/zap/zapcore"
)

// startupForge stands in for Forgejo at startup. /api/v1/version answers
// anonStatus without a credential and 200 with one; /swagger.v1.json is
// served only when swaggerVersion is set.
type startupForge struct {
	url             string
	anonStatus      int
	anonHTML        bool // refuse anonymous requests with an HTML page, as a proxy or WAF does
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
				if f.anonHTML {
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(f.anonStatus)
					_, _ = w.Write([]byte("<html><body>Access denied</body></html>"))
					return
				}
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
// A version setting is recorded for the SDK, as initConfig does.
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
	forgejo.SetServerVersion(version)
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
// one anonymous reachability request, no swagger, and the version reaches the
// SDK.
func TestStartupWithTheVersionSettingOnlyChecksReachability(t *testing.T) {
	f := newStartupForge(t, http.StatusForbidden, "")
	withStartupConfig(t, f.url, "", "16.0.7")
	logs := captureLogs(t, zapcore.InfoLevel)

	if err := checkForgejoAtStartup(context.Background(), "http"); err != nil {
		t.Fatalf("startup refused: %v", err)
	}
	if n := f.versionRequests.Load(); n != 1 {
		t.Fatalf("%d version requests, want 1 (the reachability check)", n)
	}
	if n := f.swaggerRequests.Load(); n != 0 {
		t.Fatalf("swagger requested %d times, want 0", n)
	}
	if f.authorized.Load() != 0 {
		t.Fatal("the reachability check carried a credential")
	}
	stated := logs.FilterMessage("Using the Forgejo version from the configuration").All()
	if len(stated) != 1 || stated[0].ContextMap()["source"] != forgejoVersionSource {
		t.Fatalf("log lines naming the stated version: %v", stated)
	}
	// An SDK client built now must not ask for the version itself.
	ctx := context.WithValue(context.Background(), forgejo.TokenContextKey, "caller-test-value")
	if _, err := forgejo.Client(ctx); err != nil {
		t.Fatalf("Client: %v", err)
	}
	if n := f.versionRequests.Load(); n != 1 {
		t.Fatalf("the SDK requested the version %d more times, want 0", n-1)
	}
}

// Spec stateless-http-auth, scenario "Wrong URL still fails fast", with the
// version setting: stating the version does not skip the reachability check.
func TestStartupWithTheVersionSettingRefusesAForgejoItCannotReach(t *testing.T) {
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		dead := srv.URL
		srv.Close()
		withStartupConfig(t, dead, "", "16.0.7")
		if err := checkForgejoAtStartup(context.Background(), "http"); err == nil {
			t.Fatal("startup accepted an unreachable Forgejo")
		}
	})
	t.Run("404", func(t *testing.T) {
		f := newStartupForge(t, http.StatusNotFound, "")
		withStartupConfig(t, f.url, "", "16.0.7")
		if err := checkForgejoAtStartup(context.Background(), "sse"); err == nil {
			t.Fatal("startup accepted a URL that answers 404")
		}
	})
}

// A 401 or 403 that is not a Forgejo API error -- a proxy, a WAF, another
// service -- is not "Forgejo requires sign-in", with or without the setting.
func TestStartupRefusesARefusalThatIsNotFromForgejo(t *testing.T) {
	for _, version := range []string{"", "16.0.7"} {
		t.Run("setting "+version, func(t *testing.T) {
			f := newStartupForge(t, http.StatusForbidden, "16.0.7")
			f.anonHTML = true
			withStartupConfig(t, f.url, "", version)

			err := checkForgejoAtStartup(context.Background(), "http")
			if err == nil || !strings.Contains(err.Error(), "not a Forgejo API error") {
				t.Fatalf("err = %v, want a refusal saying the answer is not from Forgejo", err)
			}
			if f.swaggerRequests.Load() != 0 {
				t.Fatal("swagger was tried after a refusal that is not from Forgejo")
			}
		})
	}
}

// Spec forgejo-version-detection, scenario "Stated version differs".
func TestStartupWarnsWhenForgejoReportsAnotherVersion(t *testing.T) {
	f := newStartupForge(t, http.StatusOK, "")
	withStartupConfig(t, f.url, "", "16.0.6")
	logs := captureLogs(t, zapcore.InfoLevel)

	if err := checkForgejoAtStartup(context.Background(), "http"); err != nil {
		t.Fatalf("startup refused: %v", err)
	}
	warned := logs.FilterLevelExact(zapcore.WarnLevel).All()
	if len(warned) != 1 || warned[0].ContextMap()["reported_version"] != "16.0.7+gitea-1.22.0" {
		t.Fatalf("warnings = %v, want one naming the reported version", warned)
	}

	// Build metadata alone is not a difference.
	f2 := newStartupForge(t, http.StatusOK, "")
	withStartupConfig(t, f2.url, "", "16.0.7")
	logs = captureLogs(t, zapcore.InfoLevel)
	if err := checkForgejoAtStartup(context.Background(), "http"); err != nil {
		t.Fatalf("startup refused: %v", err)
	}
	if n := logs.FilterLevelExact(zapcore.WarnLevel).Len(); n != 0 {
		t.Fatalf("%d warnings for 16.0.7 against 16.0.7+gitea-1.22.0, want 0", n)
	}
}
