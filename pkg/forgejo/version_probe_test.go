// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// signInForge stands in for a Forgejo instance whose /api/v1/version and
// /swagger.v1.json answers are set per test. It counts requests to each and
// requests that carried an Authorization header.
type signInForge struct {
	srv             *httptest.Server
	versionStatus   int
	versionBody     string           // the body of a non-200 /version answer; "" means a Forgejo API error
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
			} else if f.versionBody != "" {
				_, _ = w.Write([]byte(f.versionBody))
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
	huge := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"paths":{"x":"`))
		chunk := []byte(strings.Repeat("a", 64<<10))
		// Twice the bound, then the info the probe must never reach.
		for i := 0; i < 2*maxSwaggerBody/len(chunk); i++ {
			if _, err := w.Write(chunk); err != nil {
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

// Spec forgejo-version-detection, scenario "Refusal that is not from
// Forgejo": a proxy, a WAF or another service refusing the request is not a
// closed Forgejo, so swagger is not tried and the outcome is not
// ErrSignInRequired.
func TestProbeTreatsANonForgejoRefusalAsNotUsable(t *testing.T) {
	bodies := map[string]string{
		"HTML":          "<html><body>403 Forbidden</body></html>",
		"plain text":    "Unauthorized",
		"empty message": `{"message":""}`,
		"JSON array":    `["message"]`,
		"empty":         " ",
	}
	for name, body := range bodies {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			t.Run(name+" "+http.StatusText(status), func(t *testing.T) {
				f := newSignInForge(t, "", status, swaggerDoc("16.0.7"))
				f.versionBody = body

				_, err := ProbeServerVersion(context.Background())
				if err == nil || errors.Is(err, ErrSignInRequired) {
					t.Fatalf("err = %v, want a failure that is not ErrSignInRequired", err)
				}
				if _, err := CheckReachable(context.Background()); err == nil {
					t.Fatal("CheckReachable accepted a refusal that is not from Forgejo")
				}
				if n := f.swaggerRequests.Load(); n != 0 {
					t.Fatalf("swagger requested %d times, want 0", n)
				}
			})
		}
	}
}

// Spec forgejo-version-detection, scenario "Swagger redirected off the
// instance": a document from another origin is not trusted.
func TestProbeDoesNotTrustSwaggerFromAnotherOrigin(t *testing.T) {
	other := httptest.NewServer(swaggerDoc("16.0.7"))
	t.Cleanup(other.Close)
	newSignInForge(t, "", http.StatusForbidden, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/swagger.v1.json", http.StatusFound)
	})

	got, err := ProbeServerVersion(context.Background())
	if !errors.Is(err, ErrSignInRequired) || !strings.Contains(err.Error(), "outside the Forgejo instance") {
		t.Fatalf("ProbeServerVersion = %q, %v; want ErrSignInRequired naming the redirect", got, err)
	}
}

// A redirect that stays on the instance is followed.
func TestProbeFollowsASameOriginSwaggerRedirect(t *testing.T) {
	var f *signInForge
	f = newSignInForge(t, "", http.StatusForbidden, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("moved") == "" {
			http.Redirect(w, r, f.srv.URL+"/swagger.v1.json?moved=1", http.StatusFound)
			return
		}
		swaggerDoc("16.0.7")(w, r)
	})

	got, err := ProbeServerVersion(context.Background())
	if err != nil || got != "16.0.7" {
		t.Fatalf("ProbeServerVersion = %q, %v; want 16.0.7", got, err)
	}
}

// User information in -url never reaches a probe error.
func TestProbeErrorsCarryNoUserInformation(t *testing.T) {
	f := newSignInForge(t, "", http.StatusForbidden, nil)
	flag.URL = strings.Replace(f.srv.URL, "http://", "http://probe-user:probe-secret@", 1)

	_, err := ProbeServerVersion(context.Background())
	if !errors.Is(err, ErrSignInRequired) {
		t.Fatalf("err = %v, want ErrSignInRequired", err)
	}
	if strings.Contains(err.Error(), "probe-secret") || strings.Contains(err.Error(), "probe-user") {
		t.Fatalf("error leaks user information: %v", err)
	}

	f.srv.Close()
	_, err = ProbeServerVersion(context.Background())
	if err == nil || strings.Contains(err.Error(), "probe-secret") || strings.Contains(err.Error(), "probe-user") {
		t.Fatalf("transport error = %v, want one without user information", err)
	}
}

// The whole probe is bounded by probeTimeout, not by rawHTTPClient's 60s.
func TestProbeIsBoundedByItsTimeout(t *testing.T) {
	prev := probeTimeout
	probeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { probeTimeout = prev })
	release := make(chan struct{})
	newSignInForge(t, "", http.StatusForbidden, func(http.ResponseWriter, *http.Request) { <-release })
	// Registered after the stub's own cleanup, so it runs first: srv.Close
	// waits for the stalled handler.
	t.Cleanup(func() { close(release) })

	start := time.Now()
	_, err := ProbeServerVersion(context.Background())
	if !errors.Is(err, ErrSignInRequired) {
		t.Fatalf("err = %v, want ErrSignInRequired", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("probe took %s, want it cut off near %s", d, probeTimeout)
	}
}

// CheckReachable never reads swagger: it reports a version on 200, nothing on
// a Forgejo sign-in refusal, and an error for anything else.
func TestCheckReachable(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		f := newSignInForge(t, "", http.StatusOK, swaggerDoc("99.0.0"))
		got, err := CheckReachable(context.Background())
		if err != nil || got != "16.0.7+gitea-1.22.0" {
			t.Fatalf("CheckReachable = %q, %v", got, err)
		}
		if f.swaggerRequests.Load() != 0 {
			t.Fatal("swagger was requested")
		}
	})
	t.Run("sign-in required", func(t *testing.T) {
		f := newSignInForge(t, "", http.StatusForbidden, swaggerDoc("16.0.7"))
		got, err := CheckReachable(context.Background())
		if err != nil || got != "" {
			t.Fatalf("CheckReachable = %q, %v; want reachable with no version", got, err)
		}
		if f.swaggerRequests.Load() != 0 {
			t.Fatal("swagger was requested")
		}
	})
	t.Run("404", func(t *testing.T) {
		newSignInForge(t, "", http.StatusNotFound, nil)
		if _, err := CheckReachable(context.Background()); err == nil {
			t.Fatal("CheckReachable accepted a 404")
		}
	})
}

