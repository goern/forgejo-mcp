// SPDX-License-Identifier: GPL-3.0-or-later

package oauthrs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
)

// Spec oauth-resource-server, scenario "Burst of unknown key IDs".
func TestUnknownKeyIDsTriggerAtMostOneFetchPerInterval(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	ks := newTestKeySet(t, p, clock)
	v := newTestValidator(t, ks, clock)
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("fetches after start = %d, want 1", got)
	}

	// Past the floor, so exactly one refetch is allowed for the whole burst.
	clock.Advance(6 * time.Minute)
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token := signToken(t, jwa.ES256(), key, map[string]string{"kid": fmt.Sprintf("unknown-%d", i)}, accessClaims(clock))
			if _, err := v.ValidateAuthorization(context.Background(), bearer(token)); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("unknown kid: error = %v, want ErrInvalidToken", err)
			}
		}()
	}
	wg.Wait()
	if got := p.fetches.Load(); got != 2 {
		t.Fatalf("fetches after a burst of 50 unknown key IDs = %d, want 2", got)
	}
}

func TestAKeyAddedAtTheProviderIsUsedOnceTheIntervalHasPassed(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	rotated := p.addECKey("ec-2")
	token := func() string {
		return signToken(t, jwa.ES256(), rotated, map[string]string{"kid": "ec-2"}, accessClaims(clock))
	}
	if _, err := v.ValidateAuthorization(context.Background(), bearer(token())); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("new key accepted before the refresh interval: %v", err)
	}
	if got := p.fetches.Load(); got != 1 {
		t.Fatalf("fetches within the interval = %d, want 1", got)
	}

	clock.Advance(5*time.Minute + time.Second)
	if _, err := v.ValidateAuthorization(context.Background(), bearer(token())); err != nil {
		t.Fatalf("new key refused after the refresh interval: %v", err)
	}
}

// Spec oauth-resource-server, scenario "Caller disconnects during a refetch".
func TestARefetchCompletesWhenTheRequestThatTriggeredItIsCancelled(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	p.addECKey("ec-1")
	ks := newTestKeySet(t, p, clock)

	// The provider rotates, and a request for the new key arrives past the
	// floor, but its client has already gone away.
	clock.Advance(6 * time.Minute)
	p.addECKey("ec-2")
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ks.Lookup(gone, "ec-2"); err != nil {
		t.Fatalf("a cancelled caller failed the refetch: %v", err)
	}
	if got := p.fetches.Load(); got != 2 {
		t.Fatalf("fetches = %d, want 2: the one at start and the refetch", got)
	}

	// A later request inside the same interval finds the rotated key without
	// another fetch.
	if _, err := ks.Lookup(context.Background(), "ec-2"); err != nil {
		t.Fatalf("the rotated key is not known after the refetch: %v", err)
	}
	if got := p.fetches.Load(); got != 2 {
		t.Fatalf("fetches = %d, want still 2", got)
	}
}

func TestAWithdrawnKeyStopsWorkingOnceTheSetIsTooOld(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	p.addECKey("ec-2")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	p.removeKey("ec-1")
	clock.Advance(time.Hour + time.Second)
	token := signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1"}, accessClaims(clock))
	if _, err := v.ValidateAuthorization(context.Background(), bearer(token)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("withdrawn key still accepted after the maximum age: %v", err)
	}
}

func TestKnownKeysKeepWorkingDuringAProviderOutage(t *testing.T) {
	clock := newFakeClock()
	p := newTestProvider(t)
	key := p.addECKey("ec-1")
	v := newTestValidator(t, newTestKeySet(t, p, clock), clock)

	p.setStatus(http.StatusInternalServerError)
	clock.Advance(time.Hour + time.Second)
	token := signToken(t, jwa.ES256(), key, map[string]string{"kid": "ec-1"}, accessClaims(clock))
	if _, err := v.ValidateAuthorization(context.Background(), bearer(token)); err != nil {
		t.Fatalf("known key refused while the provider was down: %v", err)
	}
}

