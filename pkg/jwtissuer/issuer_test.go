// SPDX-License-Identifier: GPL-3.0-or-later

package jwtissuer

import (
	"crypto"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
)

const testIssuerURL = "https://mcp.example.org/issuer"

func loadSigning(t *testing.T, key crypto.PrivateKey) *Key {
	t.Helper()
	k, err := LoadSigningKey(writePEM(t, "PRIVATE KEY", pkcs8(t, key)))
	if err != nil {
		t.Fatalf("LoadSigningKey: %v", err)
	}
	return k
}

func newIssuer(t *testing.T, signing *Key, published ...*Key) *Issuer {
	t.Helper()
	iss, err := New(testIssuerURL, signing, published)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return iss
}

// Spec forgejo-jwt-issuer, scenario "Forgejo fetches discovery".
func TestDiscoveryDocumentHasExactlyThreeMembers(t *testing.T) {
	iss := newIssuer(t, loadSigning(t, mustECDSA(t, elliptic.P256())))

	var doc map[string]any
	if err := json.Unmarshal(iss.DiscoveryDocument(), &doc); err != nil {
		t.Fatalf("discovery document is not JSON: %v", err)
	}
	if len(doc) != 3 {
		t.Fatalf("discovery document has %d members, want exactly 3: %v", len(doc), doc)
	}
	if doc["issuer"] != testIssuerURL {
		t.Fatalf("issuer = %v, want %q", doc["issuer"], testIssuerURL)
	}
	if doc["jwks_uri"] != testIssuerURL+"/jwks.json" {
		t.Fatalf("jwks_uri = %v, want %q", doc["jwks_uri"], testIssuerURL+"/jwks.json")
	}
	algs, ok := doc["id_token_signing_alg_values_supported"].([]any)
	if !ok || len(algs) != 1 || algs[0] != "ES256" {
		t.Fatalf("id_token_signing_alg_values_supported = %v, want [ES256]", doc["id_token_signing_alg_values_supported"])
	}
}

func TestDiscoveryListsTheSigningAlgorithmFirst(t *testing.T) {
	staged, err := LoadPublishedKey(writePEM(t, "PUBLIC KEY", pkix(t, mustEd25519(t).Public())))
	if err != nil {
		t.Fatal(err)
	}
	iss := newIssuer(t, loadSigning(t, mustECDSA(t, elliptic.P384())), staged)

	var doc discoveryDocument
	if err := json.Unmarshal(iss.DiscoveryDocument(), &doc); err != nil {
		t.Fatal(err)
	}
	if strings.Join(doc.Algorithms, ",") != "ES384,EdDSA" {
		t.Fatalf("algorithms = %v, want [ES384 EdDSA]", doc.Algorithms)
	}
}

// Spec forgejo-jwt-issuer, scenarios "Private key material never published"
// and "Key published ahead of use".
func TestKeySetPublishesPublicParametersOfEveryKey(t *testing.T) {
	signing := loadSigning(t, mustECDSA(t, elliptic.P256()))
	// A published key given as a PRIVATE key file must still be published as
	// public parameters only.
	staged, err := LoadPublishedKey(writePEM(t, "PRIVATE KEY", pkcs8(t, mustEd25519(t))))
	if err != nil {
		t.Fatal(err)
	}
	iss := newIssuer(t, signing, staged)

	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(iss.JWKS(), &set); err != nil {
		t.Fatalf("key set is not JSON: %v", err)
	}
	if len(set.Keys) != 2 {
		t.Fatalf("key set has %d keys, want 2", len(set.Keys))
	}
	var kids []string
	for _, key := range set.Keys {
		for _, private := range []string{"d", "p", "q", "dp", "dq", "qi"} {
			if _, found := key[private]; found {
				t.Fatalf("key set leaks private member %q: %v", private, key)
			}
		}
		if key["use"] != "sig" {
			t.Fatalf("key use = %v, want sig", key["use"])
		}
		if key["alg"] == nil || key["kid"] == nil {
			t.Fatalf("key lacks alg or kid: %v", key)
		}
		kids = append(kids, key["kid"].(string))
	}
	sort.Strings(kids)
	want := []string{signing.KeyID(), staged.KeyID()}
	sort.Strings(want)
	if strings.Join(kids, ",") != strings.Join(want, ",") {
		t.Fatalf("key IDs = %v, want %v", kids, want)
	}

	// Publishing a key ahead of use must not make it sign.
	token, err := iss.Mint("subject", "u:1:audience", time.Now())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if header := decodeSegment(t, strings.Split(token, ".")[0]); header["kid"] != signing.KeyID() || header["alg"] != "ES256" {
		t.Fatalf("minted with %v, want kid %q and ES256 of the signing key", header, signing.KeyID())
	}
}

