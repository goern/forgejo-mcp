// SPDX-License-Identifier: GPL-3.0-or-later

package jwtissuer

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	// DiscoveryPath and JWKSPath are appended to the issuer URL. Forgejo builds
	// the discovery URL with url.JoinPath(".well-known/openid-configuration"),
	// so an issuer with a path component serves its documents under that path.
	DiscoveryPath = "/.well-known/openid-configuration"
	JWKSPath      = "/jwks.json"

	// IssuedAtBackdate is how far iat lies before the moment of signing.
	// Forgejo checks iat with zero leeway, so a clock slightly ahead of
	// Forgejo's would otherwise make every token "used before issued".
	IssuedAtBackdate = 60 * time.Second

	// Lifetime is how long a minted token stays valid after signing.
	Lifetime = 300 * time.Second

	// MaxDocumentBytes is Forgejo's cap on each fetched issuer document.
	MaxDocumentBytes = 16 << 10

	jtiBytes = 16
)

// Issuer mints tokens with one signing key and describes itself, and every
// published key, to Forgejo.
type Issuer struct {
	url       string
	signing   *Key
	discovery []byte
	jwks      []byte
}

type discoveryDocument struct {
	Issuer     string   `json:"issuer"`
	JWKSURI    string   `json:"jwks_uri"`
	Algorithms []string `json:"id_token_signing_alg_values_supported"`
}

// New builds an issuer for issuerURL. The signing key is always published;
// published adds keys that are listed in the key set without signing, which is
// how a key is staged before it signs or kept while its last tokens expire.
//
// It refuses two keys with the same key ID, and documents that would exceed
// what Forgejo is willing to fetch.
func New(issuerURL string, signing *Key, published []*Key) (*Issuer, error) {
	if err := checkIssuerURL(issuerURL); err != nil {
		return nil, err
	}
	if signing == nil || signing.signer == nil {
		return nil, fmt.Errorf("a signing key with its private half is required")
	}

	set := jwk.NewSet()
	var algorithms []string
	seen := make(map[string]string)
	for _, k := range append([]*Key{signing}, published...) {
		if k == nil {
			return nil, fmt.Errorf("a published key is missing")
		}
		if other, dup := seen[k.kid]; dup {
			return nil, fmt.Errorf("%s and %s are the same key (key ID %s); list each key once", other, k.source, k.kid)
		}
		seen[k.kid] = k.source
		if err := set.AddKey(k.public); err != nil {
			return nil, fmt.Errorf("%s: cannot add key to the key set: %w", k.source, err)
		}
		algorithms = appendUnique(algorithms, k.alg.String())
	}

	jwks, err := json.Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("cannot encode the key set: %w", err)
	}
	// Every published key's algorithm is listed, not only the signing key's.
	// Forgejo caches the discovery document and the key set together; listing
	// a staged key's algorithm up front means promoting it to signing key does
	// not have to wait for another cache cycle when the algorithms differ.
	discovery, err := json.Marshal(discoveryDocument{
		Issuer:     issuerURL,
		JWKSURI:    issuerURL + JWKSPath,
		Algorithms: algorithms,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode the discovery document: %w", err)
	}
	for name, doc := range map[string][]byte{"key set": jwks, "discovery document": discovery} {
		if len(doc) > MaxDocumentBytes {
			return nil, fmt.Errorf("the %s is %d bytes; Forgejo fetches at most %d, so publish fewer keys", name, len(doc), MaxDocumentBytes)
		}
	}

	return &Issuer{url: issuerURL, signing: signing, discovery: discovery, jwks: jwks}, nil
}

// checkIssuerURL enforces the shape Forgejo and the derived document URLs rely
// on. The scheme policy (https outside loopback) belongs to startup validation
// and is not repeated here.
func checkIssuerURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("issuer URL %q does not parse: %w", raw, err)
	}
	switch {
	case u.Scheme == "" || u.Host == "":
		return fmt.Errorf("issuer URL %q must be absolute, with a scheme and a host", raw)
	case u.User != nil:
		return fmt.Errorf("issuer URL %q must not carry user information", raw)
	case u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#"):
		return fmt.Errorf("issuer URL %q must not carry a query or a fragment", raw)
	case strings.HasSuffix(u.Path, "/"):
		// "https://h/issuer/" would publish jwks_uri "https://h/issuer//jwks.json",
		// and Forgejo compares iss byte for byte, so the two spellings are two
		// different issuers. One spelling is accepted: without the slash.
		return fmt.Errorf("issuer URL %q must not end with a slash", raw)
	}
	return nil
}

// CheckIssuerURL reports whether raw has the shape an issuer URL must have:
// absolute, without user information, query or fragment, and not ending with a
// slash. It does not check the scheme.
func CheckIssuerURL(raw string) error { return checkIssuerURL(raw) }

func appendUnique(list []string, value string) []string {
	for _, v := range list {
		if v == value {
			return list
		}
	}
	return append(list, value)
}

// URL is the issuer identifier, exactly as tokens carry it in iss.
func (i *Issuer) URL() string { return i.url }

// SigningKeyID is the key ID minted tokens carry in their header.
func (i *Issuer) SigningKeyID() string { return i.signing.kid }

// DiscoveryDocument returns the JSON served at URL()+DiscoveryPath. It holds
// exactly issuer, jwks_uri and id_token_signing_alg_values_supported, and
// advertises no endpoints: this issuer is not an authorization server.
func (i *Issuer) DiscoveryDocument() []byte { return append([]byte(nil), i.discovery...) }

// JWKS returns the JSON key set served at URL()+JWKSPath. It holds public
// parameters only.
func (i *Issuer) JWKS() []byte { return append([]byte(nil), i.jwks...) }

// Mint signs a token for subject, addressed to one Forgejo integration
// audience, as of now. The token is valid from IssuedAtBackdate before now
// until Lifetime after it, carries a random jti, and has no nbf.
//
// Validating that audience is usable as a claim value is the caller's job;
// Mint only refuses empty inputs. Errors never contain the token.
func (i *Issuer) Mint(subject, audience string, now time.Time) (string, error) {
	if subject == "" {
		return "", fmt.Errorf("cannot mint a token without a subject")
	}
	if audience == "" {
		return "", fmt.Errorf("cannot mint a token without an audience")
	}

	jti := make([]byte, jtiBytes)
	if _, err := rand.Read(jti); err != nil {
		return "", fmt.Errorf("cannot generate a token ID: %w", err)
	}

	token, err := jwt.NewBuilder().
		Issuer(i.url).
		Subject(subject).
		Audience([]string{audience}).
		IssuedAt(now.Add(-IssuedAtBackdate)).
		Expiration(now.Add(Lifetime)).
		JwtID(base64.RawURLEncoding.EncodeToString(jti)).
		Build()
	if err != nil {
		return "", fmt.Errorf("cannot build the token: %w", err)
	}
	// Forgejo requires exactly one audience. Flattening serialises it as a
	// plain string rather than a one-element array, which is the unambiguous
	// form.
	token.Options().Enable(jwt.FlattenAudience)

	signed, err := jwt.Sign(token, jwt.WithKey(i.signing.alg, i.signing.signer))
	if err != nil {
		return "", fmt.Errorf("cannot sign the token: %w", err)
	}
	return string(signed), nil
}
