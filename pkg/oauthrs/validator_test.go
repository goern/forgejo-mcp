// SPDX-License-Identifier: GPL-3.0-or-later

package oauthrs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/lestrrat-go/jwx/v3/jwa"
)

func bearer(token string) string { return "Bearer " + token }

// Spec oauth-resource-server, scenario "Valid access token".
func TestValidAccessTokenIsAccepted(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	for _, typ := range []string{"at+jwt", "JWT", ""} {
		headers := map[string]string{"kid": "ec-1"}
		if typ != "" {
			headers["typ"] = typ
		}
		token := signToken(t, jwa.ES256(), key, headers, accessClaims(clock))
		claims, err := v.ValidateAuthorization(context.Background(), bearer(token))
		if err != nil {
			t.Fatalf("typ %q: valid token refused: %v", typ, err)
		}
		if claims.Subject != "388443616722288641" {
			t.Fatalf("subject = %q", claims.Subject)
		}
		if claims.Raw["forgejo_aud"] != "u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42" {
			t.Fatalf("raw claims lost forgejo_aud: %v", claims.Raw)
		}
	}
}

func TestAudienceAsASingleStringIsAccepted(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	claims := accessClaims(clock)
	claims["aud"] = testAudience
	token := signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1"}, claims)
	if _, err := v.ValidateAuthorization(context.Background(), bearer(token)); err != nil {
		t.Fatalf("string audience refused: %v", err)
	}
}

func TestSmallClockSkewIsTolerated(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	now := clock.Now().Unix()
	claims := accessClaims(clock)
	claims["iat"], claims["nbf"], claims["exp"] = now+30, now+30, now-30
	token := signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1"}, claims)
	if _, err := v.ValidateAuthorization(context.Background(), bearer(token)); err != nil {
		t.Fatalf("token within the 60 s leeway refused: %v", err)
	}
}

// Spec oauth-resource-server: every failing condition of "The MCP endpoint
// requires a valid bearer JWT access token" is refused as an invalid token,
// and for the intended reason, so that a case cannot pass by failing on
// something else.
func TestInvalidTokensAreRefused(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)
	now := clock.Now().Unix()

	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	with := func(mutate func(map[string]any)) map[string]any {
		c := accessClaims(clock)
		mutate(c)
		return c
	}
	kid := map[string]string{"kid": "ec-1"}
	valid := signToken(t, jwa.ES256(), key, kid, accessClaims(clock))
	parts := strings.Split(valid, ".")
	otherPayload := strings.Split(signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["sub"] = "someone-else" })), ".")[1]
	noneHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","kid":"ec-1"}`))

	cases := []struct {
		name   string
		token  string
		reason string
	}{
		// Spec scenario "Audience names another resource".
		{"audience names another resource", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["aud"] = []string{"another-resource"} })), `"aud" not satisfied`},
		{"no audience", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { delete(c, "aud") })), `"aud" not satisfied`},
		{"issuer differs by a slash", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["iss"] = testIssuer + "/" })), `"iss" not satisfied`},
		{"expired beyond leeway", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["exp"] = now - 120 })), `"exp" not satisfied`},
		{"no exp", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { delete(c, "exp") })), "has no exp"},
		{"nbf beyond leeway", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["nbf"] = now + 120 })), `"nbf" not satisfied`},
		{"iat beyond leeway", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["iat"] = now + 120 })), `"iat" not satisfied`},
		// Spec scenario "ID token presented as an access token".
		{"ID token with nonce", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["nonce"] = "n-0S6_WzA2Mj" })), `carries "nonce"`},
		{"ID token with at_hash", signToken(t, jwa.ES256(), key, kid, with(func(c map[string]any) { c["at_hash"] = "77QmUPtjPfzWtF2AnpK9RQ" })), `carries "at_hash"`},
		{"typ of another token", signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1", "typ": "dpop+jwt"}, accessClaims(clock)), "does not declare an access token"},
		{"no kid", signToken(t, jwa.ES256(), key, nil, accessClaims(clock)), "header has no kid"},
		{"unknown kid", signToken(t, jwa.ES256(), otherKey, map[string]string{"kid": "ec-unknown"}, accessClaims(clock)), "no provider key has the token's key ID"},
		{"signed by another key", signToken(t, jwa.ES256(), otherKey, kid, accessClaims(clock)), "signature does not verify"},
		{"key curve and alg mismatch", signToken(t, jwa.ES384(), p384, kid, accessClaims(clock)), "is an EC P-256 key, but the token declares ES384"},
		// Spec scenario "Symmetric algorithm".
		{"HS256", signToken(t, jwa.HS256(), []byte("a-shared-secret-of-sufficient-length!"), kid, accessClaims(clock)), `algorithm "HS256" is not an accepted asymmetric algorithm`},
		{"alg none", noneHeader + "." + parts[1] + ".", `algorithm "none" is not an accepted asymmetric algorithm`},
		{"tampered payload", parts[0] + "." + otherPayload + "." + parts[2], "signature does not verify"},
		// Spec scenario "Opaque token".
		// Low-entropy on purpose: a random-looking literal here trips secret
		// scanners although it is no credential.
		{"opaque token", "opaque-reference-not-a-jwt", "not a JWT in compact serialization"},
		{"JWS JSON serialization", `{"payload":"e30","signatures":[]}`, "not a JWT in compact serialization"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := v.ValidateAuthorization(context.Background(), bearer(tc.token))
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("error = %v, want ErrInvalidToken", err)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("refused for the wrong reason: %v, want %q", err, tc.reason)
			}
		})
	}
}

