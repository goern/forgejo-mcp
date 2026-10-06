// SPDX-License-Identifier: GPL-3.0-or-later

// Package oauthrs holds the inbound half of forgejo-mcp's resource-server mode:
// reading the identity provider's metadata, keeping its signing keys, and
// validating the JWT access tokens callers present. It decides whether a token
// is acceptable and nothing more. Turning a refusal into an HTTP response is
// the transport's job.
//
// Nothing here logs. Errors carry the reason a token was refused so that an
// operator's debug log can say why, and they must never be shown to a caller;
// see ErrInvalidToken.
package oauthrs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

const (
	// DefaultMinRefreshInterval bounds how often tokens naming unknown key IDs
	// can make the server fetch the provider's key set. Without a floor, a
	// stranger sending random key IDs turns every request into a fetch.
	DefaultMinRefreshInterval = 5 * time.Minute

	// DefaultMaxKeySetAge makes a key set be refetched, lazily, once it is this
	// old. A key the provider has withdrawn stops being accepted after at most
	// this long, instead of for as long as the process runs.
	DefaultMaxKeySetAge = time.Hour

	maxDocumentBytes = 1 << 20
	fetchTimeout     = 10 * time.Second
)

var errUnknownKey = errors.New("no provider key has the token's key ID")

// KeySetOptions tunes a KeySet. Zero values select the defaults.
type KeySetOptions struct {
	Client             *http.Client
	MinRefreshInterval time.Duration
	MaxAge             time.Duration
	Now                func() time.Time
}

// KeySet holds the provider's current verification keys.
type KeySet struct {
	url         string
	client      *http.Client
	minInterval time.Duration
	maxAge      time.Duration
	now         func() time.Time

	mu        sync.RWMutex
	set       jwk.Set
	fetchedAt time.Time

	// refresh serialises fetches; lastAttempt is only read and written while
	// it is held.
	refresh     sync.Mutex
	lastAttempt time.Time
}

// NewKeySet fetches the provider's key set once and fails if it cannot, or if
// the set holds no usable signing key. A server that starts without keys would
// refuse every caller and give the operator no hint why.
func NewKeySet(ctx context.Context, jwksURL string, opts KeySetOptions) (*KeySet, error) {
	k := &KeySet{
		url:         jwksURL,
		client:      opts.Client,
		minInterval: opts.MinRefreshInterval,
		maxAge:      opts.MaxAge,
		now:         opts.Now,
	}
	if k.client == nil {
		k.client = newHTTPClient()
	}
	if k.minInterval <= 0 {
		k.minInterval = DefaultMinRefreshInterval
	}
	if k.maxAge <= 0 {
		k.maxAge = DefaultMaxKeySetAge
	}
	if k.now == nil {
		k.now = time.Now
	}

	k.lastAttempt = k.now()
	if err := k.fetch(ctx); err != nil {
		return nil, err
	}
	return k, nil
}

// Lookup returns the verification key with the given key ID.
//
// A key ID that is not in the cached set, or a cached set older than its
// maximum age, causes a refetch, but at most one per minimum refresh interval
// however many requests arrive. If a refetch fails, keys already known keep
// working: an outage at the provider must not refuse every caller.
func (k *KeySet) Lookup(ctx context.Context, kid string) (jwk.Key, error) {
	if key, ok := k.fresh(kid); ok {
		return key, nil
	}

	k.refresh.Lock()
	defer k.refresh.Unlock()

	// Another request may have refreshed the set while this one waited.
	if key, ok := k.fresh(kid); ok {
		return key, nil
	}
	if k.now().Sub(k.lastAttempt) < k.minInterval {
		return k.known(kid)
	}
	k.lastAttempt = k.now()
	// Detach the fetch from the request that triggered it. A cancellable fetch
	// lets a client that disconnects mid-fetch fail the refetch and still spend
	// the interval, and by doing that once per interval a stranger could keep
	// the key set from ever learning a rotated key. getJSON bounds the fetch
	// with its own timeout either way.
	if err := k.fetch(context.WithoutCancel(ctx)); err != nil {
		if key, lookupErr := k.known(kid); lookupErr == nil {
			return key, nil
		}
		return nil, err
	}
	return k.known(kid)
}

// fresh looks the key up only if the cached set is younger than its maximum age.
func (k *KeySet) fresh(kid string) (jwk.Key, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.now().Sub(k.fetchedAt) >= k.maxAge {
		return nil, false
	}
	return k.set.LookupKeyID(kid)
}

// known looks the key up in the cached set regardless of its age.
func (k *KeySet) known(kid string) (jwk.Key, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	if key, ok := k.set.LookupKeyID(kid); ok {
		return key, nil
	}
	return nil, errUnknownKey
}

func (k *KeySet) fetch(ctx context.Context) error {
	body, err := getJSON(ctx, k.client, k.url)
	if err != nil {
		return fmt.Errorf("fetch provider key set: %w", err)
	}
	parsed, err := jwk.Parse(body)
	if err != nil {
		return fmt.Errorf("parse provider key set from %s: %w", k.url, err)
	}
	usable, err := verificationKeys(parsed)
	if err != nil {
		return fmt.Errorf("provider key set at %s: %w", k.url, err)
	}
	k.mu.Lock()
	k.set, k.fetchedAt = usable, k.now()
	k.mu.Unlock()
	return nil
}

// verificationKeys keeps the public half of every asymmetric key that is meant
// for signatures and can be selected by key ID. Symmetric keys, encryption
// keys and keys without an ID are dropped; a set left empty is an error.
func verificationKeys(set jwk.Set) (jwk.Set, error) {
	out := jwk.NewSet()
	for i := range set.Len() {
		key, ok := set.Key(i)
		if !ok {
			continue
		}
		switch key.KeyType() {
		case jwa.RSA(), jwa.EC(), jwa.OKP():
		default:
			continue
		}
		if use, ok := key.KeyUsage(); ok && use != string(jwk.ForSignature) {
			continue
		}
		if kid, ok := key.KeyID(); !ok || kid == "" {
			continue
		}
		public, err := jwk.PublicKeyOf(key)
		if err != nil {
			continue
		}
		if err := out.AddKey(public); err != nil {
			return nil, fmt.Errorf("add key: %w", err)
		}
	}
	if out.Len() == 0 {
		return nil, errors.New("holds no asymmetric signing key with a key ID")
	}
	return out, nil
}

// newHTTPClient returns the client used for the provider's documents. It does
// not follow redirects: the operator configured these URLs, and a provider
// that starts redirecting them should be noticed, not silently followed.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: fetchTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// getJSON fetches a document of at most maxDocumentBytes and requires a 200.
func getJSON(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: read body: %w", url, err)
	}
	if len(body) > maxDocumentBytes {
		return nil, fmt.Errorf("GET %s: document exceeds %d bytes", url, maxDocumentBytes)
	}
	return body, nil
}
