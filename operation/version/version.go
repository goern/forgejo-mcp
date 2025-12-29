package version

import (
	"context"
	"fmt"

	"codeberg.org/goern/forgejo-mcp/pkg/flag"
	"codeberg.org/goern/forgejo-mcp/pkg/log"
	"codeberg.org/goern/forgejo-mcp/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetForgejoMCPServerVersion = "get_forgejo_mcp_server_version"
)

var (
	GetForgejoMCPServerVersionTool = mcp.NewTool(
		GetForgejoMCPServerVersion,
		mcp.WithDescription("Get MCP server version"),
	)
)

func RegisterTool(s *server.MCPServer) {
	s.AddTool(GetForgejoMCPServerVersionTool, GetForgejoMCPServerVersionFn)
}

func GetForgejoMCPServerVersionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetForgejoMCPServerVersionFn")
	version := flag.Version
	if version == "" {
		version = "dev"
	}
	return to.TextResult(fmt.Sprintf("Forgejo MCP Server version: %v", version))
}
