// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// withResourceServerConfig installs a valid resource-server configuration and
// restores every auth-related setting afterwards.
func withResourceServerConfig(t *testing.T) {
	t.Helper()
	saved := struct {
		mode, authServer, res, resAud, claim, issuer, keyFile, token, host, url string
		scopes, published, settings, allowedHosts, allowedOrigins               []string
		fallback, require                                                       bool
		httpPort, ssePort                                                       int
	}{
		flag.AuthMode, flag.AuthorizationServer, flag.Resource, flag.ResourceAudience, flag.ForgejoAudienceClaim,
		flag.ForgejoJWTIssuer, flag.ForgejoJWTSigningKeyFile, flag.Token, flag.Host, flag.URL,
		flag.ScopesSupported, flag.ForgejoJWTPublishedKeyFiles, flag.ResourceServerSettings, flag.AllowedHosts, flag.AllowedOrigins,
		flag.AllowOperatorTokenFallback, forgejo.RequireRequestToken(), flag.HTTPPort, flag.SSEPort,
	}
	t.Cleanup(func() {
		flag.AuthMode, flag.AuthorizationServer, flag.Resource = saved.mode, saved.authServer, saved.res
		flag.ResourceAudience, flag.ForgejoAudienceClaim, flag.ForgejoJWTIssuer = saved.resAud, saved.claim, saved.issuer
		flag.ForgejoJWTSigningKeyFile, flag.Token, flag.Host, flag.URL = saved.keyFile, saved.token, saved.host, saved.url
		flag.ScopesSupported, flag.ForgejoJWTPublishedKeyFiles = saved.scopes, saved.published
		flag.ResourceServerSettings, flag.AllowedHosts, flag.AllowedOrigins = saved.settings, saved.allowedHosts, saved.allowedOrigins
		flag.AllowOperatorTokenFallback, flag.HTTPPort, flag.SSEPort = saved.fallback, saved.httpPort, saved.ssePort
		forgejo.SetRequireRequestToken(saved.require)
		forgejo.SetServerVersion("")
	})

	flag.AuthMode = AuthModeResourceServer
	flag.AuthorizationServer = "https://sso.example.org"
	flag.Resource = "https://mcp.example.org/mcp"
	flag.ResourceAudience = ""
	flag.ForgejoAudienceClaim = "forgejo_aud"
	flag.ForgejoJWTIssuer = "https://mcp.example.org/issuer"
	flag.ForgejoJWTSigningKeyFile = "/run/credentials/forgejo-mcp/signing.pem"
	flag.ForgejoJWTPublishedKeyFiles = nil
	flag.ScopesSupported = nil
	flag.ResourceServerSettings = nil
	flag.Token = ""
	flag.AllowOperatorTokenFallback = false
	flag.Host = "0.0.0.0"
	flag.AllowedHosts = []string{"mcp.example.org"}
	flag.AllowedOrigins = nil
}

