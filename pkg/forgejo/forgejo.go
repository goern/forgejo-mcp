package forgejo

import (
	"fmt"
	"sync"

	"forgejo.com/forgejo/forgejo-mcp/pkg/flag"
	"forgejo.com/forgejo/forgejo-mcp/pkg/log"

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
			c, err := forgejo.NewClient(flag.Host, forgejo.SetToken(flag.Token))
			if err != nil {
				log.Fatalf("create forgejo client err: %v", err)
			}
			client = c
			log.Debugf("Created client for %s", flag.Host)
		}
	})
	return client
}

// VerifyConnection attempts to get basic information to verify
// that the client is properly connected
func VerifyConnection() error {
	// Try to get user info as a basic connectivity test
	_, _, err := Client().GetMyUserInfo()
	if err != nil {
		return fmt.Errorf("failed to connect to Forgejo instance at %s: %v", flag.Host, err)
	}
	return nil
}
