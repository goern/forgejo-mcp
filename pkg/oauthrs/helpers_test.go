// SPDX-License-Identifier: GPL-3.0-or-later

package oauthrs

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
)

const (
	testIssuer   = "https://sso.example.org"
	testAudience = "390183324498329601"
)

// fakeClock is a settable clock shared by the key set and the validator.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Unix(1_800_000_000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// testProvider serves a mutable key set and counts how often it is fetched.
type testProvider struct {
	t       *testing.T
	server  *httptest.Server
	mu      sync.Mutex
	keys    map[string]crypto.Signer
	status  int
	fetches atomic.Int64
}

func newTestProvider(t *testing.T) *testProvider {
	t.Helper()
	p := &testProvider{t: t, keys: map[string]crypto.Signer{}, status: http.StatusOK}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.fetches.Add(1)
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.status != http.StatusOK {
			w.WriteHeader(p.status)
			return
		}
		set := jwk.NewSet()
		for kid, signer := range p.keys {
			pub, err := jwk.Import(signer.Public())
			if err != nil {
				t.Errorf("import key: %v", err)
				return
			}
			_ = pub.Set(jwk.KeyIDKey, kid)
			_ = pub.Set(jwk.KeyUsageKey, jwk.ForSignature)
			_ = set.AddKey(pub)
		}
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *testProvider) addECKey(kid string) *ecdsa.PrivateKey {
	p.t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		p.t.Fatalf("generate key: %v", err)
	}
	p.mu.Lock()
	p.keys[kid] = k
	p.mu.Unlock()
	return k
}

func (p *testProvider) removeKey(kid string) {
	p.mu.Lock()
	delete(p.keys, kid)
	p.mu.Unlock()
}

func (p *testProvider) setStatus(status int) {
	p.mu.Lock()
	p.status = status
	p.mu.Unlock()
}

func (p *testProvider) jwksURL() string { return p.server.URL + "/jwks" }

func newTestKeySet(t *testing.T, p *testProvider, clock *fakeClock) *KeySet {
	t.Helper()
	ks, err := NewKeySet(context.Background(), p.jwksURL(), KeySetOptions{
		MinRefreshInterval: 5 * time.Minute,
		MaxAge:             time.Hour,
		Now:                clock.Now,
	})
	if err != nil {
		t.Fatalf("NewKeySet: %v", err)
	}
	return ks
}

func newTestValidator(t *testing.T, keys KeyLookup, clock *fakeClock) *Validator {
	t.Helper()
	v, err := NewValidator(testIssuer, testAudience, keys, clock.Now)
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

// signToken signs claims exactly as given, with exactly the given protected
// headers, so tests control every field Forgejo-facing validation looks at.
func signToken(t *testing.T, alg jwa.SignatureAlgorithm, key any, headers map[string]string, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	h := jws.NewHeaders()
	for name, value := range headers {
		if err := h.Set(name, value); err != nil {
			t.Fatalf("set header %s: %v", name, err)
		}
	}
	signed, err := jws.Sign(payload, jws.WithKey(alg, key, jws.WithProtectedHeaders(h)))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return string(signed)
}

// accessClaims returns a valid access-token claim set at the clock's time.
func accessClaims(clock *fakeClock) map[string]any {
	now := clock.Now().Unix()
	return map[string]any{
		"iss":         testIssuer,
		"sub":         "388443616722288641",
		"aud":         []string{testAudience, "390183248413589505"},
		"client_id":   testAudience,
		"iat":         now,
		"nbf":         now,
		"exp":         now + 3600,
		"jti":         "390190000000000001",
		"forgejo_aud": "u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42",
	}
}
