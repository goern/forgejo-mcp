package forgejo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"

	"codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2"
)

var (
	client     *forgejo.Client
	clientOnce sync.Once
)

// Client returns a Forgejo client configured to connect to a Forgejo instance
// We use the standard Forgejo SDK to ensure API compatibility
func Client() *forgejo.Client {
	clientOnce.Do(func() {
		if client == nil {
			c, err := forgejo.NewClient(flag.URL, forgejo.SetToken(flag.Token))
			if err != nil {
				log.Error("Failed to create Forgejo client",
					log.SanitizedURLField("url", flag.URL),
					log.ErrorField(err),
				)
				log.Fatalf("create forgejo client err: %v", err)
			}
			client = c
			log.Info("Successfully created Forgejo client",
				log.SanitizedURLField("url", flag.URL),
				log.BoolField("token_configured", flag.Token != ""),
			)
		}
	})
	return client
}


// GetBaseURL returns the base URL of the Forgejo instance.
func GetBaseURL() string {
	return flag.URL
}

// VerifyConnection attempts to get basic information to verify
// that the client is properly connected
func VerifyConnection() error {
	start := time.Now()

	log.Debug("Starting connection verification",
		log.SanitizedURLField("url", flag.URL),
	)

	// Try to get user info as a basic connectivity test
	user, resp, err := Client().GetMyUserInfo()
	duration := time.Since(start)

	if err != nil {
		log.Error("Connection verification failed",
			log.SanitizedURLField("url", flag.URL),
			log.DurationField("duration", duration),
			log.ErrorField(err),
		)
		return fmt.Errorf("failed to connect to Forgejo instance at %s: %v", flag.URL, err)
	}

	log.Info("Connection verification successful",
		log.SanitizedURLField("url", flag.URL),
		log.DurationField("duration", duration),
		log.StringField("authenticated_user", user.UserName),
		log.IntField("response_status", resp.StatusCode),
	)

	return nil
}

// HealthCheck performs a lightweight health check
func HealthCheck() error {
	start := time.Now()

	log.Debug("Starting health check")

	// Perform a lightweight API call to check connectivity
	// Use the same call as VerifyConnection for consistency
	_, resp, err := Client().GetMyUserInfo()
	duration := time.Since(start)

	if err != nil {
		log.Error("Health check failed",
			log.SanitizedURLField("url", flag.URL),
			log.DurationField("duration", duration),
			log.ErrorField(err),
		)
		return fmt.Errorf("health check failed: %v", err)
	}

	log.Debug("Health check successful",
		log.SanitizedURLField("url", flag.URL),
		log.DurationField("duration", duration),
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
