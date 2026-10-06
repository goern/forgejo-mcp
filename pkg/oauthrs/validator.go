// SPDX-License-Identifier: GPL-3.0-or-later

package oauthrs

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

var (
	// ErrNoToken means the request carried no bearer token: no Authorization
	// header, an empty one, or a scheme other than Bearer.
	ErrNoToken = errors.New("no bearer token")

	// ErrInvalidToken means a bearer token was presented and refused. The
	// wrapped message says why. It is for the operator's debug log only: a
	// caller able to tell refusals apart could map out what the server expects.
	ErrInvalidToken = errors.New("invalid bearer token")
)

// Leeway is the clock tolerance applied to exp, nbf and iat.
const Leeway = 60 * time.Second

// minRSABits is the smallest RSA modulus a provider key may have.
const minRSABits = 2048

// acceptedAlgorithms are the asymmetric signature algorithms a token may use.
// Symmetric algorithms and "none" are absent on purpose.
var acceptedAlgorithms = map[string]jwa.SignatureAlgorithm{
	"RS256": jwa.RS256(), "RS384": jwa.RS384(), "RS512": jwa.RS512(),
	"PS256": jwa.PS256(), "PS384": jwa.PS384(), "PS512": jwa.PS512(),
	"ES256": jwa.ES256(), "ES384": jwa.ES384(), "ES512": jwa.ES512(),
	"EdDSA": jwa.EdDSA(),
}

// idTokenOnlyClaims are defined by OpenID Connect for ID tokens and never
// appear in access tokens. Some providers give both token kinds the same
// audience, client_id and typ header, so these claims are what tells them
// apart.
var idTokenOnlyClaims = []string{"nonce", "at_hash"}

// KeyLookup finds a provider verification key by key ID.
type KeyLookup interface {
	Lookup(ctx context.Context, kid string) (jwk.Key, error)
}

// Validator decides whether a bearer token is an access token issued for this
// server.
type Validator struct {
	issuer   string
	audience string
	keys     KeyLookup
	now      func() time.Time
}

// NewValidator returns a validator for tokens from issuer that must name
// audience. A nil now selects time.Now.
func NewValidator(issuer, audience string, keys KeyLookup, now func() time.Time) (*Validator, error) {
	switch {
	case issuer == "":
		return nil, errors.New("an issuer is required")
	case audience == "":
		return nil, errors.New("an audience is required")
	case keys == nil:
		return nil, errors.New("a key lookup is required")
	}
	if now == nil {
		now = time.Now
	}
	return &Validator{issuer: issuer, audience: audience, keys: keys, now: now}, nil
}

// Claims are the verified claims of an accepted token.
type Claims struct {
	// Subject is the token's sub claim, or empty when it has none.
	Subject string
	// Raw holds every claim of the verified token as decoded JSON.
	Raw map[string]any
}

