// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	stdflag "flag"
	"reflect"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation"
	flagPkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// withAuthFlags parses args with the auth flags registered, exactly as
// initFlags does, clears every auth environment variable, and restores the
// package state afterwards.
func withAuthFlags(t *testing.T, args []string) {
	t.Helper()
	vars := []*string{
		&authMode, &authorizationServer, &resource, &resourceAudience, &scopesSupported,
		&forgejoAudienceClaim, &forgejoJWTIssuer, &forgejoJWTSigningKeyFile, &forgejoJWTPublishedKeyFiles,
	}
	savedVars := make([]string, len(vars))
	for i, v := range vars {
		savedVars[i] = *v
	}
	savedSet := flagSet
	savedFlags := struct {
		mode, authServer, res, resAud, claim, issuer, keyFile string
		scopes, published, settings                           []string
	}{
		flagPkg.AuthMode, flagPkg.AuthorizationServer, flagPkg.Resource, flagPkg.ResourceAudience,
		flagPkg.ForgejoAudienceClaim, flagPkg.ForgejoJWTIssuer, flagPkg.ForgejoJWTSigningKeyFile,
		flagPkg.ScopesSupported, flagPkg.ForgejoJWTPublishedKeyFiles, flagPkg.ResourceServerSettings,
	}
	t.Cleanup(func() {
		for i, v := range vars {
			*v = savedVars[i]
		}
		flagSet = savedSet
		flagPkg.AuthMode, flagPkg.AuthorizationServer, flagPkg.Resource = savedFlags.mode, savedFlags.authServer, savedFlags.res
		flagPkg.ResourceAudience, flagPkg.ForgejoAudienceClaim = savedFlags.resAud, savedFlags.claim
		flagPkg.ForgejoJWTIssuer, flagPkg.ForgejoJWTSigningKeyFile = savedFlags.issuer, savedFlags.keyFile
		flagPkg.ScopesSupported, flagPkg.ForgejoJWTPublishedKeyFiles = savedFlags.scopes, savedFlags.published
		flagPkg.ResourceServerSettings = savedFlags.settings
	})

	t.Setenv("FORGEJO_MCP_AUTH_MODE", "")
	for _, s := range resourceServerSettings() {
		t.Setenv(s.envVar, "")
	}

	fs := stdflag.NewFlagSet("forgejo-mcp", stdflag.ContinueOnError)
	registerAuthFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	flagSet = fs
}

// Spec oauth-resource-server, scenario "No mode configured".
func TestAuthModeDefaultsToPassthroughWithNothingGiven(t *testing.T) {
	withAuthFlags(t, nil)
	resolveAuthSettings()

	if flagPkg.AuthMode != "passthrough" {
		t.Fatalf("auth mode = %q, want passthrough", flagPkg.AuthMode)
	}
	if len(flagPkg.ResourceServerSettings) != 0 {
		t.Fatalf("settings recorded as given: %v", flagPkg.ResourceServerSettings)
	}
	if flagPkg.ForgejoAudienceClaim != "forgejo_aud" {
		t.Fatalf("audience claim default = %q, want forgejo_aud", flagPkg.ForgejoAudienceClaim)
	}
}

func TestResourceServerSettingsResolveFromFlagsAndEnvironment(t *testing.T) {
	withAuthFlags(t, []string{
		"-auth-mode", "resource-server",
		"-authorization-server", "https://sso.example.org",
		"-scopes-supported", "openid  profile email",
		"-forgejo-jwt-published-key-files", "next.pem, old.pem",
	})
	t.Setenv("FORGEJO_MCP_RESOURCE", "https://mcp.example.org/mcp")
	resolveAuthSettings()

	if flagPkg.AuthMode != "resource-server" {
		t.Fatalf("auth mode = %q", flagPkg.AuthMode)
	}
	if flagPkg.AuthorizationServer != "https://sso.example.org" || flagPkg.Resource != "https://mcp.example.org/mcp" {
		t.Fatalf("authorization server %q, resource %q", flagPkg.AuthorizationServer, flagPkg.Resource)
	}
	if want := []string{"openid", "profile", "email"}; !reflect.DeepEqual(flagPkg.ScopesSupported, want) {
		t.Fatalf("scopes = %q, want %q", flagPkg.ScopesSupported, want)
	}
	if want := []string{"next.pem", "old.pem"}; !reflect.DeepEqual(flagPkg.ForgejoJWTPublishedKeyFiles, want) {
		t.Fatalf("published key files = %q, want %q", flagPkg.ForgejoJWTPublishedKeyFiles, want)
	}
	want := []string{"-authorization-server", "FORGEJO_MCP_RESOURCE", "-scopes-supported", "-forgejo-jwt-published-key-files"}
	if !reflect.DeepEqual(flagPkg.ResourceServerSettings, want) {
		t.Fatalf("settings recorded as given = %q, want %q", flagPkg.ResourceServerSettings, want)
	}
}

