package operation

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/attachment"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/branchprotection"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/hook"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/org"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/pull"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/release"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/search"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/tracking"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/user"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/version"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/operation/wiki"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v2/pkg/log"

	"github.com/mark3labs/mcp-go/server"
)

var (
	mcpServer *server.MCPServer
)

// streamableHTTPEndpointPath is the library's own default. It is named here
// because the transport is mounted on a mux at this path rather than served as
// the root handler.
const streamableHTTPEndpointPath = "/mcp"

// requestTokenContextFunc lifts a per-request credential out of the
// Authorization header and into the context the tool handlers see.
func requestTokenContextFunc(ctx context.Context, r *http.Request) context.Context {
	if token := extractToken(r.Header.Get("Authorization")); token != "" {
		return forgejo.WithToken(ctx, token)
	}
	return ctx
}

// disableLibraryHostCheck reports whether the library's own loopback protection
// should be switched off.
//
// When the operator declares an explicit allow-list, this server's middleware
// owns the Host policy and is stricter than the library's, which accepts any
// loopback name. Leaving both on means a request arriving over loopback with a
// declared external name is refused by the library after our check has already
// accepted it — two policies disagreeing about one header. With no declared
// list the library's protection stays on as written.
func disableLibraryHostCheck() bool { return len(flag.AllowedHosts) > 0 }

// newSSEHandler builds the SSE transport exactly as Run does.
//
// It exists so the conformance tests drive the real construction — the option
// set, the context function and the mount — rather than a copy of it that can
// drift. A test that builds its own handler proves nothing about this one.
func newSSEHandler(s *server.MCPServer) http.Handler {
	opts := []server.SSEOption{server.WithSSEContextFunc(requestTokenContextFunc)}
	if disableLibraryHostCheck() {
		opts = append(opts, server.WithSSEDisableLocalhostProtection(true))
	}
	return server.NewSSEServer(s, opts...)
}

// newStreamableHTTPHandler builds the streamable HTTP transport exactly as Run
// does, mounted on a mux at its endpoint path. Serving the transport as the
// root handler instead would make it answer on every path.
func newStreamableHTTPHandler(s *server.MCPServer) http.Handler {
	opts := []server.StreamableHTTPOption{
		server.WithHTTPContextFunc(requestTokenContextFunc),
		server.WithEndpointPath(streamableHTTPEndpointPath),
	}
	if disableLibraryHostCheck() {
		opts = append(opts, server.WithDisableLocalhostProtection(true))
	}
	mux := http.NewServeMux()
	mux.Handle(streamableHTTPEndpointPath, server.NewStreamableHTTPServer(s, opts...))
	return mux
}

func RegisterTool(s *server.MCPServer) {
	log.Info("Registering MCP tools")

	RegisterUserTool(s)
	RegisterRepoTool(s)
	RegisterIssueTool(s)
	RegisterPullTool(s)
	RegisterPullReviewTool(s)
	RegisterSearchTool(s)
	RegisterVersionTool(s)
	RegisterActionsTool(s)
	RegisterOrgTool(s)
	RegisterTrackingTool(s)
	RegisterAttachmentTool(s)
	RegisterReleaseTool(s)
	RegisterBranchProtectionTool(s)
	RegisterHookTool(s)
	RegisterWikiTool(s)

	log.Info("All MCP tools registered successfully")
}

// Per-domain registration functions exposed for CLI domain grouping.

func RegisterUserTool(s *server.MCPServer) {
	user.RegisterTool(s)
	log.Debug("Registered user tools")
}

func RegisterRepoTool(s *server.MCPServer) {
	repo.RegisterTool(s)
	log.Debug("Registered repository tools")
}

func RegisterIssueTool(s *server.MCPServer) {
	issue.RegisterTool(s)
	log.Debug("Registered issue tools")
}

func RegisterPullTool(s *server.MCPServer) {
	pull.RegisterTool(s)
	log.Debug("Registered pull request tools")
}

func RegisterPullReviewTool(s *server.MCPServer) {
	pull.RegisterReviewTools(s)
	log.Debug("Registered pull review write tools")
}

func RegisterSearchTool(s *server.MCPServer) {
	search.RegisterTool(s)
	log.Debug("Registered search tools")
}

func RegisterVersionTool(s *server.MCPServer) {
	version.RegisterTool(s)
	log.Debug("Registered version tools")
}

func RegisterActionsTool(s *server.MCPServer) {
	actions.RegisterTool(s)
	log.Debug("Registered actions tools")
}

func RegisterOrgTool(s *server.MCPServer) {
	org.RegisterTool(s)
	log.Debug("Registered org tools")
}

func RegisterTrackingTool(s *server.MCPServer) {
	tracking.RegisterTool(s)
	log.Debug("Registered time tracking tools")
}

func RegisterAttachmentTool(s *server.MCPServer) {
	attachment.RegisterTool(s)
	log.Debug("Registered attachment tools")
}

func RegisterReleaseTool(s *server.MCPServer) {
	release.RegisterTool(s)
	log.Debug("Registered release tools")
}

func RegisterBranchProtectionTool(s *server.MCPServer) {
	branchprotection.RegisterTool(s)
	log.Debug("Registered branch protection tools")
}

