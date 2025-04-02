package operation

import (
	"fmt"

	"forgejo.com/forgejo/forgejo-mcp/operation/issue"
	"forgejo.com/forgejo/forgejo-mcp/operation/pull"
	"forgejo.com/forgejo/forgejo-mcp/operation/repo"
	"forgejo.com/forgejo/forgejo-mcp/operation/search"
	"forgejo.com/forgejo/forgejo-mcp/operation/user"
	"forgejo.com/forgejo/forgejo-mcp/operation/version"
	"forgejo.com/forgejo/forgejo-mcp/pkg/flag"
	"forgejo.com/forgejo/forgejo-mcp/pkg/log"

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
	switch transport {
	case "stdio":
		if err := server.ServeStdio(mcpServer); err != nil {
			return err
		}
	case "sse":
		sseServer := server.NewSSEServer(mcpServer)
		log.Infof("Forgejo MCP SSE server listening on :%d", flag.Port)
		if err := sseServer.Start(fmt.Sprintf(":%d", flag.Port)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid transport type: %s. Must be 'stdio' or 'sse'", transport)
	}
	return nil
}

func newMCPServer(version string) *server.MCPServer {
	return server.NewMCPServer(
		"Forgejo MCP Server",
		version,
		server.WithLogging(),
	)
}