// A flag explicitly set to its default still wins over the environment, and
// still counts as given, so passthrough mode refuses it.
func TestExplicitAuthFlagEqualToTheDefaultBeatsTheEnvironment(t *testing.T) {
	withAuthFlags(t, []string{"-forgejo-audience-claim", "forgejo_aud"})
	t.Setenv("FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM", "other_claim")
	resolveAuthSettings()

	if flagPkg.ForgejoAudienceClaim != "forgejo_aud" {
		t.Fatalf("audience claim = %q, want the explicit flag value", flagPkg.ForgejoAudienceClaim)
	}
	if want := []string{"-forgejo-audience-claim"}; !reflect.DeepEqual(flagPkg.ResourceServerSettings, want) {
		t.Fatalf("settings recorded as given = %q, want %q", flagPkg.ResourceServerSettings, want)
	}
}

// Spec oauth-resource-server, scenario "Issuer configured without the mode",
// for every resource-server-only setting, given as an environment variable and
// as a flag: passthrough mode records it and refuses to start, naming it.
func TestPassthroughRefusesEveryResourceServerSetting(t *testing.T) {
	for _, s := range resourceServerSettings() {
		t.Run(s.envVar, func(t *testing.T) {
			withAuthFlags(t, nil)
			t.Setenv(s.envVar, "value")
			resolveAuthSettings()
			err := operation.ValidateAuthConfig("stdio", false)
			if err == nil || !strings.Contains(err.Error(), s.envVar) || !strings.Contains(err.Error(), "requires -auth-mode resource-server") {
				t.Fatalf("passthrough with %s: error = %v, want a refusal naming it", s.envVar, err)
			}
		})
		t.Run("-"+s.flagName, func(t *testing.T) {
			withAuthFlags(t, []string{"-" + s.flagName, "value"})
			resolveAuthSettings()
			err := operation.ValidateAuthConfig("stdio", false)
			if err == nil || !strings.Contains(err.Error(), "-"+s.flagName) {
				t.Fatalf("passthrough with -%s: error = %v, want a refusal naming it", s.flagName, err)
			}
		})
	}
}

// --cli mode never parses flags, so the environment alone must configure it.
func TestAuthSettingsResolveFromTheEnvironmentWithoutAFlagSet(t *testing.T) {
	withAuthFlags(t, nil)
	flagSet = nil
	t.Setenv("FORGEJO_MCP_AUTH_MODE", "resource-server")
	t.Setenv("FORGEJO_MCP_FORGEJO_JWT_ISSUER", "https://mcp.example.org/issuer")
	resolveAuthSettings()

	if flagPkg.AuthMode != "resource-server" || flagPkg.ForgejoJWTIssuer != "https://mcp.example.org/issuer" {
		t.Fatalf("auth mode %q, issuer %q", flagPkg.AuthMode, flagPkg.ForgejoJWTIssuer)
	}
	if want := []string{"FORGEJO_MCP_FORGEJO_JWT_ISSUER"}; !reflect.DeepEqual(flagPkg.ResourceServerSettings, want) {
		t.Fatalf("settings recorded as given = %q, want %q", flagPkg.ResourceServerSettings, want)
	}
}
