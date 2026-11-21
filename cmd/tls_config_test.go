// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	stdflag "flag"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// withTLSFlags parses args into a flag set holding the TLS flags, as initFlags
// does, and restores the previous state afterwards.
func withTLSFlags(t *testing.T, args []string) {
	t.Helper()
	prevSet, prevCert, prevKey, prevCA := flagSet, tlsCert, tlsKey, tlsCA
	fs := stdflag.NewFlagSet("forgejo-mcp", stdflag.ContinueOnError)
	fs.StringVar(&tlsCert, "tls-cert", "", "")
	fs.StringVar(&tlsKey, "tls-key", "", "")
	fs.StringVar(&tlsCA, "tls-ca", "", "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	flagSet = fs
	t.Cleanup(func() { flagSet, tlsCert, tlsKey, tlsCA = prevSet, prevCert, prevKey, prevCA })
}

func clearTLSEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{"FORGEJO_TLS_CERT", "FORGEJO_TLS_KEY", "FORGEJO_TLS_CA"} {
		t.Setenv(v, "")
	}
}

// Spec forgejo-client-tls, scenario "Flag wins over environment".
func TestTLSFlagWinsOverEnvironment(t *testing.T) {
	clearTLSEnv(t)
	withTLSFlags(t, []string{"-tls-ca", "/b.pem"})
	t.Setenv("FORGEJO_TLS_CA", "/a.pem")

	if got := resolveTLSSettings().CAFile; got != "/b.pem" {
		t.Fatalf("CAFile = %q, want /b.pem", got)
	}
}

// Spec forgejo-client-tls, scenario "Environment alone configures the client
// certificate".
func TestTLSEnvironmentAloneConfiguresTheClientCertificate(t *testing.T) {
	clearTLSEnv(t)
	withTLSFlags(t, nil)
	t.Setenv("FORGEJO_TLS_CERT", "/c.pem")
	t.Setenv("FORGEJO_TLS_KEY", "/k.pem")

	want := forgejo.TLSSettings{CertFile: "/c.pem", KeyFile: "/k.pem"}
	if got := resolveTLSSettings(); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

// Spec forgejo-client-tls, scenario "CLI mode honours the environment": --cli
// runs initConfig without parsing any flag set.
func TestTLSCLIModeHonoursTheEnvironment(t *testing.T) {
	clearTLSEnv(t)
	prev := flagSet
	flagSet = nil
	t.Cleanup(func() { flagSet = prev })
	t.Setenv("FORGEJO_TLS_CERT", "/c.pem")
	t.Setenv("FORGEJO_TLS_KEY", "/k.pem")
	t.Setenv("FORGEJO_TLS_CA", "/ca.pem")

	want := forgejo.TLSSettings{CertFile: "/c.pem", KeyFile: "/k.pem", CAFile: "/ca.pem"}
	if got := resolveTLSSettings(); got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
}

// Spec forgejo-client-tls, scenario "Nothing configured".
func TestTLSNothingConfigured(t *testing.T) {
	clearTLSEnv(t)
	withTLSFlags(t, nil)

	if s := resolveTLSSettings(); s.Configured() {
		t.Fatalf("settings = %+v, want none", s)
	}
}
