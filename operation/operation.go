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
	// User Tool
	user.RegisterTool(s)

	// Repo Tool
	repo.RegisterTool(s)

	// Issue Tool
	issue.RegisterTool(s)

	// Pull Tool
	pull.RegisterTool(s)

	// Search Tool
	search.RegisterTool(s)

	// Version Tool
	version.RegisterTool(s)
}

func Run(transport, version string) error {
	flag.Version = version
	mcpServer = newMCPServer(version)
	RegisterTool(mcpServer)

	// Test connection to Forgejo instance before starting the server
	log.Infof("Testing connection to Forgejo instance at %s", flag.URL)
	if err := testConnection(); err != nil {
		log.Errorf("Failed to connect to Forgejo instance: %v", err)
		return fmt.Errorf("connection test failed: %w", err)
	}
	log.Infof("Successfully connected to Forgejo instance")

	switch transport {
	case "stdio":
		log.Infof("Starting MCP server with stdio transport")
		if err := server.ServeStdio(mcpServer); err != nil {
			return err
		}
	case "sse":
		sseServer := server.NewSSEServer(mcpServer)
		log.Infof("Starting MCP SSE server on port %d", flag.SSEPort)
		if err := sseServer.Start(fmt.Sprintf(":%d", flag.SSEPort)); err != nil {
			return fmt.Errorf("failed to start SSE server: %w", err)
		}
	default:
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
