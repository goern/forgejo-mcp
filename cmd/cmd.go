package cmd

import (
	"context"
	"errors"
	stdflag "flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation"
	flagPkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"
)

var (
	transport      string
	urlFlag        string
	ssePort        int
	httpPort       int
	token          string
	userAgent      string
	host           string
	allowedHosts   string
	allowedOrigins string
	allowFallback  bool

	// flagSet is retained so initConfig can ask whether a flag was actually
	// passed, rather than comparing it to its default value — which cannot
	// tell "unset" from "explicitly set to the default".
	flagSet *stdflag.FlagSet

	debug bool
)

// isVersionRequest returns true for both the "version" subcommand and the
// GNU-standard --version / -version flags.  All three forms must exit before
// flag.Parse() runs so that --url is not required.
func isVersionRequest() bool {
	if len(os.Args) < 2 {
		return false
	}
	arg := os.Args[1]
	return arg == "version" || arg == "--version" || arg == "-version"
}

// initFlags registers and parses CLI flags using a dedicated FlagSet to avoid
// polluting the global flag.CommandLine (which breaks `go test`).
func initFlags() {
	fs := stdflag.NewFlagSet("forgejo-mcp", stdflag.ExitOnError)
	flagSet = fs

	fs.StringVar(
		&transport,
		"t",
		"stdio",
		"Transport type (stdio, sse, or http)",
	)
	fs.StringVar(
		&transport,
		"transport",
		"stdio",
		"Transport type (stdio, sse, or http)",
	)
	fs.StringVar(
		&urlFlag,
		"url",
		"",
		"Forgejo instance URL (required, must start with http:// or https://)",
	)
	fs.IntVar(
		&ssePort,
		"sse-port",
		8080,
		"Port for SSE transport mode",
	)
	fs.IntVar(
		&httpPort,
		"http-port",
		8080,
		"Port for streamable HTTP transport mode",
	)
	fs.StringVar(
		&token,
		"token",
		"",
		"Your personal access token",
	)
	fs.StringVar(
		&userAgent,
		"user-agent",
		"",
		"User agent for HTTP requests (default: forgejo-mcp/<version>)",
	)
	fs.StringVar(
		&host,
		"host",
		"127.0.0.1",
		"Address the sse and http transports bind to. The default reaches this machine only; "+
			"set 0.0.0.0 to accept connections from the network, which also requires -allowed-hosts",
	)
	fs.StringVar(
		&allowedHosts,
		"allowed-hosts",
		"",
		"Comma-separated Host names this server answers to, required when -host is not loopback "+
			"(for example: mcp.example.org,mcp.example.org:8080)",
	)
	fs.StringVar(
		&allowedOrigins,
		"allowed-origins",
		"",
		"Comma-separated web origins allowed to send an Origin header, as full origins "+
			"(for example: https://console.example.org). Empty means no browser origin is accepted; "+
			"requests with no Origin header are unaffected",
	)
	fs.BoolVar(
		&allowFallback,
		"allow-operator-token-fallback",
		false,
		"On the sse and http transports, serve a request that carries no Authorization header using "+
			"this server's own token. Off by default: it hands this server's forge credential to any "+
			"caller that can reach the port",
	)
	fs.BoolVar(
		&debug,
		"d",
		false,
		"debug mode",
	)
	fs.BoolVar(
		&debug,
		"debug",
		false,
		"debug mode",
	)

	// ExitOnError: Parse exits the process on error, so the return is moot.
	_ = fs.Parse(os.Args[1:])

	flagPkg.URL = urlFlag
	flagPkg.UserAgent = userAgent
	initConfig()
}

// initConfig resolves URL, token, and debug from flags and environment variables.
func initConfig() {
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
	flagPkg.HTTPPort = httpPort

	flagPkg.Host = resolveString(host, "FORGEJO_MCP_HOST", "host")
	flagPkg.AllowedHosts = splitList(resolveString(allowedHosts, "FORGEJO_MCP_ALLOWED_HOSTS", "allowed-hosts"))
	flagPkg.AllowedOrigins = splitList(resolveString(allowedOrigins, "FORGEJO_MCP_ALLOWED_ORIGINS", "allowed-origins"))
	flagPkg.AllowOperatorTokenFallback = allowFallback
	if !flagWasPassed("allow-operator-token-fallback") {
		if v := os.Getenv("FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK"); v != "" {
			flagPkg.AllowOperatorTokenFallback = isTruthy(v)
			log.Debug("Using FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK environment variable")
		}
	}
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

	// User agent - CLI flag takes precedence, then environment variable, then default
	if flagPkg.UserAgent == "" {
		flagPkg.UserAgent = os.Getenv("FORGEJO_USER_AGENT")
		if flagPkg.UserAgent != "" {
			log.Debug("Using FORGEJO_USER_AGENT environment variable")
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
	if isVersionRequest() {
		fmt.Printf("forgejo-mcp %s\n", version)
		return
	}

	// CLI mode: detect --cli early, skip default flag parsing since CLI
	// has its own args (tool name, --args, --output) that would confuse it.
	cliMode = hasCLIFlag()
	if cliMode {
		initConfig()
	} else {
		initFlags()
	}

	// Set default user agent if not provided via CLI or env var
	if flagPkg.UserAgent == "" {
		flagPkg.UserAgent = "forgejo-mcp/" + version
		log.Debug("Using default user agent",
			log.StringField("user_agent", flagPkg.UserAgent),
		)
	}

	// Sync flushes buffered logs at exit; its error (e.g. syncing stderr) is
	// not actionable here.
	defer func() { _ = log.Default().Sync() }()

	if cliMode {
		RunCLI(version)
		return
	}

	log.Infof("Starting Forgejo MCP Server %s", version)
	log.Info("Server configuration loaded",
		log.SanitizedURLField("url", flagPkg.URL),
		log.StringField("transport", transport),
		log.IntField("sse-port", flagPkg.SSEPort),
		log.BoolField("debug", flagPkg.Debug),
		log.BoolField("token_configured", flagPkg.Token != ""),
		log.StringField("user_agent", flagPkg.UserAgent),
	)

	if err := operation.Run(transport, version); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("Server shutdown due to context cancellation")
			return
		}
		log.Fatalf("Run Forgejo MCP Server Error: %v", err)
	}
}

// flagWasPassed reports whether the named flag was actually given on the
// command line. Comparing a flag to its default value cannot tell "not set"
// from "set to the default", which is how an environment variable came to
// override an explicit -host 127.0.0.1.
func flagWasPassed(name string) bool {
	if flagSet == nil {
		return false
	}
	passed := false
	flagSet.Visit(func(f *stdflag.Flag) {
		if f.Name == name {
			passed = true
		}
	})
	return passed
}

// resolveString returns the flag value when the flag was passed, otherwise the
// environment variable, otherwise the flag's default.
func resolveString(value, envVar, flagName string) string {
	if flagWasPassed(flagName) {
		return value
	}
	if env := os.Getenv(envVar); env != "" {
		log.Debug("Using environment variable", log.StringField("variable", envVar))
		return env
	}
	return value
}

// splitList turns a comma-separated value into a trimmed, non-empty list.
func splitList(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// isTruthy matches the spelling already accepted by this project's other
// boolean environment variables.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
