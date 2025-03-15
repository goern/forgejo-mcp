package operation

import (
	"fmt"

	"gitea.com/gitea/gitea-mcp/operation/repo"
	"gitea.com/gitea/gitea-mcp/operation/user"
	"gitea.com/gitea/gitea-mcp/pkg/log"
	"github.com/mark3labs/mcp-go/server"
)

var (
	mcpServer *server.MCPServer
)

func RegisterTool(s *server.MCPServer) {
	// User Tool
	s.AddTool(user.GetMyUserInfoTool, user.MyUserInfoFn)

	// Repo Tool
	s.AddTool(repo.GetMyReposTool, repo.MyUserReposFn)
}

func Run(transport, version string) error {
	mcpServer = newMCPServer(version)
	RegisterTool(mcpServer)
	switch transport {
	case "stdio":
		if err := server.ServeStdio(mcpServer); err != nil {
			return err
		}
	case "sse":
		sseServer := server.NewSSEServer(mcpServer)
		log.Infof("Gitea MCP SSE server listening on :8080")
		if err := sseServer.Start(":8080"); err != nil {
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
