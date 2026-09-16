// SPDX-License-Identifier: GPL-3.0-or-later

// Package jwtissuer makes forgejo-mcp an issuer of short-lived JWTs that
// Forgejo 16 accepts through Authorized Integrations. It loads the signing
// key, mints a token for an authenticated caller, and renders the OpenID
// discovery document and key set Forgejo fetches to verify those tokens.
//
// Nothing in this package logs. Every value it handles is either key material
// or a credential, and the only safe amount of either in a log line is none.
package jwtissuer

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// minRSABits is the smallest RSA modulus accepted. Forgejo itself would verify
// a smaller one, but this key becomes a credential for every user whose
// integration trusts the issuer, so a weak one is refused here.
const minRSABits = 2048

// Key is one asymmetric key the issuer signs with or publishes.
type Key struct {
	// source is the file the key came from. It appears in error messages so an
	// operator knows which setting to fix; it is never key material.
	source string
	alg    jwa.SignatureAlgorithm
	kid    string
	// signer holds the private half and is nil when only a public key was
	// loaded. It carries the kid, so a signature names the key that made it.
	signer jwk.Key
	// public is what the key set publishes: public parameters plus kid, use
	// and alg, never private material.
	public jwk.Key
}

// KeyID is the RFC 7638 thumbprint of the public key, base64url without
// padding.
func (k *Key) KeyID() string { return k.kid }

// Algorithm is the JWS algorithm the key type determines, e.g. "ES256".
func (k *Key) Algorithm() string { return k.alg.String() }

// Source is the path the key was loaded from.
func (k *Key) Source() string { return k.source }

// LoadSigningKey reads a PEM-encoded private key the issuer signs with. A file
// holding only a public key is refused: it cannot sign.
func LoadSigningKey(path string) (*Key, error) {
	k, err := loadKey(path)
	if err != nil {
		return nil, err
	}
	if k.signer == nil {
		return nil, fmt.Errorf("%s: holds a public key, but signing needs a private key", path)
	}
	return k, nil
}

// LoadPublishedKey reads a PEM-encoded public or private key that the issuer
// publishes without signing with it, for example a key staged for rotation.
func LoadPublishedKey(path string) (*Key, error) {
	return loadKey(path)
}

func loadKey(path string) (*Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: cannot read key file: %w", path, err)
	}
	raw, err := parsePEM(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	k, err := newKey(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	k.source = path
	return k, nil
}

// parsePEM decodes exactly one PEM block. A file with several blocks is
// refused rather than guessed at: which key signs must never depend on the
// order someone concatenated files in.
func parsePEM(data []byte) (any, error) {
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if strings.TrimSpace(string(rest)) != "" {
		return nil, fmt.Errorf("more than one PEM block; put one key in each file")
	}
	switch block.Type {
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	case "ENCRYPTED PRIVATE KEY":
		return nil, fmt.Errorf("encrypted private keys are not supported; supply the key decrypted, with file permissions protecting it")
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}
}

// newKey maps a parsed key to its algorithm and builds the JWK forms. The
// mapping is fixed: the key type decides the algorithm, never configuration.
func newKey(raw any) (*Key, error) {
	var (
		alg     jwa.SignatureAlgorithm
		private crypto.PrivateKey
		public  crypto.PublicKey
		err     error
	)
	switch v := raw.(type) {
	case *ecdsa.PrivateKey:
		private, public = v, &v.PublicKey
		alg, err = ecAlgorithm(v.Curve)
	case *ecdsa.PublicKey:
		public = v
		alg, err = ecAlgorithm(v.Curve)
	case ed25519.PrivateKey:
		private, public = v, v.Public()
		alg = jwa.EdDSA()
	case ed25519.PublicKey:
		public = v
		alg = jwa.EdDSA()
	case *rsa.PrivateKey:
		private, public = v, &v.PublicKey
		alg, err = rsaAlgorithm(v.N.BitLen())
	case *rsa.PublicKey:
		public = v
		alg, err = rsaAlgorithm(v.N.BitLen())
	default:
		return nil, fmt.Errorf("unsupported key type %T; use an EC P-256 or P-384, Ed25519, or RSA key of at least %d bits", raw, minRSABits)
	}
	if err != nil {
		return nil, err
	}

	pub, err := jwk.Import(public)
	if err != nil {
		return nil, fmt.Errorf("cannot convert public key: %w", err)
	}
	thumbprint, err := pub.Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("cannot compute key ID: %w", err)
	}
	k := &Key{alg: alg, kid: base64.RawURLEncoding.EncodeToString(thumbprint), public: pub}

	if err := setFields(pub, map[string]any{
		jwk.KeyIDKey:     k.kid,
		jwk.KeyUsageKey:  jwk.ForSignature,
		jwk.AlgorithmKey: alg,
	}); err != nil {
		return nil, err
	}

	if private != nil {
		signer, err := jwk.Import(private)
		if err != nil {
			return nil, fmt.Errorf("cannot convert private key: %w", err)
		}
		if err := setFields(signer, map[string]any{
			jwk.KeyIDKey:     k.kid,
			jwk.AlgorithmKey: alg,
		}); err != nil {
			return nil, err
		}
		k.signer = signer
	}
	return k, nil
}

func setFields(key jwk.Key, fields map[string]any) error {
	for name, value := range fields {
		if err := key.Set(name, value); err != nil {
			return fmt.Errorf("cannot set %q on key: %w", name, err)
		}
	}
	return nil
}

func ecAlgorithm(curve elliptic.Curve) (jwa.SignatureAlgorithm, error) {
	switch curve {
	case elliptic.P256():
		return jwa.ES256(), nil
	case elliptic.P384():
		return jwa.ES384(), nil
	default:
		return jwa.SignatureAlgorithm{}, fmt.Errorf("unsupported EC curve %s; use P-256 or P-384", curve.Params().Name)
	}
}

func rsaAlgorithm(bits int) (jwa.SignatureAlgorithm, error) {
	if bits < minRSABits {
		return jwa.SignatureAlgorithm{}, fmt.Errorf("RSA key has %d bits; at least %d are required", bits, minRSABits)
	}
	return jwa.RS256(), nil
}