// Spec oauth-resource-server: "resource-server mode refuses to start on an
// unusable configuration", "Auth mode is selected explicitly and defaults to
// passthrough" and "passthrough mode refuses resource-server settings". Every
// row names the setting that fixes it.
func TestValidateAuthConfig(t *testing.T) {
	cases := []struct {
		name      string
		transport string
		cli       bool
		mutate    func()
		want      []string // substrings of the error; nil means valid
	}{
		{name: "valid resource-server configuration", transport: "http", mutate: func() {}},
		{name: "unknown mode value", transport: "http", mutate: func() { flag.AuthMode = "oauth" },
			want: []string{"-auth-mode", "passthrough", "resource-server"}},
		{name: "sse transport", transport: "sse", mutate: func() {},
			want: []string{"requires the http transport"}},
		{name: "stdio transport", transport: "stdio", mutate: func() {},
			want: []string{"requires the http transport"}},
		{name: "cli mode", transport: "", cli: true, mutate: func() {},
			want: []string{"--cli"}},
		{name: "operator token present", transport: "http", mutate: func() { flag.Token = "decoy-operator-value" },
			want: []string{"takes no operator token", "FORGEJO_ACCESS_TOKEN"}},
		{name: "operator token fallback", transport: "http", mutate: func() { flag.AllowOperatorTokenFallback = true },
			want: []string{"-allow-operator-token-fallback"}},
		{name: "missing authorization server", transport: "http", mutate: func() { flag.AuthorizationServer = "" },
			want: []string{"-authorization-server"}},
		{name: "missing resource and signing key", transport: "http", mutate: func() { flag.Resource, flag.ForgejoJWTSigningKeyFile = "", "" },
			want: []string{"-resource", "-forgejo-jwt-signing-key-file"}},
		{name: "missing forgejo jwt issuer", transport: "http", mutate: func() { flag.ForgejoJWTIssuer = "" },
			want: []string{"-forgejo-jwt-issuer"}},
		{name: "scope with a quote", transport: "http", mutate: func() { flag.ScopesSupported = []string{"openid", `pro"file`} },
			want: []string{"-scopes-supported", "not a valid OAuth scope"}},
		{name: "forgejo issuer over http", transport: "http", mutate: func() { flag.ForgejoJWTIssuer = "http://mcp.example.org/issuer" },
			want: []string{"-forgejo-jwt-issuer", "https"}},
		// Spec scenario "Issuer URL with a trailing slash".
		{name: "forgejo issuer with a trailing slash", transport: "http", mutate: func() { flag.ForgejoJWTIssuer = "https://mcp.example.org/issuer/" },
			want: []string{"must not end with a slash"}},
		{name: "forgejo issuer with a query", transport: "http", mutate: func() { flag.ForgejoJWTIssuer = "https://mcp.example.org/issuer?v=1" },
			want: []string{"query"}},
		{name: "authorization server over plain http", transport: "http", mutate: func() { flag.AuthorizationServer = "http://sso.example.org" },
			want: []string{"-authorization-server", "https"}},
		{name: "authorization server over http on loopback", transport: "http", mutate: func() { flag.AuthorizationServer = "http://127.0.0.1:8081" }},
		{name: "resource host not answered", transport: "http", mutate: func() { flag.Resource = "https://other.example.org/mcp" },
			want: []string{"-resource", "-allowed-hosts"}},
		// Spec scenario "Resource names another path".
		{name: "resource path is not the MCP endpoint", transport: "http", mutate: func() { flag.Resource = "https://mcp.example.org/api/mcp" },
			want: []string{"-resource", `"/mcp"`}},
		{name: "resource path with a trailing slash", transport: "http", mutate: func() { flag.Resource = "https://mcp.example.org/mcp/" },
			want: []string{"-resource", `"/mcp"`}},
		// Spec scenario "Issuer host not answered".
		{name: "issuer host not answered", transport: "http", mutate: func() { flag.ForgejoJWTIssuer = "https://issuer.example.org/issuer" },
			want: []string{"-forgejo-jwt-issuer", "-allowed-hosts"}},
		{name: "loopback names on a loopback listener", transport: "http", mutate: func() {
			flag.Host, flag.AllowedHosts = "127.0.0.1", nil
			flag.Resource, flag.ForgejoJWTIssuer = "https://localhost:8443/mcp", "https://localhost:8443/issuer"
		}},
		{name: "passthrough with nothing given", transport: "stdio", mutate: func() { flag.AuthMode = AuthModePassthrough }},
		{name: "empty mode means passthrough", transport: "stdio", mutate: func() { flag.AuthMode = "" }},
		// Spec scenario "Issuer configured without the mode".
		{name: "passthrough with a resource-server setting", transport: "http", mutate: func() {
			flag.AuthMode = AuthModePassthrough
			flag.ResourceServerSettings = []string{"FORGEJO_MCP_AUTHORIZATION_SERVER"}
		}, want: []string{"FORGEJO_MCP_AUTHORIZATION_SERVER", "requires -auth-mode resource-server"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withResourceServerConfig(t)
			tc.mutate()
			err := ValidateAuthConfig(tc.transport, tc.cli)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("valid configuration refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid configuration accepted")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q does not mention %q", err, w)
				}
			}
		})
	}
}

// freePort returns a port nothing listens on right now.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func assertPortFree(t *testing.T, port int) {
	t.Helper()
	addr := (&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}).String()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port %d is bound after a refused start: %v", port, err)
	}
	_ = ln.Close()
}

// Spec scenario "Incompatible transport": refused before binding any listener.
func TestRefusedConfigurationNeverBinds(t *testing.T) {
	withResourceServerConfig(t)
	flag.Host, flag.AllowedHosts = "127.0.0.1", []string{"mcp.example.org"}
	port := freePort(t)
	flag.SSEPort = port

	err := Run("sse", "test")
	if err == nil || !strings.Contains(err.Error(), "requires the http transport") {
		t.Fatalf("Run(sse) error = %v, want a transport refusal", err)
	}
	assertPortFree(t, port)
}

// fixture serves a Forgejo instance and an identity provider, and writes a
// signing key, so prepareResourceServer and the request flow can run for real.
type fixture struct {
	providerIssuer string
	providerKey    *ecdsa.PrivateKey
	providerKid    string

	mu sync.Mutex
	// forgejoRequests records "<path> <Authorization header>" for every request
	// Forgejo received.
	forgejoRequests []string
}

func (f *fixture) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.forgejoRequests...)
}

