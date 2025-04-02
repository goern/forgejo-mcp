package cmd

import (
	"context"
	"flag"
	"os"

	"forgejo.org/forgejo/forgejo-mcp/operation"
	flagPkg "forgejo.org/forgejo/forgejo-mcp/pkg/flag"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
)

var (
	transport string
	host      string
	port      int
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
		"https://forgejo.org",
		"Forgejo host",
	)
	flag.IntVar(
		&port,
		"port",
		8080,
		"sse port",
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
		true,
		"debug mode",
	)
	flag.BoolVar(
		&debug,
		"debug",
		true,
		"debug mode",
	)

	flag.Parse()

	flagPkg.Host = host
	if flagPkg.Host == "" {
		flagPkg.Host = os.Getenv("GITEA_HOST")
	}
	if flagPkg.Host == "" {
		flagPkg.Host = "https://forgejo.org"
	}

	flagPkg.Port = port

	flagPkg.Token = token
	if flagPkg.Token == "" {
		flagPkg.Token = os.Getenv("GITEA_ACCESS_TOKEN")
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
		log.Fatalf("Run Forgejo MCP Server Error: %v", err)
	}
}