func RegisterHookTool(s *server.MCPServer) {
	hook.RegisterTools(s)
	log.Debug("Registered webhook tools")
}

func RegisterWikiTool(s *server.MCPServer) {
	wiki.RegisterTool(s)
	log.Debug("Registered wiki tools")
}

func extractToken(auth string) string {
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		// No scheme prefix. Spec: bare tokens MUST be rejected and treated
		// as if no Authorization header were present (no silent identity).
		return ""
	}
	scheme := strings.ToLower(parts[0])
	if scheme == "token" || scheme == "bearer" {
		return parts[1]
	}
	return ""
}

func Run(transport, version string) error {
	flag.Version = version
	mcpServer = newMCPServer(version)
	RegisterTool(mcpServer)
	RegisterCoreResources(mcpServer)

	// Test connection to Forgejo instance before starting the server
	log.Info("Testing connection to Forgejo instance",
		log.SanitizedURLField("url", flag.URL),
	)
	if err := testConnection(); err != nil {
		log.Error("Failed to connect to Forgejo instance",
			log.SanitizedURLField("url", flag.URL),
			log.ErrorField(err),
		)
		return fmt.Errorf("connection test failed: %w", err)
	}
	log.Info("Successfully connected to Forgejo instance",
		log.SanitizedURLField("url", flag.URL),
	)

	switch transport {
	case "stdio":
		log.Info("Starting MCP server with stdio transport")
		log.Info("MCP server ready for stdio communication")
		if err := server.ServeStdio(mcpServer); err != nil {
			log.Error("MCP stdio server failed",
				log.ErrorField(err),
			)
			return err
		}
		log.Info("MCP stdio server shutdown")
	case "sse":
		sseServer := newSSEHandler(mcpServer)
		log.Info("Starting MCP SSE server",
			log.IntField("port", flag.SSEPort),
		)
		if err := serveMCPOverHTTP("sse", sseServer, flag.SSEPort); err != nil {
			log.Error("Failed to start SSE server",
				log.IntField("port", flag.SSEPort),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to start SSE server: %w", err)
		}
		log.Info("MCP SSE server shutdown")
	case "http":
		httpMux := newStreamableHTTPHandler(mcpServer)
		log.Info("Starting MCP streamable HTTP server",
			log.IntField("port", flag.HTTPPort),
		)
		if err := serveMCPOverHTTP("http", httpMux, flag.HTTPPort); err != nil {
			log.Error("Failed to start streamable HTTP server",
				log.IntField("port", flag.HTTPPort),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to start streamable HTTP server: %w", err)
		}
		log.Info("MCP streamable HTTP server shutdown")
	default:
		log.Error("Invalid transport configuration",
			log.StringField("transport", transport),
			log.StringField("valid_options", "stdio, sse, http"),
		)
		return fmt.Errorf("invalid transport type: %s. Must be 'stdio', 'sse', or 'http'", transport)
	}
	return nil
}

func testConnection() error {
	return forgejo.VerifyConnection()
}

func RegisterCoreResources(s *server.MCPServer) {
	RegisterCommitResource(s)
	RegisterIssueResources(s)
	RegisterLabelResources(s)
	RegisterOwnerResource(s)
	RegisterPullResources(s)
	RegisterRepoResource(s)
	RegisterStatusResource(s)
	RegisterBranchProtectionResources(s)
	RegisterHookResources(s)
	RegisterWikiResource(s)
	log.Debug("Registered core resource templates")
}

func RegisterWikiResource(s *server.MCPServer) {
	wiki.RegisterWikiResource(s)
	log.Debug("Registered wiki resource template")
}

func RegisterLabelResources(s *server.MCPServer) {
	issue.RegisterLabelResources(s)
	log.Debug("Registered label resource templates")
}

func RegisterBranchProtectionResources(s *server.MCPServer) {
	branchprotection.RegisterResource(s)
	log.Debug("Registered branch protection resource templates")
}

func RegisterHookResources(s *server.MCPServer) {
	hook.RegisterResources(s)
	log.Debug("Registered webhook resource templates")
}

func RegisterPullResources(s *server.MCPServer) {
	pull.RegisterPullResources(s)
	log.Debug("Registered pull request resource template")
}

func RegisterIssueResources(s *server.MCPServer) {
	issue.RegisterIssueResources(s)
	log.Debug("Registered issue resource templates")
}

func RegisterCommitResource(s *server.MCPServer) {
	repo.RegisterCommitResource(s)
	log.Debug("Registered commit resource template")
}

func RegisterOwnerResource(s *server.MCPServer) {
	user.RegisterOwnerResource(s)
	log.Debug("Registered owner resource template")
}

func RegisterRepoResource(s *server.MCPServer) {
	repo.RegisterRepoResource(s)
	log.Debug("Registered repo resource template")
}

func RegisterStatusResource(s *server.MCPServer) {
	repo.RegisterStatusResource(s)
	log.Debug("Registered commit status resource template")
}

func newMCPServer(version string) *server.MCPServer {
	return server.NewMCPServer(
		"Forgejo MCP Server",
		version,
		server.WithLogging(),
		server.WithResourceCapabilities(false, false),
	)
}