func newFixture(t *testing.T, forgejoVersion string, signingKey crypto.PrivateKey, issuerSuffix string) *fixture {
	t.Helper()
	f := &fixture{providerKid: "provider-1"}

	forgejoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.forgejoRequests = append(f.forgejoRequests, r.URL.Path+" "+r.Header.Get("Authorization"))
		f.mu.Unlock()
		switch r.URL.Path {
		case "/api/v1/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": forgejoVersion})
		case "/api/v1/user":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "login": "synapse"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(forgejoSrv.Close)
	flag.URL = forgejoSrv.URL

	providerKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f.providerKey = providerKey
	var idp *httptest.Server
	idp = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": idp.URL + issuerSuffix, "jwks_uri": idp.URL + "/keys"})
		case "/keys":
			pub, _ := jwk.Import(providerKey.Public())
			_ = pub.Set(jwk.KeyIDKey, f.providerKid)
			set := jwk.NewSet()
			_ = set.AddKey(pub)
			_ = json.NewEncoder(w).Encode(set)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(idp.Close)
	flag.AuthorizationServer = idp.URL
	f.providerIssuer = idp.URL

	der, err := x509.MarshalPKCS8PrivateKey(signingKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "signing.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	flag.ForgejoJWTSigningKeyFile = path
	return f
}

func mustP256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// captureLogs routes the package logger into an observer for the test.
func captureLogs(t *testing.T, level zapcore.Level) *observer.ObservedLogs {
	t.Helper()
	prev := log.Default()
	core, logs := observer.New(level)
	log.SetDefault(zap.New(core))
	t.Cleanup(func() { log.SetDefault(prev) })
	return logs
}

// Spec forgejo-jwt-issuer, scenario "Startup warning".
func TestPrepareResourceServerSucceedsAndWarnsAboutTheSigningKey(t *testing.T) {
	withResourceServerConfig(t)
	f := newFixture(t, "16.0.3", mustP256(t), "")
	logs := captureLogs(t, zap.WarnLevel)

	rs, err := prepareResourceServer(context.Background())
	if err != nil {
		t.Fatalf("prepareResourceServer: %v", err)
	}
	for _, req := range f.requests() {
		if !strings.HasSuffix(req, " ") {
			t.Fatalf("the Forgejo version probe sent an Authorization header: %q", req)
		}
	}
	var found bool
	for _, entry := range logs.All() {
		if !strings.Contains(entry.Message, "credential for every Forgejo user") {
			continue
		}
		fields := entry.ContextMap()
		if fields["kid"] != rs.issuer.SigningKeyID() || fields["issuer"] != flag.ForgejoJWTIssuer {
			t.Fatalf("warning fields = %v, want kid %q and issuer %q", fields, rs.issuer.SigningKeyID(), flag.ForgejoJWTIssuer)
		}
		found = true
	}
	if !found {
		t.Fatalf("no startup warning about the signing key; logged: %v", logs.All())
	}
}

// Spec oauth-resource-server, scenario "Forgejo too old".
func TestPrepareResourceServerRefusesForgejoBefore16(t *testing.T) {
	withResourceServerConfig(t)
	newFixture(t, "15.0.3", mustP256(t), "")

	_, err := prepareResourceServer(context.Background())
	if err == nil || !strings.Contains(err.Error(), "requires Forgejo 16.0 or newer") || !strings.Contains(err.Error(), "15.0.3") {
		t.Fatalf("error = %v, want a refusal naming Forgejo 16.0 and the reported version", err)
	}
}

func TestPrepareResourceServerAcceptsADevelopmentBuildOf16(t *testing.T) {
	withResourceServerConfig(t)
	newFixture(t, "16.0.0-dev-741-6f391573+gitea-1.22.0", mustP256(t), "")

	if _, err := prepareResourceServer(context.Background()); err != nil {
		t.Fatalf("development build of Forgejo 16 refused: %v", err)
	}
}

// Spec forgejo-jwt-issuer, scenario "Unsupported key".
func TestPrepareResourceServerRefusesAnUnsupportedSigningKey(t *testing.T) {
	withResourceServerConfig(t)
	weak, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	newFixture(t, "16.0.3", weak, "")

	_, err = prepareResourceServer(context.Background())
	if err == nil || !strings.Contains(err.Error(), "-forgejo-jwt-signing-key-file") {
		t.Fatalf("error = %v, want a refusal naming -forgejo-jwt-signing-key-file", err)
	}
}

// Spec oauth-resource-server, scenario "Provider issuer mismatch".
func TestPrepareResourceServerRefusesAProviderIssuerMismatch(t *testing.T) {
	withResourceServerConfig(t)
	newFixture(t, "16.0.3", mustP256(t), "/")

	_, err := prepareResourceServer(context.Background())
	if err == nil || !strings.Contains(err.Error(), "-authorization-server") || !strings.Contains(err.Error(), "must match exactly") {
		t.Fatalf("error = %v, want a refusal of the issuer mismatch", err)
	}
}
