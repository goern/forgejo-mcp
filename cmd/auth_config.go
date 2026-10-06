// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	stdflag "flag"
	"os"
	"strings"

	flagPkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// defaultForgejoAudienceClaim is the inbound claim that carries a caller's
// Forgejo integration audience unless configured otherwise.
const defaultForgejoAudienceClaim = "forgejo_aud"

var (
	authMode                    string
	authorizationServer         string
	resource                    string
	resourceAudience            string
	scopesSupported             string
	forgejoAudienceClaim        string
	forgejoJWTIssuer            string
	forgejoJWTSigningKeyFile    string
	forgejoJWTPublishedKeyFiles string
)

// authSetting is one setting that applies only to resource-server mode.
type authSetting struct {
	target   *string
	flagName string
	envVar   string
}

func resourceServerSettings() []authSetting {
	return []authSetting{
		{&authorizationServer, "authorization-server", "FORGEJO_MCP_AUTHORIZATION_SERVER"},
		{&resource, "resource", "FORGEJO_MCP_RESOURCE"},
		{&resourceAudience, "resource-audience", "FORGEJO_MCP_RESOURCE_AUDIENCE"},
		{&scopesSupported, "scopes-supported", "FORGEJO_MCP_SCOPES_SUPPORTED"},
		{&forgejoAudienceClaim, "forgejo-audience-claim", "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM"},
		{&forgejoJWTIssuer, "forgejo-jwt-issuer", "FORGEJO_MCP_FORGEJO_JWT_ISSUER"},
		{&forgejoJWTSigningKeyFile, "forgejo-jwt-signing-key-file", "FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE"},
		{&forgejoJWTPublishedKeyFiles, "forgejo-jwt-published-key-files", "FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES"},
	}
}

// registerAuthFlags adds the auth-mode flags to fs. Every resource-server
// setting registers with an empty default so that "was it given at all" can
// be told from the flag set; defaults are applied when resolving.
func registerAuthFlags(fs *stdflag.FlagSet) {
	fs.StringVar(&authMode, "auth-mode", "passthrough",
		"How the http transport authenticates callers: passthrough forwards the caller's Authorization header to "+
			"Forgejo; resource-server validates an identity provider's JWT access token and signs a Forgejo "+
			"Authorized Integration token (Forgejo 16 or newer)")
	fs.StringVar(&authorizationServer, "authorization-server", "",
		"resource-server mode: issuer URL of the identity provider callers obtain tokens from")
	fs.StringVar(&resource, "resource", "",
		"resource-server mode: canonical URI of this server's MCP endpoint, e.g. https://mcp.example.org/mcp")
	fs.StringVar(&resourceAudience, "resource-audience", "",
		"resource-server mode: value an inbound token's aud must contain (default: the value of -resource)")
	fs.StringVar(&scopesSupported, "scopes-supported", "",
		"resource-server mode: space-separated scopes advertised to clients")
	fs.StringVar(&forgejoAudienceClaim, "forgejo-audience-claim", "",
		"resource-server mode: inbound claim holding the caller's Forgejo integration audience (default: "+
			defaultForgejoAudienceClaim+")")
	fs.StringVar(&forgejoJWTIssuer, "forgejo-jwt-issuer", "",
		"resource-server mode: https issuer URL this server presents to Forgejo, e.g. https://mcp.example.org/issuer")
	fs.StringVar(&forgejoJWTSigningKeyFile, "forgejo-jwt-signing-key-file", "",
		"resource-server mode: PEM private key (EC P-256/P-384, Ed25519 or RSA >= 2048 bits) that signs Forgejo tokens")
	fs.StringVar(&forgejoJWTPublishedKeyFiles, "forgejo-jwt-published-key-files", "",
		"resource-server mode: comma-separated PEM keys published without signing with them, for key rotation")
}

// resolveAuthSettings copies the auth settings into pkg/flag, flag over
// environment over default, and records which resource-server-only settings
// were given at all, so that passthrough mode can refuse them rather than
// silently ignore a configuration that was meant to enable the mode.
func resolveAuthSettings() {
	flagPkg.AuthMode = resolveString(authMode, "FORGEJO_MCP_AUTH_MODE", "auth-mode")
	if flagPkg.AuthMode == "" {
		flagPkg.AuthMode = "passthrough"
	}

	flagPkg.ResourceServerSettings = nil
	values := make(map[string]string)
	for _, s := range resourceServerSettings() {
		values[s.flagName] = resolveString(*s.target, s.envVar, s.flagName)
		switch {
		case flagWasPassed(s.flagName):
			flagPkg.ResourceServerSettings = append(flagPkg.ResourceServerSettings, "-"+s.flagName)
		case os.Getenv(s.envVar) != "":
			flagPkg.ResourceServerSettings = append(flagPkg.ResourceServerSettings, s.envVar)
		}
	}

	flagPkg.AuthorizationServer = values["authorization-server"]
	flagPkg.Resource = values["resource"]
	flagPkg.ResourceAudience = values["resource-audience"]
	flagPkg.ScopesSupported = strings.Fields(values["scopes-supported"])
	flagPkg.ForgejoAudienceClaim = values["forgejo-audience-claim"]
	if flagPkg.ForgejoAudienceClaim == "" {
		flagPkg.ForgejoAudienceClaim = defaultForgejoAudienceClaim
	}
	flagPkg.ForgejoJWTIssuer = values["forgejo-jwt-issuer"]
	flagPkg.ForgejoJWTSigningKeyFile = values["forgejo-jwt-signing-key-file"]
	flagPkg.ForgejoJWTPublishedKeyFiles = splitList(values["forgejo-jwt-published-key-files"])
}