// Spec oauth-resource-server, scenarios "No token" and "Forgejo token scheme":
// a missing header and a non-Bearer scheme mean no bearer token was presented.
func TestAuthorizationSchemes(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)
	token := signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1"}, accessClaims(clock))

	for _, header := range []string{"", "Bearer", "Bearer   ", "token " + token, "Basic dXNlcjpwYXNz"} {
		if _, err := v.ValidateAuthorization(context.Background(), header); !errors.Is(err, ErrNoToken) {
			t.Errorf("header %q: error = %v, want ErrNoToken", truncate(header), err)
		}
	}
	for _, header := range []string{"bearer " + token, "BEARER " + token} {
		if _, err := v.ValidateAuthorization(context.Background(), header); err != nil {
			t.Errorf("header with scheme %q refused: %v", strings.Fields(header)[0], err)
		}
	}
}

func truncate(s string) string {
	if len(s) > 20 {
		return s[:20] + "…"
	}
	return s
}

// A refusal's error must not contain the token, so that logging it at debug
// level cannot leak a credential.
func TestRefusalErrorsDoNotContainTheToken(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	claims := accessClaims(clock)
	claims["aud"] = []string{"another-resource"}
	token := signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1"}, claims)
	_, err := v.ValidateAuthorization(context.Background(), bearer(token))
	if err == nil {
		t.Fatal("token with a foreign audience was accepted")
	}
	for _, part := range strings.Split(token, ".") {
		if len(part) > 8 && strings.Contains(err.Error(), part) {
			t.Fatalf("error contains a token segment: %v", err)
		}
	}
}

func TestNewValidatorRequiresItsInputs(t *testing.T) {
	keys := newTestKeySet(t, func() *testProvider { p := newTestProvider(t); p.addECKey("ec-1"); return p }(), newFakeClock())
	if _, err := NewValidator("", testAudience, keys, nil); err == nil {
		t.Error("accepted an empty issuer")
	}
	if _, err := NewValidator(testIssuer, "", keys, nil); err == nil {
		t.Error("accepted an empty audience")
	}
	if _, err := NewValidator(testIssuer, testAudience, nil, nil); err == nil {
		t.Error("accepted a nil key lookup")
	}
}
