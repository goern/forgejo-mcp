package cmd

import (
	"context"
	"flag"
	"os"

	"gitea.com/gitea/gitea-mcp/operation"
	flagPkg "gitea.com/gitea/gitea-mcp/pkg/flag"
	"gitea.com/gitea/gitea-mcp/pkg/log"
)

var (
	transport string
	host      string
	token     string

	debug bool
)

func init() {
	flag.StringVar(
		&transport,
		"t",
		"stdio",
		"Transport type (stdio or sse)",
	)
	flag.StringVar(
		&transport,
		"transport",
		"stdio",
		"Transport type (stdio or sse)",
	)
	flag.StringVar(
		&host,
		"host",
		"https://gitea.com",
		"Gitea host",
	)
	flag.StringVar(
		&token,
		"token",
		"",
		"Your personal access token",
	)
	flag.BoolVar(
		&debug,
		"d",
		false,
		"debug mode",
	)
	flag.BoolVar(
		&debug,
		"debug",
		false,
		"debug mode",
	)

	flag.Parse()

	flagPkg.Host = host
	if flagPkg.Host == "" {
		flagPkg.Host = os.Getenv("GITEA_HOST")
	}
	if flagPkg.Host == "" {
		flagPkg.Host = "https://gitea.com"
	}

	flagPkg.Token = token
	if flagPkg.Token == "" {
		flagPkg.Token = os.Getenv("GITEA_TOKEN")
	}

	if debug {
		flagPkg.Debug = debug
	}
	if !debug {
		flagPkg.Debug = os.Getenv("GITEA_DEBUG") == "true"
	}
}

func Execute(version string) {
	defer log.Default().Sync()
	if err := operation.Run(transport, version); err != nil {
		if err == context.Canceled {
			log.Info("Server shutdown due to context cancellation")
			return
		}
		log.Fatalf("Run Gitea MCP Server Error: %v", err)
	}
}
