package operation

import (
	"fmt"

	"forgejo.org/forgejo/forgejo-mcp/operation/issue"
	"forgejo.org/forgejo/forgejo-mcp/operation/pull"
	"forgejo.org/forgejo/forgejo-mcp/operation/repo"
	"forgejo.org/forgejo/forgejo-mcp/operation/search"
	"forgejo.org/forgejo/forgejo-mcp/operation/user"
	"forgejo.org/forgejo/forgejo-mcp/operation/version"
	"forgejo.org/forgejo/forgejo-mcp/pkg/flag"
	"forgejo.org/forgejo/forgejo-mcp/pkg/forgejo"
	"forgejo.org/forgejo/forgejo-mcp/pkg/log"

	"github.com/mark3labs/mcp-go/server"
)

var (
	mcpServer *server.MCPServer
)

func RegisterTool(s *server.MCPServer) {
	log.Info("Registering MCP tools")

	// User Tool
	user.RegisterTool(s)
	log.Debug("Registered user tools")

	// Repo Tool
	repo.RegisterTool(s)
	log.Debug("Registered repository tools")

	// Issue Tool
	issue.RegisterTool(s)
	log.Debug("Registered issue tools")

	// Pull Tool
	pull.RegisterTool(s)
	log.Debug("Registered pull request tools")

	// Search Tool
	search.RegisterTool(s)
	log.Debug("Registered search tools")

	// Version Tool
	version.RegisterTool(s)
	log.Debug("Registered version tools")

	log.Info("All MCP tools registered successfully")
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
	default:
		log.Error("Invalid transport configuration",
			log.StringField("transport", transport),
			log.StringField("valid_options", "stdio, sse"),
		)
		return fmt.Errorf("invalid transport type: %s. Must be 'stdio' or 'sse'", transport)
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
