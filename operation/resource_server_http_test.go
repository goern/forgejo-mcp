// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"go.uber.org/zap"
)

const (
	rsHost          = "mcp.example.org"
	rsClientID      = "390183324498329601"
	rsForgejoAud    = "u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42"
	rsSubject       = "388443616722288641"
	rsMetadataURL   = "https://mcp.example.org/.well-known/oauth-protected-resource/mcp"
	rsBaseChallenge = `Bearer resource_metadata="` + rsMetadataURL + `", scope="openid profile"`
)

// rsEnv is a running resource-server HTTP surface.
type rsEnv struct {
	addr    string
	fixture *fixture
	rs      *resourceServer
	// reached records, for each request that reached the MCP handler, the
	// outcome of calling Forgejo through both client paths.
	reached chan error
}

// startResourceServer prepares resource-server mode against a fake Forgejo and
// identity provider and serves it through the real listener stack. The MCP
// handler is a probe that calls Forgejo through the typed SDK client and the
// raw-HTTP helper, exactly as tool handlers do.
func startResourceServer(t *testing.T) *rsEnv {
	t.Helper()
	withResourceServerConfig(t)
	env := &rsEnv{reached: make(chan error, 8)}
	env.fixture = newFixture(t, "16.0.3", mustP256(t), "")
	flag.Host = "127.0.0.1"
	flag.AllowedHosts = []string{rsHost}
	flag.ScopesSupported = []string{"openid", "profile"}
	flag.ResourceAudience = rsClientID

	if err := ValidateAuthConfig("http", false); err != nil {
		t.Fatalf("ValidateAuthConfig: %v", err)
	}
	rs, err := prepareResourceServer(context.Background())
	if err != nil {
		t.Fatalf("prepareResourceServer: %v", err)
	}
	env.rs = rs

	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := requestTokenContextFunc(r.Context(), r)
		client, err := forgejo.Client(ctx)
		if err == nil {
			_, _, err = client.GetMyUserInfo()
		}
		if err == nil {
			var user map[string]any
			err = forgejo.DoJSON(ctx, http.MethodGet, "/user", nil, &user)
		}
		env.reached <- err
		w.WriteHeader(http.StatusNoContent)
	})
	handler, err := rs.handler(probe)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	cfg, err := resolveTransportConfig("http")
	if err != nil {
		t.Fatalf("resolveTransportConfig: %v", err)
	}
	forgejo.SetRequireRequestToken(cfg.requireAuth)
	ln := mustListenLoopback(t)
	srv := newMCPHTTPServer(handler, cfg)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	env.addr = ln.Addr().String()
	return env
}

// do sends a request with Host set to the resource host and never follows a
// redirect, so a redirect is visible as a 3xx.
func (e *rsEnv) do(t *testing.T, method, path, authorization string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+e.addr+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Host = rsHost
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

// accessToken signs an access token from the fake provider. mutate may edit
// the claims before signing.
func (e *rsEnv) accessToken(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	now := time.Now().Unix()
	claims := map[string]any{
		"iss": e.fixture.providerIssuer, "sub": rsSubject, "aud": []string{rsClientID, "390183248413589505"},
		"iat": now, "nbf": now, "exp": now + 3600, "jti": "at-1", "forgejo_aud": rsForgejoAud,
	}
	if mutate != nil {
		mutate(claims)
	}
	key, err := jwk.Import(e.fixture.providerKey)
	if err != nil {
		t.Fatal(err)
	}
	_ = key.Set(jwk.KeyIDKey, e.fixture.providerKid)
	b := jwt.NewBuilder()
	for name, value := range claims {
		b = b.Claim(name, value)
	}
	token, err := b.Build()
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.ES256(), key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return string(signed)
}

// Spec oauth-resource-server, scenarios "Metadata at the path-specific
// location" and "Metadata with a forged Host".
func TestProtectedResourceMetadataIsPublishedWithoutAuthentication(t *testing.T) {
	env := startResourceServer(t)
	for _, path := range []string{"/.well-known/oauth-protected-resource/mcp", "/.well-known/oauth-protected-resource"} {
		resp, body := env.do(t, http.MethodGet, path, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, resp.StatusCode)
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(body), &doc); err != nil {
			t.Fatalf("GET %s: not JSON: %v", path, err)
		}
		if doc["resource"] != "https://mcp.example.org/mcp" {
			t.Fatalf("resource = %v", doc["resource"])
		}
		if servers, _ := doc["authorization_servers"].([]any); len(servers) != 1 || servers[0] != env.fixture.providerIssuer {
			t.Fatalf("authorization_servers = %v", doc["authorization_servers"])
		}
		if methods, _ := doc["bearer_methods_supported"].([]any); len(methods) != 1 || methods[0] != "header" {
			t.Fatalf("bearer_methods_supported = %v", doc["bearer_methods_supported"])
		}
		if scopes, _ := doc["scopes_supported"].([]any); len(scopes) != 2 {
			t.Fatalf("scopes_supported = %v", doc["scopes_supported"])
		}
	}

	got := rawRequest(t, env.addr, "GET /.well-known/oauth-protected-resource/mcp HTTP/1.1", "Host: attacker.example.com")
	if !strings.Contains(got, "403") {
		t.Fatalf("metadata with a forged Host was not rejected: %q", got)
	}
}