func TestKeySetKeepsOnlyUsableSigningKeys(t *testing.T) {
	serve := func(body string) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	unusable := `{"keys":[
		{"kty":"oct","kid":"sym","k":"c2VjcmV0"},
		{"kty":"EC","use":"enc","kid":"enc","crv":"P-256","x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"},
		{"kty":"EC","crv":"P-256","x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}
	]}`
	if _, err := NewKeySet(context.Background(), serve(unusable), KeySetOptions{}); err == nil || !strings.Contains(err.Error(), "no asymmetric signing key") {
		t.Fatalf("NewKeySet over only unusable keys: error = %v", err)
	}

	usable := `{"keys":[
		{"kty":"oct","kid":"sym","k":"c2VjcmV0"},
		{"kty":"EC","use":"sig","kid":"good","crv":"P-256","x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}
	]}`
	ks, err := NewKeySet(context.Background(), serve(usable), KeySetOptions{})
	if err != nil {
		t.Fatalf("NewKeySet: %v", err)
	}
	if _, err := ks.Lookup(context.Background(), "good"); err != nil {
		t.Fatalf("usable key not found: %v", err)
	}
	if _, err := ks.Lookup(context.Background(), "sym"); err == nil {
		t.Fatal("symmetric key was kept")
	}
}

func TestNewKeySetFailsWhenTheSetCannotBeFetched(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	if _, err := NewKeySet(context.Background(), srv.URL+"/jwks", KeySetOptions{}); err == nil {
		t.Fatal("NewKeySet succeeded against a 404")
	}
}

// metadataServer serves JSON documents by path.
func metadataServer(t *testing.T, docs map[string]func(base string) map[string]any) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		build, ok := docs[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(build(srv.URL))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoveryPrefersOpenIDConnect(t *testing.T) {
	srv := metadataServer(t, map[string]func(string) map[string]any{
		"/.well-known/openid-configuration": func(base string) map[string]any {
			return map[string]any{"issuer": base, "jwks_uri": base + "/keys"}
		},
	})
	m, err := DiscoverProvider(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatalf("DiscoverProvider: %v", err)
	}
	if m.JWKSURI != srv.URL+"/keys" {
		t.Fatalf("jwks_uri = %q", m.JWKSURI)
	}
}

func TestDiscoveryFallsBackToRFC8414ForAnIssuerWithAPath(t *testing.T) {
	srv := metadataServer(t, map[string]func(string) map[string]any{
		"/.well-known/oauth-authorization-server/tenant": func(base string) map[string]any {
			return map[string]any{"issuer": base + "/tenant", "jwks_uri": base + "/tenant/keys"}
		},
	})
	if _, err := DiscoverProvider(context.Background(), nil, srv.URL+"/tenant"); err != nil {
		t.Fatalf("DiscoverProvider with RFC 8414 location: %v", err)
	}
}

// Spec oauth-resource-server, scenario "Provider issuer mismatch".
func TestDiscoveryRefusesAnIssuerThatDiffersByATrailingSlash(t *testing.T) {
	srv := metadataServer(t, map[string]func(string) map[string]any{
		"/.well-known/openid-configuration": func(base string) map[string]any {
			return map[string]any{"issuer": base + "/", "jwks_uri": base + "/keys"}
		},
	})
	_, err := DiscoverProvider(context.Background(), nil, srv.URL)
	if err == nil || !strings.Contains(err.Error(), srv.URL+"/") || !strings.Contains(err.Error(), fmt.Sprintf("%q", srv.URL)) {
		t.Fatalf("error = %v, want a refusal showing both issuer values", err)
	}
}

func TestDiscoveryRefusesMetadataWithoutAKeySet(t *testing.T) {
	srv := metadataServer(t, map[string]func(string) map[string]any{
		"/.well-known/openid-configuration": func(base string) map[string]any {
			return map[string]any{"issuer": base}
		},
	})
	if _, err := DiscoverProvider(context.Background(), nil, srv.URL); err == nil || !strings.Contains(err.Error(), "jwks_uri") {
		t.Fatalf("error = %v, want a refusal naming jwks_uri", err)
	}
}

func TestDiscoveryFailsForAnUnreachableProvider(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	unreachable := srv.URL
	srv.Close()
	if _, err := DiscoverProvider(context.Background(), nil, unreachable); err == nil {
		t.Fatal("DiscoverProvider succeeded against a closed server")
	}
}

func TestDiscoveryDoesNotFollowRedirects(t *testing.T) {
	target := metadataServer(t, map[string]func(string) map[string]any{
		"/.well-known/openid-configuration": func(base string) map[string]any {
			return map[string]any{"issuer": base, "jwks_uri": base + "/keys"}
		},
	})
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)
	if _, err := DiscoverProvider(context.Background(), nil, redirector.URL); err == nil {
		t.Fatal("DiscoverProvider followed a redirect")
	}
}
