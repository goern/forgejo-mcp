package cmd

import (
	"flag"

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
		"debug",
		false,
		"debug mode",
	)

	flag.Parse()

	flagPkg.Host = host
	flagPkg.Token = token
}

func Execute(version string) {
	defer log.Sync()
	if err := operation.Run(transport, version); err != nil {
		log.Fatalf("Run Gitea MCP Server Error: %v", err)
	}
}
