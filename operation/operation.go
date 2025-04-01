package operation

import (
	"fmt"

	"gitea.com/gitea/gitea-mcp/operation/issue"
	"gitea.com/gitea/gitea-mcp/operation/pull"
	"gitea.com/gitea/gitea-mcp/operation/repo"
	"gitea.com/gitea/gitea-mcp/operation/search"
	"gitea.com/gitea/gitea-mcp/operation/user"
	"gitea.com/gitea/gitea-mcp/operation/version"
	"gitea.com/gitea/gitea-mcp/pkg/flag"
	"gitea.com/gitea/gitea-mcp/pkg/log"

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
		log.Infof("Gitea MCP SSE server listening on :%d", flag.Port)
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
		"Gitea MCP Server",
		version,
		server.WithLogging(),
	)
}
