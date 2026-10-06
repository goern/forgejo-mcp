// SPDX-License-Identifier: GPL-3.0-or-later

package oauthrs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ProviderMetadata is the part of an identity provider's metadata this server
// relies on.
type ProviderMetadata struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// DiscoverProvider reads the provider's metadata for issuer. It tries OpenID
// Connect discovery first and the RFC 8414 location second, and it requires
// the reported issuer to equal the configured one exactly and a jwks_uri to be
// present.
//
// A nil client selects one that does not follow redirects.
func DiscoverProvider(ctx context.Context, client *http.Client, issuer string) (*ProviderMetadata, error) {
	u, err := url.Parse(issuer)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("provider issuer %q is not an absolute URL", issuer)
	}
	if client == nil {
		client = newHTTPClient()
	}

	var failures []error
	for _, location := range metadataLocations(u, issuer) {
		body, err := getJSON(ctx, client, location)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		var m ProviderMetadata
		if err := json.Unmarshal(body, &m); err != nil {
			failures = append(failures, fmt.Errorf("%s: not a JSON metadata document: %w", location, err))
			continue
		}
		// A document that was found but names another issuer is a
		// misconfiguration, not a reason to keep looking. Saying so plainly
		// beats reporting that nothing was found.
		if m.Issuer != issuer {
			return nil, fmt.Errorf("provider metadata at %s reports issuer %q, but %q is configured; the two must match exactly", location, m.Issuer, issuer)
		}
		if m.JWKSURI == "" {
			return nil, fmt.Errorf("provider metadata at %s names no jwks_uri", location)
		}
		if err := checkJWKSURI(m.JWKSURI, u.Scheme); err != nil {
			return nil, fmt.Errorf("provider metadata at %s: %w", location, err)
		}
		return &m, nil
	}
	return nil, fmt.Errorf("cannot read provider metadata for %s: %w", issuer, errors.Join(failures...))
}

// metadataLocations lists where the metadata may live, in the order tried.
//
// OpenID Connect Discovery appends to the issuer, dropping a trailing slash.
// RFC 8414 inserts the well-known segment between the host and the issuer's
// path.
func metadataLocations(u *url.URL, issuer string) []string {
	oidc := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"

	rfc := *u
	rfc.Path = "/.well-known/oauth-authorization-server" + strings.TrimSuffix(u.Path, "/")
	rfc.RawPath = ""
	return []string{oidc, rfc.String()}
}

// checkJWKSURI requires an absolute http(s) URL. Plain http is accepted only
// when the issuer itself is http, which only happens in local development.
func checkJWKSURI(raw, issuerScheme string) error {
	j, err := url.Parse(raw)
	if err != nil || !j.IsAbs() || j.Host == "" {
		return fmt.Errorf("jwks_uri %q is not an absolute URL", raw)
	}
	switch j.Scheme {
	case "https":
		return nil
	case "http":
		if issuerScheme == "http" {
			return nil
		}
	}
	return fmt.Errorf("jwks_uri %q must use https", raw)
}
