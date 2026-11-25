// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Setting names as an operator writes them, for error messages.
const (
	tlsCertSetting = "-tls-cert / FORGEJO_TLS_CERT"
	tlsKeySetting  = "-tls-key / FORGEJO_TLS_KEY"
	tlsCASetting   = "-tls-ca / FORGEJO_TLS_CA"
)

// TLSSettings names the PEM files that configure the TLS layer of every
// connection to Forgejo. The zero value configures nothing.
type TLSSettings struct {
	// CertFile and KeyFile are the client certificate presented to Forgejo.
	// They are given together or not at all.
	CertFile string
	KeyFile  string
	// CAFile is a CA bundle trusted for Forgejo's server certificate instead
	// of the system roots.
	CAFile string
}

// Configured reports whether any TLS setting was given.
func (s TLSSettings) Configured() bool {
	return s.CertFile != "" || s.KeyFile != "" || s.CAFile != ""
}

// TLSSummary describes an applied TLS configuration for the startup log. It
// carries no key material.
type TLSSummary struct {
	ClientCert bool
	// NotAfter is the client certificate's expiry; zero without one.
	NotAfter time.Time
	CA       bool
}

// ConfigureTLS validates s against the Forgejo URL forgeURL and installs the
// result under both shared HTTP clients, so every SDK client, every raw-HTTP
// helper and the version probe use it. With nothing configured it changes
// nothing and returns nil, nil.
//
// It is meant to run once at startup, before any request: it replaces the
// package-level transports and is not safe to call concurrently with traffic.
// An error means nothing was installed; the caller must not continue.
func ConfigureTLS(s TLSSettings, forgeURL string) (*TLSSummary, error) {
	if !s.Configured() {
		return nil, nil
	}
	if u, err := url.Parse(forgeURL); err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("TLS settings need an https Forgejo URL, got %q", forgeURL)
	}
	cfg, summary, err := buildTLSConfig(s, time.Now())
	if err != nil {
		return nil, err
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.TLSClientConfig = cfg
	sdkHTTPClient.Transport = &rejectionTransport{base: base}
	rawHTTPClient.Transport = &rejectionTransport{base: base}
	return summary, nil
}

// buildTLSConfig loads and checks the files s names. now is the time the
// client certificate must still be valid at.
func buildTLSConfig(s TLSSettings, now time.Time) (*tls.Config, *TLSSummary, error) {
	if (s.CertFile == "") != (s.KeyFile == "") {
		return nil, nil, fmt.Errorf("%s and %s must be given together", tlsCertSetting, tlsKeySetting)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	summary := &TLSSummary{}

	if s.CertFile != "" {
		certPEM, err := os.ReadFile(s.CertFile)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", tlsCertSetting, err)
		}
		keyPEM, err := os.ReadFile(s.KeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", tlsKeySetting, err)
		}
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, nil, fmt.Errorf("%s and %s do not form a valid certificate and key: %w", tlsCertSetting, tlsKeySetting, err)
		}
		leaf := pair.Leaf
		if leaf == nil {
			if leaf, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", tlsCertSetting, err)
			}
		}
		if now.After(leaf.NotAfter) {
			return nil, nil, fmt.Errorf("%s: the client certificate expired at %s", tlsCertSetting, leaf.NotAfter.UTC().Format(time.RFC3339))
		}
		if now.Before(leaf.NotBefore) {
			return nil, nil, fmt.Errorf("%s: the client certificate is not valid until %s; check the clock on this host", tlsCertSetting, leaf.NotBefore.UTC().Format(time.RFC3339))
		}
		cfg.Certificates = []tls.Certificate{pair}
		summary.ClientCert = true
		summary.NotAfter = leaf.NotAfter
	}

	if s.CAFile != "" {
		caPEM, err := os.ReadFile(s.CAFile)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", tlsCASetting, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, nil, errors.New(tlsCASetting + ": the file holds no PEM certificate")
		}
		// Replaces the system roots: naming a CA pins Forgejo's issuer.
		cfg.RootCAs = pool
		summary.CA = true
	}
	return cfg, summary, nil
}