func TestTheSameKeyListedTwiceIsRefused(t *testing.T) {
	ed := mustEd25519(t)
	signing := loadSigning(t, ed)
	again, err := LoadPublishedKey(writePEM(t, "PUBLIC KEY", pkix(t, ed.Public())))
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(testIssuerURL, signing, []*Key{again})
	if err == nil || !strings.Contains(err.Error(), "same key") {
		t.Fatalf("New error = %v, want a refusal of the duplicate key", err)
	}
}

func TestIssuerURLShape(t *testing.T) {
	signing := loadSigning(t, mustEd25519(t))
	for _, ok := range []string{"https://mcp.example.org/issuer", "https://mcp.example.org"} {
		if _, err := New(ok, signing, nil); err != nil {
			t.Errorf("New(%q) refused a valid issuer URL: %v", ok, err)
		}
	}
	for _, bad := range []string{
		"https://mcp.example.org/issuer/",
		"https://mcp.example.org/",
		"https://mcp.example.org/issuer?x=1",
		"https://mcp.example.org/issuer#frag",
		"https://user@mcp.example.org/issuer",
		"/issuer",
		"",
	} {
		if _, err := New(bad, signing, nil); err == nil {
			t.Errorf("New(%q) accepted an issuer URL it must refuse", bad)
		}
	}
}

// Forgejo fetches at most 16 KiB per document; a configuration that would
// publish more is refused instead of producing an issuer Forgejo cannot read.
func TestDocumentsBeyondForgejosLimitAreRefused(t *testing.T) {
	signing := loadSigning(t, mustEd25519(t))
	var published []*Key
	for range 200 {
		k, err := LoadPublishedKey(writePEM(t, "PUBLIC KEY", pkix(t, mustEd25519(t).Public())))
		if err != nil {
			t.Fatal(err)
		}
		published = append(published, k)
	}
	_, err := New(testIssuerURL, signing, published)
	if err == nil || !strings.Contains(err.Error(), "at most 16384") {
		t.Fatalf("New error = %v, want a refusal naming the 16 KiB limit", err)
	}
}

// decodeSegment decodes one base64url JWT segment into a JSON object.
func decodeSegment(t *testing.T, segment string) map[string]any {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatalf("segment is not base64url: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("segment is not a JSON object: %v", err)
	}
	return out
}