// ValidateAuthorization validates the value of an Authorization header. It
// returns ErrNoToken when there is no bearer token, and an error wrapping
// ErrInvalidToken when a token was presented and refused.
func (v *Validator) ValidateAuthorization(ctx context.Context, header string) (*Claims, error) {
	token, ok := bearerToken(header)
	if !ok {
		return nil, ErrNoToken
	}
	claims, err := v.validate(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return claims, nil
}

// bearerToken extracts the token of a Bearer Authorization header. The scheme
// is matched case-insensitively; any other scheme, Forgejo's "token" included,
// is not a bearer token.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func (v *Validator) validate(ctx context.Context, token string) (*Claims, error) {
	if strings.Count(token, ".") != 2 {
		return nil, errors.New("not a JWT in compact serialization")
	}
	msg, err := jws.Parse([]byte(token))
	if err != nil {
		return nil, fmt.Errorf("not a signed JWT: %w", err)
	}
	signatures := msg.Signatures()
	if len(signatures) != 1 {
		return nil, fmt.Errorf("carries %d signatures, want 1", len(signatures))
	}
	header := signatures[0].ProtectedHeaders()

	declared, ok := header.Algorithm()
	if !ok {
		return nil, errors.New("header has no alg")
	}
	alg, ok := acceptedAlgorithms[declared.String()]
	if !ok {
		return nil, fmt.Errorf("algorithm %q is not an accepted asymmetric algorithm", declared.String())
	}
	if typ, ok := header.Type(); ok && !strings.EqualFold(typ, "at+jwt") && !strings.EqualFold(typ, "JWT") {
		return nil, fmt.Errorf("typ %q does not declare an access token", typ)
	}
	kid, ok := header.KeyID()
	if !ok || kid == "" {
		return nil, errors.New("header has no kid")
	}

	key, err := v.keys.Lookup(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("key %q: %w", kid, err)
	}
	if err := keyAllows(key, alg); err != nil {
		return nil, fmt.Errorf("key %q: %w", kid, err)
	}
	if _, err := jws.Verify([]byte(token), jws.WithKey(alg, key)); err != nil {
		return nil, fmt.Errorf("signature does not verify: %w", err)
	}

	raw, err := payloadClaims(token)
	if err != nil {
		return nil, err
	}
	for _, name := range idTokenOnlyClaims {
		if _, found := raw[name]; found {
			return nil, fmt.Errorf("carries %q, a claim only ID tokens carry", name)
		}
	}
	if _, found := raw["exp"]; !found {
		return nil, errors.New("has no exp")
	}

	if _, err := jwt.Parse([]byte(token),
		jwt.WithVerify(false),
		jwt.WithValidate(true),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithRequiredClaim("exp"),
		jwt.WithAcceptableSkew(Leeway),
		jwt.WithClock(jwt.ClockFunc(v.now)),
	); err != nil {
		return nil, fmt.Errorf("claims: %w", err)
	}
	// iat in the future is checked explicitly rather than trusting a library
	// default: a token "issued" later than now is refused beyond the leeway.
	if iat, ok := numericClaim(raw, "iat"); ok && iat.After(v.now().Add(Leeway)) {
		return nil, errors.New("iat lies in the future")
	}

	subject, _ := raw["sub"].(string)
	return &Claims{Subject: subject, Raw: raw}, nil
}

// keyAllows checks that the provider key is of a type the declared algorithm
// can use, and that a key which names its own algorithm names this one. The
// algorithm is thereby bound to the key, not taken from the token alone.
func keyAllows(key jwk.Key, alg jwa.SignatureAlgorithm) error {
	name := alg.String()
	if own, ok := key.Algorithm(); ok && own.String() != name {
		return fmt.Errorf("is for %s, but the token declares %s", own.String(), name)
	}
	var raw any
	if err := jwk.Export(key, &raw); err != nil {
		return fmt.Errorf("cannot use key: %w", err)
	}
	switch k := raw.(type) {
	case *rsa.PublicKey:
		if !strings.HasPrefix(name, "RS") && !strings.HasPrefix(name, "PS") {
			return fmt.Errorf("is an RSA key, but the token declares %s", name)
		}
		if k.N.BitLen() < minRSABits {
			return fmt.Errorf("is an RSA key of %d bits; at least %d are required", k.N.BitLen(), minRSABits)
		}
	case *ecdsa.PublicKey:
		want := map[string]elliptic.Curve{"ES256": elliptic.P256(), "ES384": elliptic.P384(), "ES512": elliptic.P521()}[name]
		if want == nil || k.Curve != want {
			return fmt.Errorf("is an EC %s key, but the token declares %s", k.Curve.Params().Name, name)
		}
	case ed25519.PublicKey:
		if name != "EdDSA" {
			return fmt.Errorf("is an Ed25519 key, but the token declares %s", name)
		}
	default:
		return fmt.Errorf("has unsupported type %T", raw)
	}
	return nil
}

// payloadClaims decodes the claims segment of an already verified token.
func payloadClaims(token string) (map[string]any, error) {
	segment := strings.Split(token, ".")[1]
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, fmt.Errorf("claims are not base64url: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, fmt.Errorf("claims are not a JSON object: %w", err)
	}
	return claims, nil
}

func numericClaim(claims map[string]any, name string) (time.Time, bool) {
	v, ok := claims[name].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(v), 0), true
}