func TestScopesSupportedIsOmittedWhenNotConfigured(t *testing.T) {
	withResourceServerConfig(t)
	newFixture(t, "16.0.3", mustP256(t), "")
	rs, err := prepareResourceServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rs.scopes = nil
	handler, err := rs.handler(http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://x/.well-known/oauth-protected-resource", nil)
	rec := &recorder{header: http.Header{}}
	handler.ServeHTTP(rec, req)
	if strings.Contains(rec.body.String(), "scopes_supported") {
		t.Fatalf("scopes_supported present without configuration: %s", rec.body.String())
	}
	if challenge := bearerChallenge(rsMetadataURL, nil); strings.Contains(challenge, "scope=") {
		t.Fatalf("challenge carries scope without configuration: %s", challenge)
	}
}

// Spec forgejo-jwt-issuer, scenario "Forgejo fetches discovery", served over
// HTTP.
func TestIssuerDocumentsAreServedUnderTheIssuerPath(t *testing.T) {
	env := startResourceServer(t)

	resp, body := env.do(t, http.MethodGet, "/issuer/.well-known/openid-configuration", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("discovery = %d, want 200", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("discovery Cache-Control = %q", cc)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(body), &doc); err != nil || len(doc) != 3 {
		t.Fatalf("discovery document = %s (%v), want exactly 3 members", body, err)
	}
	if doc["issuer"] != "https://mcp.example.org/issuer" || doc["jwks_uri"] != "https://mcp.example.org/issuer/jwks.json" {
		t.Fatalf("discovery document = %v", doc)
	}

	resp, body = env.do(t, http.MethodGet, "/issuer/jwks.json", "")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("jwks = %d, Cache-Control %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if strings.Contains(body, `"d"`) {
		t.Fatalf("key set leaks private material: %s", body)
	}

	if resp, _ := env.do(t, http.MethodPost, "/issuer/jwks.json", ""); resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST to the key set = %d, want 405", resp.StatusCode)
	}
}

// Spec oauth-resource-server, scenarios "Root OpenID discovery" and "Trailing
// slash".
func TestOnlyDefinedRoutesAnswerAndNothingRedirects(t *testing.T) {
	env := startResourceServer(t)
	for _, path := range []string{
		"/",
		"/.well-known/openid-configuration",
		"/.well-known/oauth-protected-resource/mcp/",
		"/issuer/",
		"/issuer/jwks.json/",
		"/mcp/",
		"/sse",
	} {
		resp, _ := env.do(t, http.MethodGet, path, "")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, resp.StatusCode)
		}
	}
	// An unclean path would draw a redirect from http.ServeMux.
	got := rawRequest(t, env.addr, "GET /issuer/../mcp HTTP/1.1", "Host: "+rsHost)
	if !strings.Contains(got, "404") || strings.Contains(got, "301") || strings.Contains(got, "Location:") {
		t.Fatalf("GET /issuer/../mcp was not a plain 404: %q", got)
	}
}

func TestOptionsStarWithAForgedHostIsRefusedInResourceServerMode(t *testing.T) {
	env := startResourceServer(t)
	got := rawRequest(t, env.addr, "OPTIONS * HTTP/1.1", "Host: attacker.example.com")
	if !strings.Contains(got, "403") {
		t.Fatalf("OPTIONS * with a forged Host was not rejected: %q", got)
	}
}

