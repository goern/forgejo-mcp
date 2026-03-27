package operation

import (
	"fmt"

	"codeberg.org/goern/forgejo-mcp/v2/operation/actions"
	"codeberg.org/goern/forgejo-mcp/v2/operation/issue"
	"codeberg.org/goern/forgejo-mcp/v2/operation/org"
	"codeberg.org/goern/forgejo-mcp/v2/operation/pull"
	"codeberg.org/goern/forgejo-mcp/v2/operation/repo"
	"codeberg.org/goern/forgejo-mcp/v2/operation/search"
	"codeberg.org/goern/forgejo-mcp/v2/operation/user"
	"codeberg.org/goern/forgejo-mcp/v2/operation/version"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
	"codeberg.org/goern/forgejo-mcp/v2/pkg/log"

	"github.com/mark3labs/mcp-go/server"
)

var (
	mcpServer *server.MCPServer
)

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

func Run(transport, version string) error {
	flag.Version = version
	mcpServer = newMCPServer(version)
	RegisterTool(mcpServer)

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
		sseServer := server.NewSSEServer(mcpServer)
		log.Info("Starting MCP SSE server",
			log.IntField("port", flag.SSEPort),
		)
		log.Info("MCP SSE server ready for connections",
			log.IntField("port", flag.SSEPort),
			log.StringField("endpoint", fmt.Sprintf("http://localhost:%d", flag.SSEPort)),
		)
		if err := sseServer.Start(fmt.Sprintf(":%d", flag.SSEPort)); err != nil {
			log.Error("Failed to start SSE server",
				log.IntField("port", flag.SSEPort),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to start SSE server: %w", err)
		}
		log.Info("MCP SSE server shutdown")
	case "http":
		httpServer := server.NewStreamableHTTPServer(mcpServer)
		log.Info("Starting MCP streamable HTTP server",
			log.IntField("port", flag.HTTPPort),
		)
		log.Info("MCP streamable HTTP server ready for connections",
			log.IntField("port", flag.HTTPPort),
			log.StringField("endpoint", fmt.Sprintf("http://localhost:%d", flag.HTTPPort)),
		)
		if err := httpServer.Start(fmt.Sprintf(":%d", flag.HTTPPort)); err != nil {
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


func newMCPServer(version string) *server.MCPServer {
	return server.NewMCPServer(
		"Forgejo MCP Server",
		version,
		server.WithLogging(),
	)
}
