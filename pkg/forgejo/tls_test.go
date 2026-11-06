// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// testCA is a certificate authority generated for one test. Nothing here is
// written outside t.TempDir(), so no key material enters the repository.
type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

var serialCounter atomic.Int64

func newTestCA(t *testing.T, name string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serialCounter.Add(1)),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCA{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

// issue returns a leaf certificate and key signed by ca, valid until notAfter.
func (ca *testCA) issue(t *testing.T, usage x509.ExtKeyUsage, notAfter time.Time) tls.Certificate {
	t.Helper()
	certPEM, keyPEM := ca.issuePEM(t, usage, notAfter)
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func (ca *testCA) issuePEM(t *testing.T, usage x509.ExtKeyUsage, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serialCounter.Add(1)),
		Subject:      pkix.Name{CommonName: "forgejo-mcp test"},
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// clientFiles writes a client certificate signed by ca and returns the
// settings naming it.
func clientFiles(t *testing.T, ca *testCA, notAfter time.Time) TLSSettings {
	t.Helper()
	certPEM, keyPEM := ca.issuePEM(t, x509.ExtKeyUsageClientAuth, notAfter)
	return TLSSettings{
		CertFile: writeFile(t, "client.crt", certPEM),
		KeyFile:  writeFile(t, "client.key", keyPEM),
	}
}

// mtlsForge starts a TLS server with a certificate from serverCA. With
// clientCA set it requires a client certificate signed by it. It answers
// /api/v1/version and /api/v1/user like Forgejo, and 401 for the token
// "rejected", echoing it. reached counts requests that got past the
// handshake. flag.URL points at it for the test.
func mtlsForge(t *testing.T, serverCA, clientCA *testCA) (reached *atomic.Int64) {
	t.Helper()
	reached = &atomic.Int64{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"16.0.3"}`))
		case r.Header.Get("Authorization") == "token rejected":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"access token does not exist [sha: rejected]"}`))
		case r.URL.Path == "/api/v1/user":
			_, _ = w.Write([]byte(`{"id":1,"login":"alice"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	cfg := &tls.Config{
		Certificates: []tls.Certificate{serverCA.issue(t, x509.ExtKeyUsageServerAuth, time.Now().Add(time.Hour))},
		MinVersion:   tls.VersionTLS12,
	}
	if clientCA != nil {
		pool := x509.NewCertPool()
		pool.AddCert(clientCA.cert)
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	srv.TLS = cfg
	srv.StartTLS()
	t.Cleanup(srv.Close)

	savedURL, savedToken := flag.URL, flag.Token
	flag.URL, flag.Token = srv.URL, ""
	client = nil
	t.Cleanup(func() {
		flag.URL, flag.Token = savedURL, savedToken
		client = nil
		SetServerVersion("")
	})
	return reached
}

// restoreTransports undoes ConfigureTLS at the end of the test.
func restoreTransports(t *testing.T) {
	t.Helper()
	sdk, raw := sdkHTTPClient.Transport, rawHTTPClient.Transport
	t.Cleanup(func() {
		sdkHTTPClient.Transport, rawHTTPClient.Transport = sdk, raw
		sdkHTTPClient.CloseIdleConnections()
		rawHTTPClient.CloseIdleConnections()
	})
}

func configure(t *testing.T, s TLSSettings) *TLSSummary {
	t.Helper()
	restoreTransports(t)
	summary, err := ConfigureTLS(s, flag.URL)
	if err != nil {
		t.Fatalf("ConfigureTLS: %v", err)
	}
	return summary
}

func tokenCtx(token string) context.Context {
	return WithToken(context.Background(), token)
}

// Spec forgejo-client-tls, scenario "SDK call to an mTLS-only instance".
func TestTLSSDKCallToAnMTLSOnlyInstance(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	mtlsForge(t, ca, ca)
	s := clientFiles(t, ca, time.Now().Add(time.Hour))
	s.CAFile = writeFile(t, "ca.pem", ca.pem)
	configure(t, s)

	c, err := Client(tokenCtx("caller-token"))
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	u, _, err := c.GetMyUserInfo()
	if err != nil {
		t.Fatalf("GetMyUserInfo: %v", err)
	}
	if u.UserName != "alice" {
		t.Fatalf("login = %q, want alice", u.UserName)
	}
}

// Spec forgejo-client-tls, scenario "Raw-HTTP call to an mTLS-only instance".
func TestTLSRawHTTPCallToAnMTLSOnlyInstance(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	mtlsForge(t, ca, ca)
	s := clientFiles(t, ca, time.Now().Add(time.Hour))
	s.CAFile = writeFile(t, "ca.pem", ca.pem)
	configure(t, s)

	var out struct {
		Login string `json:"login"`
	}
	if err := DoJSON(tokenCtx("caller-token"), http.MethodGet, "/user", nil, &out); err != nil {
		t.Fatalf("DoJSON: %v", err)
	}
	if out.Login != "alice" {
		t.Fatalf("login = %q, want alice", out.Login)
	}
}

// Spec forgejo-client-tls, scenario "Version probe to an mTLS-only instance".
func TestTLSVersionProbeToAnMTLSOnlyInstance(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	mtlsForge(t, ca, ca)
	s := clientFiles(t, ca, time.Now().Add(time.Hour))
	s.CAFile = writeFile(t, "ca.pem", ca.pem)
	configure(t, s)

	v, err := ProbeServerVersion(context.Background())
	if err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
	if v != "16.0.3" {
		t.Fatalf("version = %q", v)
	}
}

// Spec forgejo-client-tls, scenario "No client certificate configured against
// an mTLS-only instance".
func TestTLSNoClientCertificateAgainstAnMTLSOnlyInstance(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	reached := mtlsForge(t, ca, ca)
	configure(t, TLSSettings{CAFile: writeFile(t, "ca.pem", ca.pem)})

	if _, err := ProbeServerVersion(context.Background()); err == nil {
		t.Fatal("expected the handshake to fail")
	}
	if err := DoJSON(tokenCtx("caller-token"), http.MethodGet, "/user", nil, &struct{}{}); err == nil {
		t.Fatal("expected the handshake to fail")
	}
	if n := reached.Load(); n != 0 {
		t.Fatalf("%d requests reached the handler", n)
	}
}

// Spec forgejo-client-tls, scenario "Private CA trusted" (no client
// certificate: -tls-ca on its own).
func TestTLSPrivateCATrusted(t *testing.T) {
	ca := newTestCA(t, "private CA")
	mtlsForge(t, ca, nil)
	summary := configure(t, TLSSettings{CAFile: writeFile(t, "ca.pem", ca.pem)})
	if !summary.CA || summary.ClientCert {
		t.Fatalf("summary = %+v", summary)
	}
	if _, err := ProbeServerVersion(context.Background()); err != nil {
		t.Fatalf("ProbeServerVersion: %v", err)
	}
}

// Spec forgejo-client-tls, scenario "Private CA without the setting".
func TestTLSPrivateCAWithoutTheSetting(t *testing.T) {
	ca := newTestCA(t, "private CA")
	mtlsForge(t, ca, nil)

	_, err := ProbeServerVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("expected a certificate verification error, got %v", err)
	}
}

// Spec forgejo-client-tls, scenario "Server certificate from another CA".
func TestTLSServerCertificateFromAnotherCA(t *testing.T) {
	serverCA, otherCA := newTestCA(t, "CA B"), newTestCA(t, "CA A")
	mtlsForge(t, serverCA, nil)
	configure(t, TLSSettings{CAFile: writeFile(t, "ca.pem", otherCA.pem)})

	_, err := ProbeServerVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("expected a certificate verification error, got %v", err)
	}
}

// The rejection handling from stateless-http-auth sits on top of the TLS
// transport: a 401 over mTLS still has its echoed credential replaced.
func TestTLSRejectionStillReplacesTheBody(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	mtlsForge(t, ca, ca)
	s := clientFiles(t, ca, time.Now().Add(time.Hour))
	s.CAFile = writeFile(t, "ca.pem", ca.pem)
	configure(t, s)
	SetServerVersion("16.0.3")

	rejection := &Rejection{}
	ctx := WithRejection(tokenCtx("rejected"), rejection)
	c, err := Client(ctx)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	_, _, err = c.GetMyUserInfo()
	if err == nil || err.Error() != RejectedCredentialMessage {
		t.Fatalf("error = %v, want %q", err, RejectedCredentialMessage)
	}
	if !rejection.Rejected() {
		t.Fatal("the rejection was not recorded")
	}
}

// Spec forgejo-client-tls, scenario "Nothing configured".
func TestTLSNothingConfiguredLeavesTheTransportsAlone(t *testing.T) {
	restoreTransports(t)
	sdk, raw := sdkHTTPClient.Transport, rawHTTPClient.Transport
	summary, err := ConfigureTLS(TLSSettings{}, "http://forge.example")
	if err != nil || summary != nil {
		t.Fatalf("ConfigureTLS = %v, %v; want nil, nil", summary, err)
	}
	if sdkHTTPClient.Transport != sdk || rawHTTPClient.Transport != raw {
		t.Fatal("the transports were replaced")
	}
}

// Spec forgejo-client-tls, requirement "TLS settings are validated at startup
// and fail closed", plus "An expired client certificate is refused at
// startup".
func TestTLSStartupValidation(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	valid := clientFiles(t, ca, time.Now().Add(time.Hour))
	expired := clientFiles(t, ca, time.Now().Add(-time.Hour))
	other := clientFiles(t, ca, time.Now().Add(time.Hour))
	caFile := writeFile(t, "ca.pem", ca.pem)
	notPEM := writeFile(t, "junk.pem", []byte("not a certificate\n"))
	missing := filepath.Join(t.TempDir(), "missing.pem")

	cases := []struct {
		name     string
		settings TLSSettings
		url      string
		want     []string
	}{
		{"certificate without key", TLSSettings{CertFile: valid.CertFile}, "", []string{tlsCertSetting, tlsKeySetting, "together"}},
		{"key without certificate", TLSSettings{KeyFile: valid.KeyFile}, "", []string{tlsCertSetting, tlsKeySetting, "together"}},
		{"missing certificate file", TLSSettings{CertFile: missing, KeyFile: valid.KeyFile}, "", []string{tlsCertSetting}},
		{"missing key file", TLSSettings{CertFile: valid.CertFile, KeyFile: missing}, "", []string{tlsKeySetting}},
		{"key of another certificate", TLSSettings{CertFile: valid.CertFile, KeyFile: other.KeyFile}, "", []string{"do not form a valid"}},
		{"key file not PEM", TLSSettings{CertFile: valid.CertFile, KeyFile: notPEM}, "", []string{"do not form a valid"}},
		{"CA file without certificates", TLSSettings{CAFile: notPEM}, "", []string{tlsCASetting, "no PEM certificate"}},
		{"missing CA file", TLSSettings{CAFile: missing}, "", []string{tlsCASetting}},
		{"expired client certificate", expired, "", []string{tlsCertSetting, "expired at"}},
		{"http URL", TLSSettings{CAFile: caFile}, "http://forge.example", []string{"https"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreTransports(t)
			sdk, raw := sdkHTTPClient.Transport, rawHTTPClient.Transport
			u := tc.url
			if u == "" {
				u = "https://forge.example"
			}
			_, err := ConfigureTLS(tc.settings, u)
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not mention %q", err, w)
				}
			}
			if sdkHTTPClient.Transport != sdk || rawHTTPClient.Transport != raw {
				t.Fatal("a failed configuration replaced the transports")
			}
		})
	}
}

// Spec forgejo-client-tls, scenario "Valid certificate": the summary carries
// the expiry and nothing else about the certificate.
func TestTLSSummaryOfAValidCertificate(t *testing.T) {
	ca := newTestCA(t, "forge CA")
	notAfter := time.Now().Add(time.Hour).Truncate(time.Second)
	s := clientFiles(t, ca, notAfter)
	restoreTransports(t)
	summary, err := ConfigureTLS(s, "https://forge.example")
	if err != nil {
		t.Fatalf("ConfigureTLS: %v", err)
	}
	if !summary.ClientCert || summary.CA || !summary.NotAfter.Equal(notAfter) {
		t.Fatalf("summary = %+v, want client cert expiring %s", summary, notAfter)
	}
}