// Spec oauth-resource-server, scenarios "No token" and "Expired and wrongly
// signed tokens are indistinguishable".
func TestTheMCPEndpointChallengesUniformly(t *testing.T) {
	env := startResourceServer(t)

	resp, body := env.do(t, http.MethodPost, "/mcp", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != rsBaseChallenge {
		t.Fatalf("challenge without a token = %q, want %q", got, rsBaseChallenge)
	}
	if body != "" {
		t.Fatalf("401 body = %q, want empty", body)
	}

	expired := env.accessToken(t, func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() })
	otherKey := mustP256(t)
	forged, err := jwk.Import(otherKey)
	if err != nil {
		t.Fatal(err)
	}
	_ = forged.Set(jwk.KeyIDKey, env.fixture.providerKid)
	tok, _ := jwt.NewBuilder().Issuer(env.fixture.providerIssuer).Subject(rsSubject).Audience([]string{rsClientID}).
		Expiration(time.Now().Add(time.Hour)).Build()
	wrongSignature, err := jwt.Sign(tok, jwt.WithKey(jwa.ES256(), forged))
	if err != nil {
		t.Fatal(err)
	}

	type outcome struct{ status, challenge, body string }
	var outcomes []outcome
	for _, token := range []string{expired, string(wrongSignature)} {
		resp, body := env.do(t, http.MethodPost, "/mcp", "Bearer "+token)
		outcomes = append(outcomes, outcome{resp.Status, resp.Header.Get("WWW-Authenticate"), body})
	}
	if outcomes[0] != outcomes[1] {
		t.Fatalf("refusals are distinguishable: %+v vs %+v", outcomes[0], outcomes[1])
	}
	if want := rsBaseChallenge + `, error="invalid_token"`; outcomes[0].challenge != want || outcomes[0].status != "401 Unauthorized" {
		t.Fatalf("refusal = %+v, want 401 with challenge %q", outcomes[0], want)
	}
	if len(env.fixture.requests()) != 1 { // only the startup version probe
		t.Fatalf("refused requests reached Forgejo: %v", env.fixture.requests())
	}
}

// Spec forgejo-jwt-issuer, scenarios "Claim missing" and "Claim is an array".
func TestATokenWithoutAUsableForgejoAudienceIsForbidden(t *testing.T) {
	env := startResourceServer(t)
	for name, mutate := range map[string]func(map[string]any){
		"claim missing":     func(c map[string]any) { delete(c, "forgejo_aud") },
		"claim is an array": func(c map[string]any) { c["forgejo_aud"] = []string{"u:1:a", "u:2:b"} },
	} {
		t.Run(name, func(t *testing.T) {
			resp, body := env.do(t, http.MethodPost, "/mcp", "Bearer "+env.accessToken(t, mutate))
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", resp.StatusCode)
			}
			if !strings.Contains(body, "forgejo_aud") {
				t.Fatalf("403 body does not name the claim: %q", body)
			}
			if resp.Header.Get("WWW-Authenticate") != "" {
				t.Fatalf("403 carries a challenge: %q", resp.Header.Get("WWW-Authenticate"))
			}
		})
	}
	if len(env.fixture.requests()) != 1 {
		t.Fatalf("forbidden requests reached Forgejo: %v", env.fixture.requests())
	}
}

func TestCustomAudienceClaimName(t *testing.T) {
	env := startResourceServer(t)
	env.rs.audienceClaim = "urn:example:forgejo"
	handler, err := env.rs.handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := r.Context().Value(mintedTokenKey{}).(string)
		_, _ = w.Write([]byte(token))
	}))
	if err != nil {
		t.Fatal(err)
	}
	token := env.accessToken(t, func(c map[string]any) {
		delete(c, "forgejo_aud")
		c["urn:example:forgejo"] = "u:7:custom"
	})
	req, _ := http.NewRequest(http.MethodPost, "http://x/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := &recorder{header: http.Header{}}
	handler.ServeHTTP(rec, req)
	if rec.status != 0 && rec.status != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.status, rec.body.String())
	}
	if claims := jwtClaims(t, rec.body.String()); claims["aud"] != "u:7:custom" {
		t.Fatalf("minted aud = %v, want u:7:custom", claims["aud"])
	}
}