// Spec forgejo-jwt-issuer, scenario "Claims of a minted JWT".
func TestMintedTokenShape(t *testing.T) {
	signing := loadSigning(t, mustECDSA(t, elliptic.P256()))
	iss := newIssuer(t, signing)
	now := time.Unix(1789063466, 0)
	const (
		subject  = "388443616722288641"
		audience = "u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42"
	)

	token, err := iss.Mint(subject, audience, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}

	header := decodeSegment(t, parts[0])
	if header["alg"] != "ES256" || header["typ"] != "JWT" || header["kid"] != signing.KeyID() {
		t.Fatalf("header = %v, want alg ES256, typ JWT, kid %q", header, signing.KeyID())
	}

	claims := decodeSegment(t, parts[1])
	var names []string
	for name := range claims {
		names = append(names, name)
	}
	sort.Strings(names)
	if got := strings.Join(names, ","); got != "aud,exp,iat,iss,jti,sub" {
		t.Fatalf("claims = %s, want exactly aud,exp,iat,iss,jti,sub", got)
	}
	if claims["iss"] != testIssuerURL || claims["sub"] != subject {
		t.Fatalf("iss/sub = %v/%v", claims["iss"], claims["sub"])
	}
	if claims["aud"] != audience {
		t.Fatalf("aud = %#v, want the single string %q", claims["aud"], audience)
	}
	if iat, _ := claims["iat"].(float64); int64(iat) != now.Unix()-60 {
		t.Fatalf("iat = %v, want %d (60 s before signing)", claims["iat"], now.Unix()-60)
	}
	if exp, _ := claims["exp"].(float64); int64(exp) != now.Unix()+300 {
		t.Fatalf("exp = %v, want %d (300 s after signing)", claims["exp"], now.Unix()+300)
	}
	jti, err := base64.RawURLEncoding.DecodeString(claims["jti"].(string))
	if err != nil || len(jti) < 16 {
		t.Fatalf("jti %v is not at least 128 random bits", claims["jti"])
	}

	// The decoded header and claims, never the signature, so that the Showboat
	// demo can show what Forgejo receives.
	rawHeader, _ := base64.RawURLEncoding.DecodeString(parts[0])
	rawClaims, _ := base64.RawURLEncoding.DecodeString(parts[1])
	t.Logf("header %s", rawHeader)
	t.Logf("claims %s", rawClaims)
}

// Every key type Forgejo accepts yields a token that verifies against the key
// set the issuer publishes.
func TestMintedTokenVerifiesAgainstThePublishedKeySet(t *testing.T) {
	cases := map[string]crypto.PrivateKey{
		"ES256": mustECDSA(t, elliptic.P256()),
		"ES384": mustECDSA(t, elliptic.P384()),
		"EdDSA": mustEd25519(t),
		"RS256": mustRSA(t, 2048),
	}
	for alg, key := range cases {
		t.Run(alg, func(t *testing.T) {
			iss := newIssuer(t, loadSigning(t, key))
			token, err := iss.Mint("subject", "u:1:audience", time.Now())
			if err != nil {
				t.Fatalf("Mint: %v", err)
			}
			set, err := jwk.Parse(iss.JWKS())
			if err != nil {
				t.Fatalf("parse published key set: %v", err)
			}
			if _, err := jws.Verify([]byte(token), jws.WithKeySet(set)); err != nil {
				t.Fatalf("token does not verify against the published key set: %v", err)
			}
			if header := decodeSegment(t, strings.Split(token, ".")[0]); header["alg"] != alg {
				t.Fatalf("header alg = %v, want %s", header["alg"], alg)
			}
		})
	}
}

// Spec forgejo-jwt-issuer, scenario "Two MCP requests".
func TestMintedTokensDifferInJTI(t *testing.T) {
	iss := newIssuer(t, loadSigning(t, mustEd25519(t)))
	now := time.Now()
	a, err := iss.Mint("subject", "u:1:audience", now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := iss.Mint("subject", "u:1:audience", now)
	if err != nil {
		t.Fatal(err)
	}
	jtiA := decodeSegment(t, strings.Split(a, ".")[1])["jti"]
	jtiB := decodeSegment(t, strings.Split(b, ".")[1])["jti"]
	if jtiA == jtiB {
		t.Fatalf("two tokens share jti %v", jtiA)
	}
}

func TestMintRefusesEmptySubjectOrAudience(t *testing.T) {
	iss := newIssuer(t, loadSigning(t, mustEd25519(t)))
	if _, err := iss.Mint("", "u:1:audience", time.Now()); err == nil {
		t.Error("Mint accepted an empty subject")
	}
	if _, err := iss.Mint("subject", "", time.Now()); err == nil {
		t.Error("Mint accepted an empty audience")
	}
}
