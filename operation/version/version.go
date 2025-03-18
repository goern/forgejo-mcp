package version

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/flag"
	"gitea.com/gitea/gitea-mcp/pkg/to"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetGiteaMCPServerVersion = "get_gitea_mcp_server_version"
)

var (
	GetGiteaMCPServerVersionTool = mcp.NewTool(
		GetGiteaMCPServerVersion,
		mcp.WithDescription("Get Gitea MCP Server Version"),
	)
)

func GetGiteaMCPServerVersionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	version := flag.Version
	if version == "" {
		version = "dev"
	}
	return to.TextResult(fmt.Sprintf("Gitea MCP Server version: %v", version))
}
