// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs"

	"go.uber.org/zap"
)

const (
	// protectedResourceMetadataPath is the RFC 9728 well-known location.
	protectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

	// issuerDocumentCacheControl lets proxies and Forgejo cache the issuer
	// documents briefly. Forgejo applies its own cache on top.
	issuerDocumentCacheControl = "public, max-age=300"

	// maxAudienceClaimLength bounds the Forgejo audience taken from a token.
	maxAudienceClaimLength = 256
)

// mintedTokenKey carries the Forgejo token minted for a request, from the
// authentication layer to requestTokenContextFunc. It is unexported so that
// nothing outside this package can place a credential in a request context.
type mintedTokenKey struct{}

type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
}

// handler builds the whole HTTP surface of resource-server mode around the MCP
// transport handler.
//
// The public routes (protected resource metadata, the issuer's discovery
// document and key set) and the one protected route (the MCP endpoint) are
// matched exactly on the request path. http.ServeMux is deliberately not used:
// it answers an unclean path such as "/issuer/../mcp" with a redirect, and
// Forgejo refuses to follow redirects when it fetches the issuer documents.
// Every other path gets 404. Authentication wraps the protected route only, so
// the documents a caller needs before it has a token are reachable without one.
func (rs *resourceServer) handler(mcp http.Handler) (http.Handler, error) {
	resourceURL, err := url.Parse(rs.resource)
	if err != nil {
		return nil, fmt.Errorf("resource %q: %w", rs.resource, err)
	}
	issuerURL, err := url.Parse(rs.issuer.URL())
	if err != nil {
		return nil, fmt.Errorf("issuer %q: %w", rs.issuer.URL(), err)
	}

	metadataPath := protectedResourceMetadataPath + strings.TrimSuffix(resourceURL.Path, "/")
	metadataURL := (&url.URL{Scheme: resourceURL.Scheme, Host: resourceURL.Host, Path: metadataPath}).String()
	metadata, err := json.Marshal(protectedResourceMetadata{
		Resource:               rs.resource,
		AuthorizationServers:   []string{rs.authServer},
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        rs.scopes,
	})
	if err != nil {
		return nil, fmt.Errorf("encode protected resource metadata: %w", err)
	}

	routes := map[string]http.Handler{
		metadataPath:                             staticDocument(metadata, ""),
		protectedResourceMetadataPath:            staticDocument(metadata, ""),
		issuerURL.Path + jwtissuer.DiscoveryPath: staticDocument(rs.issuer.DiscoveryDocument(), issuerDocumentCacheControl),
		issuerURL.Path + jwtissuer.JWKSPath:      staticDocument(rs.issuer.JWKS(), issuerDocumentCacheControl),
		streamableHTTPEndpointPath:               rs.authenticate(mcp, bearerChallenge(metadataURL, rs.scopes)),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		route.ServeHTTP(w, r)
	}), nil
}

// staticDocument serves a fixed JSON document to GET and HEAD.
func staticDocument(body []byte, cacheControl string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(body)
	})
}

// bearerChallenge is the WWW-Authenticate value of every 401, without the error
// parameter. Its inputs are validated at startup, so no quoting is needed.
func bearerChallenge(metadataURL string, scopes []string) string {
	challenge := `Bearer resource_metadata="` + metadataURL + `"`
	if len(scopes) > 0 {
		challenge += `, scope="` + strings.Join(scopes, " ") + `"`
	}
	return challenge
}

// authenticate validates the caller's access token, derives the Forgejo
// audience, mints the Forgejo token and hands it to next through the request
// context. The caller's own token goes no further than this function.
func (rs *resourceServer) authenticate(next http.Handler, challenge string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := rs.validator.ValidateAuthorization(r.Context(), r.Header.Get("Authorization"))
		if err == nil && claims.Subject == "" {
			err = fmt.Errorf("%w: the token has no sub", oauthrs.ErrInvalidToken)
		}
		if err != nil {
			value := challenge
			if errors.Is(err, oauthrs.ErrInvalidToken) {
				value += `, error="invalid_token"`
			}
			logRefusalAtDebug("Refused a request to the MCP endpoint",
				log.StringField("reason", err.Error()),
				log.StringField("path", truncateForLog(r.URL.Path)),
			)
			w.Header().Set("WWW-Authenticate", value)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		audience, ok := usableAudience(claims.Raw[rs.audienceClaim])
		if !ok {
			logRefusalAtDebug("Refused a token without a usable Forgejo audience claim",
				log.StringField("claim", rs.audienceClaim),
				log.StringField("sub", truncateForLog(claims.Subject)),
			)
			// No WWW-Authenticate challenge, deliberately (design D4). The token
			// is valid; what is missing is data at the identity provider, not
			// authorization or scope. A challenge, with or without
			// error="insufficient_scope", invites a spec-following client to
			// re-authorize or step up its scopes, and that loop cannot succeed.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintf(w,
				"Forbidden: the access token carries no usable %q claim. It must hold the audience of your Forgejo "+
					"Authorized Integration for this server.\n", rs.audienceClaim)
			return
		}

		token, err := rs.issuer.Mint(claims.Subject, audience, time.Now())
		if err != nil {
			log.Error("Cannot mint a Forgejo token", log.ErrorField(err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		log.Debug("Authenticated a request to the MCP endpoint",
			log.StringField("sub", truncateForLog(claims.Subject)),
			log.StringField("forgejo_audience", truncateForLog(audience)),
		)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), mintedTokenKey{}, token)))
	})
}

// usableAudience accepts a single non-empty string of bounded length with no
// whitespace or control characters. Forgejo requires exactly one audience.
func usableAudience(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || s == "" || len(s) > maxAudienceClaimLength {
		return "", false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", false
		}
	}
	return s, true
}

// validScopeToken reports whether s is an OAuth 2.0 scope token (RFC 6749
// section 3.3): one or more of %x21 / %x23-5B / %x5D-7E. Scopes are placed
// into a quoted header value, so a quote or backslash must never reach it.
func validScopeToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x21 || c > 0x7E || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}

// logRefusalAtDebug logs a refusal at debug level through the same rate bound
// as the door's refusal lines: the values come from strangers.
func logRefusalAtDebug(msg string, fields ...zap.Field) {
	ok, dropped := refusals.admit(time.Now())
	if dropped > 0 {
		log.Warn("Suppressed refusal log lines over the preceding window",
			log.IntField("suppressed", dropped),
		)
	}
	if ok {
		log.Debug(msg, fields...)
	}
}