func TestValidateVersion(t *testing.T) {
	for _, v := range []string{"16", "16.0.7", "v16.0.7", "16.0.7+gitea-1.22.0", "16.0.0-dev-741-6f391573+gitea-1.22.0"} {
		if err := ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
	// Each has a numeric major, but the SDK's parser refuses it.
	for _, v := range []string{"16.banana", "+16", "16.0.7 rc", "16..0", "16.0.x", "banana"} {
		if err := ValidateVersion(v); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want an error", v)
		}
	}
}

func TestSameVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"16.0.7", "16.0.7+gitea-1.22.0", true},
		{"16", "16.0.0", true},
		{"16.0.7", "16.0.6+gitea-1.22.0", false},
		{"16.0.7", "16.0.7-rc1", false},
	}
	for _, c := range cases {
		if got := SameVersion(c.a, c.b); got != c.want {
			t.Errorf("SameVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// The swagger fallback goes through the same client as /version, so it
// carries the TLS settings from -tls-cert, -tls-key and -tls-ca. Against an
// instance that requires both sign-in and a client certificate, the probe
// reads the version only with the certificate configured; without it, the
// handshake fails, which is not "sign-in required".
func TestProbeFallsBackToSwaggerOverMutualTLS(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	var swaggerRequests atomic.Int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Only signed in user is allowed to call APIs."}`))
		case "/swagger.v1.json":
			swaggerRequests.Add(1)
			swaggerDoc("16.0.7+gitea-1.22.0")(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.issue(t, x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour))},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	savedURL, savedToken := flag.URL, flag.Token
	flag.URL, flag.Token = srv.URL, ""
	t.Cleanup(func() { flag.URL, flag.Token = savedURL, savedToken })
	restoreTransports(t)

	if _, err := ProbeServerVersion(context.Background()); err == nil || errors.Is(err, ErrSignInRequired) {
		t.Fatalf("without TLS settings: err = %v, want a handshake failure that is not ErrSignInRequired", err)
	}

	s := clientFiles(t, ca, time.Now().Add(time.Hour))
	s.CAFile = writeFile(t, "ca.pem", ca.pem)
	configure(t, s)

	v, err := ProbeServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if v != "16.0.7+gitea-1.22.0" {
		t.Fatalf("version = %q, want 16.0.7+gitea-1.22.0", v)
	}
	if swaggerRequests.Load() != 1 {
		t.Fatalf("%d swagger requests got past the handshake, want 1", swaggerRequests.Load())
	}
}
