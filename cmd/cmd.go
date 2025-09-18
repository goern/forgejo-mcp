package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"

	"forgejo.org/forgejo/forgejo-mcp/operation"
	flagPkg "forgejo.org/forgejo/forgejo-mcp/pkg/flag"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"
)

var (
	transport string
	urlFlag   string
	ssePort   int
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
		&urlFlag,
		"url",
		"",
		"Forgejo instance URL (required, must start with http:// or https://)",
	)
	flag.IntVar(
		&ssePort,
		"sse-port",
		8080,
		"Port for SSE transport mode",
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

	flagPkg.URL = urlFlag
	if flagPkg.URL == "" {
		flagPkg.URL = os.Getenv("GITEA_HOST")
	}
	if flagPkg.URL == "" {
		log.Fatalf("URL is required. Please provide a Forgejo instance URL with -url flag or GITEA_HOST environment variable")
	}

	// Validate URL has proper scheme
	if err := validateURL(flagPkg.URL); err != nil {
		log.Fatalf("Invalid URL: %v", err)
	}

	flagPkg.SSEPort = ssePort
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

func validateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must start with http:// or https://, got: %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include a host")
	}

	return nil
}

func Execute(version string) {
	defer log.Default().Sync()

	log.Infof("Starting Forgejo MCP Server %s", version)
	log.Infof("Configuration: url=%s, transport=%s, sse-port=%d, debug=%t", flagPkg.URL, transport, flagPkg.SSEPort, flagPkg.Debug)

	if err := operation.Run(transport, version); err != nil {
		if err == context.Canceled {
			log.Info("Server shutdown due to context cancellation")
			return
		}
		log.Fatalf("Run Forgejo MCP Server Error: %v", err)
	}
}
