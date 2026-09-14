// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs"
)

const (
	// AuthModePassthrough forwards the caller's Authorization header to Forgejo.
	AuthModePassthrough = "passthrough"
	// AuthModeResourceServer validates identity-provider tokens and signs
	// Forgejo Authorized Integration tokens.
	AuthModeResourceServer = "resource-server"

	// minForgejoMajorForResourceServer is the first Forgejo release with
	// Authorized Integrations.
	minForgejoMajorForResourceServer = 16
)

// resourceServerMode reports whether the configured auth mode is
// resource-server.
func resourceServerMode() bool { return flag.AuthMode == AuthModeResourceServer }

// ValidateAuthConfig checks the auth settings that can be judged without any
// I/O. It runs before anything is bound or contacted, so a misconfigured start
// never opens a socket.
//
// cli reports whether the process runs in --cli mode, which has no transport.
func ValidateAuthConfig(transport string, cli bool) error {
	mode := flag.AuthMode
	if mode == "" {
		mode = AuthModePassthrough
	}
	switch mode {
	case AuthModePassthrough:
		if len(flag.ResourceServerSettings) > 0 {
			return fmt.Errorf("refusing to start: %s requires -auth-mode resource-server; remove it, or enable that mode",
				strings.Join(flag.ResourceServerSettings, ", "))
		}
		return nil
	case AuthModeResourceServer:
	default:
		return fmt.Errorf("refusing to start: -auth-mode %q is not valid; use passthrough or resource-server", mode)
	}

	if cli {
		return errors.New("refusing to start: -auth-mode resource-server requires the http transport and cannot be used with --cli")
	}
	if transport != "http" {
		return fmt.Errorf("refusing to start: -auth-mode resource-server requires the http transport, not %q", transport)
	}
	if flag.Token != "" {
		return errors.New("refusing to start: -auth-mode resource-server takes no operator token; " +
			"unset -token and FORGEJO_ACCESS_TOKEN (and GITEA_ACCESS_TOKEN)")
	}
	if flag.AllowOperatorTokenFallback {
		return errors.New("refusing to start: -allow-operator-token-fallback cannot be combined with -auth-mode resource-server")
	}

	var missing []string
	for _, s := range []struct{ name, value string }{
		{"-authorization-server", flag.AuthorizationServer},
		{"-resource", flag.Resource},
		{"-forgejo-jwt-issuer", flag.ForgejoJWTIssuer},
		{"-forgejo-jwt-signing-key-file", flag.ForgejoJWTSigningKeyFile},
	} {
		if strings.TrimSpace(s.value) == "" {
			missing = append(missing, s.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("refusing to start: -auth-mode resource-server requires %s", strings.Join(missing, ", "))
	}
	for _, scope := range flag.ScopesSupported {
		if !validScopeToken(scope) {
			return fmt.Errorf("refusing to start: -scopes-supported contains %q, which is not a valid OAuth scope", scope)
		}
	}

	if _, err := httpsOrLoopbackURL("-authorization-server", flag.AuthorizationServer); err != nil {
		return err
	}
	resourceURL, err := httpsOrLoopbackURL("-resource", flag.Resource)
	if err != nil {
		return err
	}
	// The metadata publishes -resource as the resource and derives its own
	// location from its path, while the endpoint is always served at
	// streamableHTTPEndpointPath. Any other path starts cleanly and then names
	// a resource this server does not serve, which clients must reject.
	if resourceURL.Path != streamableHTTPEndpointPath {
		return fmt.Errorf("refusing to start: the path of -resource (%q) must be %q, the MCP endpoint this server serves",
			resourceURL.Path, streamableHTTPEndpointPath)
	}
	issuerURL, err := url.Parse(flag.ForgejoJWTIssuer)
	if err != nil || issuerURL.Scheme != "https" || issuerURL.Host == "" {
		return fmt.Errorf("refusing to start: -forgejo-jwt-issuer %q must be an https URL, because Forgejo only fetches https issuers",
			flag.ForgejoJWTIssuer)
	}
	if err := jwtissuer.CheckIssuerURL(flag.ForgejoJWTIssuer); err != nil {
		return fmt.Errorf("refusing to start: -forgejo-jwt-issuer: %w", err)
	}

	policy := hostPolicy{
		allowed:      normalizeList(flag.AllowedHosts),
		loopbackOnly: isLoopbackHostname(strings.TrimSpace(flag.Host)),
	}
	for _, s := range []struct {
		name string
		u    *url.URL
	}{{"-resource", resourceURL}, {"-forgejo-jwt-issuer", issuerURL}} {
		if !policy.permits(s.u.Host) {
			return fmt.Errorf("refusing to start: the host of %s (%s) is not one this server answers to; add it to -allowed-hosts",
				s.name, s.u.Host)
		}
	}
	return nil
}

// httpsOrLoopbackURL parses an absolute URL that must use https, or http on a
// loopback host, which only local development needs.
func httpsOrLoopbackURL(name, raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("refusing to start: %s %q is not an absolute URL", name, raw)
	}
	switch {
	case u.Scheme == "https":
		return u, nil
	case u.Scheme == "http" && isLoopbackHostname(u.Hostname()):
		return u, nil
	default:
		return nil, fmt.Errorf("refusing to start: %s %q must use https (http is accepted only for a loopback host)", name, raw)
	}
}

// resourceServer holds everything resource-server mode needs at request time.
type resourceServer struct {
	issuer        *jwtissuer.Issuer
	validator     *oauthrs.Validator
	resource      string
	authServer    string
	scopes        []string
	audienceClaim string
}

// prepareResourceServer runs the startup checks that need I/O, in order: load
// the keys, read Forgejo's version, and read the identity provider's metadata
// and keys. It fails on the first problem, before anything is bound.
func prepareResourceServer(ctx context.Context) (*resourceServer, error) {
	signing, err := jwtissuer.LoadSigningKey(flag.ForgejoJWTSigningKeyFile)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -forgejo-jwt-signing-key-file: %w", err)
	}
	published := make([]*jwtissuer.Key, 0, len(flag.ForgejoJWTPublishedKeyFiles))
	for _, path := range flag.ForgejoJWTPublishedKeyFiles {
		k, err := jwtissuer.LoadPublishedKey(path)
		if err != nil {
			return nil, fmt.Errorf("refusing to start: -forgejo-jwt-published-key-files: %w", err)
		}
		published = append(published, k)
	}
	issuer, err := jwtissuer.New(flag.ForgejoJWTIssuer, signing, published)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -forgejo-jwt-issuer: %w", err)
	}

	version, err := forgejo.ProbeServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: cannot read the Forgejo version: %w", err)
	}
	major, err := forgejo.MajorVersion(version)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: %w", err)
	}
	if major < minForgejoMajorForResourceServer {
		return nil, fmt.Errorf("refusing to start: -auth-mode resource-server requires Forgejo %d.0 or newer, but %s reports %s",
			minForgejoMajorForResourceServer, log.SanitizeURL(flag.URL), version)
	}
	forgejo.SetServerVersion(version)

	metadata, err := oauthrs.DiscoverProvider(ctx, nil, flag.AuthorizationServer)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -authorization-server: %w", err)
	}
	keys, err := oauthrs.NewKeySet(ctx, metadata.JWKSURI, oauthrs.KeySetOptions{})
	if err != nil {
		return nil, fmt.Errorf("refusing to start: -authorization-server: %w", err)
	}
	audience := flag.ResourceAudience
	if audience == "" {
		audience = flag.Resource
	}
	validator, err := oauthrs.NewValidator(flag.AuthorizationServer, audience, keys, nil)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: %w", err)
	}

	log.Warn("The Forgejo signing key is a credential for every Forgejo user whose Authorized Integration trusts this issuer",
		log.StringField("kid", issuer.SigningKeyID()),
		log.StringField("issuer", issuer.URL()),
		log.StringField("forgejo_version", version),
	)

	return &resourceServer{
		issuer:        issuer,
		validator:     validator,
		resource:      flag.Resource,
		authServer:    flag.AuthorizationServer,
		scopes:        flag.ScopesSupported,
		audienceClaim: flag.ForgejoAudienceClaim,
	}, nil
}
