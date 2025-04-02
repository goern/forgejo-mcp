package gitea

import (
	"fmt"
	"sync"

	"gitea.com/gitea/gitea-mcp/pkg/flag"
	"gitea.com/gitea/gitea-mcp/pkg/log"

	"code.gitea.io/sdk/gitea"
)

var (
	client     *gitea.Client
	clientOnce sync.Once
)

// Client returns a Gitea client configured to connect to a Forgejo instance
// We use the standard Gitea SDK to ensure API compatibility
func Client() *gitea.Client {
	clientOnce.Do(func() {
		if client == nil {
			c, err := gitea.NewClient(flag.Host, gitea.SetToken(flag.Token))
			if err != nil {
				log.Fatalf("create gitea client err: %v", err)
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
		return fmt.Errorf("failed to connect to Forgejo/Gitea instance at %s: %v", flag.Host, err)
	}
	return nil
}