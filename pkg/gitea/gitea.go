package gitea

import (
	"os"
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
			host, token := flag.Host, flag.Token
			if host == "" {
				host = os.Getenv("GITEA_HOST")
			}
			if host == "" {
				host = "https://gitea.com"
			}
			if token == "" {
				token = os.Getenv("GITEA_TOKEN")
			}

			c, err := gitea.NewClient(host, gitea.SetToken(token))
			if err != nil {
				log.Fatalf("create gitea client err: %v", err)
			}
			client = c
		}
	})
	return client
}
