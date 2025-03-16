package version

import (
	"context"
	"fmt"

	"gitea.com/gitea/gitea-mcp/pkg/flag"
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

func GetGiteaMCPServerVersionFn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	version := flag.Version
	if version == "" {
		version = "dev"
	}
	return mcp.NewToolResultText(fmt.Sprintf("Gitea MCP Server version: %v", version)), nil
}
