package cmd

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"

	"codeberg.org/goern/forgejo-mcp/operation"
	flagPkg "codeberg.org/goern/forgejo-mcp/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/pkg/log"
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
		flagPkg.URL = os.Getenv("FORGEJO_URL")
		if flagPkg.URL != "" {
			log.Debug("Using FORGEJO_URL environment variable")
		}
	}
	if flagPkg.URL == "" {
		// Fallback to deprecated GITEA_HOST with warning
		if giteaHost := os.Getenv("GITEA_HOST"); giteaHost != "" {
			log.Warn("Deprecated environment variable used",
				log.StringField("deprecated_var", "GITEA_HOST"),
				log.StringField("preferred_var", "FORGEJO_URL"),
				log.StringField("migration_help", "Please update your configuration to use FORGEJO_URL"),
			)
			flagPkg.URL = giteaHost
		}
	}
	if flagPkg.URL == "" {
		log.Fatal("Missing required configuration",
			log.StringField("missing", "url"),
			log.StringField("help", "Provide URL with -url flag or FORGEJO_URL environment variable"),
		)
	}

	// Validate URL has proper scheme
	log.Debug("Validating URL configuration",
		log.SanitizedURLField("url", flagPkg.URL),
	)
	if err := validateURL(flagPkg.URL); err != nil {
		log.Fatal("Invalid URL configuration",
			log.SanitizedURLField("url", flagPkg.URL),
			log.ErrorField(err),
		)
	}

	flagPkg.SSEPort = ssePort
	flagPkg.Token = token
	if flagPkg.Token == "" {
		flagPkg.Token = os.Getenv("FORGEJO_ACCESS_TOKEN")
		if flagPkg.Token != "" {
			log.Debug("Using FORGEJO_ACCESS_TOKEN environment variable")
		}
	}
	if flagPkg.Token == "" {
		// Fallback to deprecated GITEA_ACCESS_TOKEN with warning
		if giteaToken := os.Getenv("GITEA_ACCESS_TOKEN"); giteaToken != "" {
			log.Warn("Deprecated environment variable used",
				log.StringField("deprecated_var", "GITEA_ACCESS_TOKEN"),
				log.StringField("preferred_var", "FORGEJO_ACCESS_TOKEN"),
				log.StringField("migration_help", "Please update your configuration to use FORGEJO_ACCESS_TOKEN"),
			)
			flagPkg.Token = giteaToken
		}
	}

	if debug {
		flagPkg.Debug = debug
		log.Debug("Debug mode enabled via flag")
	}
	if !debug {
		flagPkg.Debug = os.Getenv("FORGEJO_DEBUG") == "true"
		if flagPkg.Debug {
			log.Debug("Debug mode enabled via FORGEJO_DEBUG environment variable")
		}
		if !flagPkg.Debug {
			// Fallback to deprecated GITEA_DEBUG with warning
			if os.Getenv("GITEA_DEBUG") == "true" {
				log.Warn("Deprecated environment variable used",
					log.StringField("deprecated_var", "GITEA_DEBUG"),
					log.StringField("preferred_var", "FORGEJO_DEBUG"),
					log.StringField("migration_help", "Please update your configuration to use FORGEJO_DEBUG"),
				)
				flagPkg.Debug = true
			}
		}
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
	log.Info("Server configuration loaded",
		log.SanitizedURLField("url", flagPkg.URL),
		log.StringField("transport", transport),
		log.IntField("sse-port", flagPkg.SSEPort),
		log.BoolField("debug", flagPkg.Debug),
		log.BoolField("token_configured", flagPkg.Token != ""),
	)

	if err := operation.Run(transport, version); err != nil {
		if err == context.Canceled {
			log.Info("Server shutdown due to context cancellation")
			return
		}
		log.Fatalf("Run Forgejo MCP Server Error: %v", err)
	}
}
