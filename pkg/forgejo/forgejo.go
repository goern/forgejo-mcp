package forgejo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var (
	client   *forgejo.Client
	clientMu sync.Mutex
)

type contextKey string

const (
	TokenContextKey contextKey = "forgejo-token"
)

// WithToken adds a Forgejo token to the context.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, TokenContextKey, token)
}

// Client returns a Forgejo client configured to connect to a Forgejo instance.
// If a token is found in the context, a new ephemeral client is returned.
// Otherwise, the shared singleton client is used.
func Client(ctx context.Context) (*forgejo.Client, error) {
	token, ok := ctx.Value(TokenContextKey).(string)
	if ok && token != "" {
		// Use configured user agent or default to forgejo-mcp/<version>
		userAgent := flag.UserAgent
		if userAgent == "" {
			userAgent = "forgejo-mcp/" + flag.Version
		}

		c, err := forgejo.NewClient(flag.URL,
			forgejo.SetToken(token),
			forgejo.SetUserAgent(userAgent),
		)
		if err != nil {
			log.ErrorCtx(ctx, "Failed to create ephemeral Forgejo client",
				log.SanitizedURLField("url", flag.URL),
				log.ErrorField(err),
			)
			return nil, fmt.Errorf("create ephemeral client: %w", err)
		}
		return c, nil
	}

	// No per-request credential. Whether the operator's own configured
	// credential may stand in for it is the policy in credential.go: allowed
	// for stdio and for a loopback-only listener, refused for a listener the
	// network can reach.
	if _, err := tokenForRequest(ctx); err != nil {
		log.ErrorCtx(ctx, "Refusing to serve request with the server's own credential",
			log.ErrorField(err),
		)
		return nil, err
	}

	clientMu.Lock()
	defer clientMu.Unlock()

	if client != nil {
		return client, nil
	}

	// Use configured user agent or default to forgejo-mcp/<version>
	userAgent := flag.UserAgent
	if userAgent == "" {
		userAgent = "forgejo-mcp/" + flag.Version
	}

	c, err := forgejo.NewClient(flag.URL,
		forgejo.SetToken(flag.Token),
		forgejo.SetUserAgent(userAgent),
	)
	if err != nil {
		log.Error("Failed to create Forgejo client",
			log.SanitizedURLField("url", flag.URL),
			log.ErrorField(err),
		)
		// Never fatal: the caller decides. At startup, RegisterTool's connection
		// test turns this into a clean "connection test failed" exit; in tests it
		// stays an ordinary error instead of killing the test binary.
		return nil, fmt.Errorf("create forgejo client: %w", err)
	}
	client = c
	log.Info("Successfully created Forgejo client",
		log.SanitizedURLField("url", flag.URL),
		log.BoolField("token_configured", flag.Token != ""),
		log.StringField("user_agent", userAgent),
	)
	return client, nil
}

// GetBaseURL returns the base URL of the Forgejo instance.
func GetBaseURL() string {
	return flag.URL
}

// VerifyConnection attempts to get basic information to verify
// that the client is properly connected.
// Uses the /version endpoint (no auth required) so that tokens scoped
// only to repo/issue — e.g. organisation tokens — are not rejected.
func VerifyConnection() error {
	start := time.Now()

	log.Debug("Starting connection verification",
		log.SanitizedURLField("url", flag.URL),
	)

	client, err := Client(context.Background())
	if err != nil {
		return err
	}
	version, resp, err := client.ServerVersion()
	duration := time.Since(start)

	if err != nil {
		log.Error("Connection verification failed",
			log.SanitizedURLField("url", flag.URL),
			log.DurationField("duration", duration),
			log.ErrorField(err),
		)
		return fmt.Errorf("failed to connect to Forgejo instance at %s: %w", flag.URL, err)
	}

	log.Info("Connection verification successful",
		log.SanitizedURLField("url", flag.URL),
		log.DurationField("duration", duration),
		log.StringField("server_version", version),
		log.IntField("response_status", resp.StatusCode),
	)

	return nil
}

// HealthCheck performs a lightweight health check
func HealthCheck() error {
	start := time.Now()

	log.Debug("Starting health check")

	client, err := Client(context.Background())
	if err != nil {
		return err
	}
	version, resp, err := client.ServerVersion()
	duration := time.Since(start)

	if err != nil {
		log.Error("Health check failed",
			log.SanitizedURLField("url", flag.URL),
			log.DurationField("duration", duration),
			log.ErrorField(err),
		)
		return fmt.Errorf("health check failed: %w", err)
	}

	log.Debug("Health check successful",
		log.SanitizedURLField("url", flag.URL),
		log.DurationField("duration", duration),
		log.StringField("server_version", version),
		log.IntField("response_status", resp.StatusCode),
	)

	return nil
}

// LogAPICall logs API call information with timing
func LogAPICall(ctx context.Context, method, endpoint string, duration time.Duration, statusCode int, err error) {
	if err != nil {
		log.ErrorCtx(ctx, "API call failed",
			log.StringField("method", method),
			log.StringField("endpoint", endpoint),
			log.DurationField("duration", duration),
			log.IntField("status_code", statusCode),
			log.ErrorField(err),
		)
	} else {
		log.DebugCtx(ctx, "API call completed",
			log.StringField("method", method),
			log.StringField("endpoint", endpoint),
			log.DurationField("duration", duration),
			log.IntField("status_code", statusCode),
		)
	}
}