// Spec forgejo-jwt-issuer, scenario "A tool call reaches Forgejo"; spec
// oauth-resource-server, scenario "Forgejo never sees the inbound token".
func TestAToolCallReachesForgejoWithTheMintedTokenOnly(t *testing.T) {
	env := startResourceServer(t)
	inbound := env.accessToken(t, nil)

	resp, _ := env.do(t, http.MethodPost, "/mcp", "Bearer "+inbound)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("authenticated request = %d, want it to reach the handler", resp.StatusCode)
	}
	if err := <-env.reached; err != nil {
		t.Fatalf("calling Forgejo from the handler failed: %v", err)
	}

	var userCalls []string
	for _, req := range env.fixture.requests() {
		if strings.Contains(req, inbound) {
			t.Fatalf("Forgejo received the inbound token: %q", req)
		}
		if strings.HasPrefix(req, "/api/v1/user ") {
			userCalls = append(userCalls, strings.TrimPrefix(req, "/api/v1/user "))
		}
		if strings.HasPrefix(req, "/api/v1/version ") && req != "/api/v1/version " {
			t.Fatalf("a version request carried a credential: %q", req)
		}
	}
	if len(userCalls) != 2 {
		t.Fatalf("Forgejo saw %d user requests, want 2 (typed client and raw helper): %v", len(userCalls), env.fixture.requests())
	}
	if userCalls[0] != userCalls[1] {
		t.Fatalf("the two client paths used different credentials: %q vs %q", userCalls[0], userCalls[1])
	}
	minted, ok := strings.CutPrefix(userCalls[0], "token ")
	if !ok {
		t.Fatalf("Forgejo credential = %q, want the token scheme", userCalls[0])
	}
	claims := jwtClaims(t, minted)
	if claims["iss"] != "https://mcp.example.org/issuer" || claims["sub"] != rsSubject || claims["aud"] != rsForgejoAud {
		t.Fatalf("minted token claims = %v", claims)
	}
	// No per-request version probe: the version was recorded at startup.
	versionCalls := 0
	for _, req := range env.fixture.requests() {
		if strings.HasPrefix(req, "/api/v1/version") {
			versionCalls++
		}
	}
	if versionCalls != 1 {
		t.Fatalf("Forgejo saw %d version requests, want only the startup probe", versionCalls)
	}
}

// Spec oauth-resource-server, scenario "Debug logging of a refusal"; spec
// forgejo-jwt-issuer, scenario "Minted JWTs are not logged".
func TestNeitherTokenIsLoggedAtAnyLevel(t *testing.T) {
	env := startResourceServer(t)
	logs := captureLogs(t, zap.DebugLevel)

	inbound := env.accessToken(t, nil)
	refused := env.accessToken(t, func(c map[string]any) { c["aud"] = []string{"another-resource"} })
	env.do(t, http.MethodPost, "/mcp", "Bearer "+refused)
	env.do(t, http.MethodPost, "/mcp", "Bearer "+inbound)
	if err := <-env.reached; err != nil {
		t.Fatalf("calling Forgejo failed: %v", err)
	}

	var secrets []string
	for _, token := range []string{inbound, refused} {
		secrets = append(secrets, token, strings.Split(token, ".")[2])
	}
	for _, req := range env.fixture.requests() {
		if minted, ok := strings.CutPrefix(strings.TrimPrefix(req, "/api/v1/user "), "token "); ok {
			secrets = append(secrets, minted, strings.Split(minted, ".")[2])
		}
	}

	if logs.Len() == 0 {
		t.Fatal("nothing was logged; the scan below would prove nothing")
	}
	for _, entry := range logs.All() {
		line := entry.Message + " " + fmt.Sprint(entry.ContextMap())
		for _, secret := range secrets {
			if strings.Contains(line, secret) {
				t.Fatalf("a token appears in the log: %q", line)
			}
		}
	}
}

func TestUsableAudience(t *testing.T) {
	for value, want := range map[any]bool{
		"u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42": true,
		"":                       false,
		"u:1:a b":                false,
		"u:1:a\x00":              false,
		strings.Repeat("a", 257): false,
		42:                       false,
	} {
		if _, got := usableAudience(value); got != want {
			t.Errorf("usableAudience(%q) = %v, want %v", fmt.Sprint(value), got, want)
		}
	}
	if _, ok := usableAudience([]any{"u:1:a"}); ok {
		t.Error("usableAudience accepted an array")
	}
}

// jwtClaims decodes the claims of a compact JWT without verifying it.
func jwtClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a compact JWT: %q", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("claims segment: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatalf("claims JSON: %v", err)
	}
	return claims
}

// recorder is a minimal ResponseWriter for handler-level tests.
type recorder struct {
	header http.Header
	status int
	body   strings.Builder
}

func (r *recorder) Header() http.Header { return r.header }
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
func (r *recorder) WriteHeader(status int) { r.status = status }
