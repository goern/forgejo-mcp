package gitea

import (
	"sync"

	"gitea.com/gitea/gitea-mcp/pkg/flag"
	"gitea.com/gitea/gitea-mcp/pkg/log"

	"code.gitea.io/sdk/gitea"
)

var (
	client     *gitea.Client
	clientOnce sync.Once
)

func Client() *gitea.Client {
	clientOnce.Do(func() {
		if client == nil {
			c, err := gitea.NewClient(flag.Host, gitea.SetToken(flag.Token))
			if err != nil {
				log.Fatalf("create gitea client err: %v", err)
			}
			client = c
		}
	})
	return client
}
