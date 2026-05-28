package resource

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTemplate registers a resource template and its handler on the MCP server.
// This is a thin wrapper around mcp.NewResourceTemplate + s.AddResourceTemplate.
func RegisterTemplate(
	s *server.MCPServer,
	uriTemplate string,
	name string,
	handler server.ResourceTemplateHandlerFunc,
	opts ...mcp.ResourceTemplateOption,
) {
	t := mcp.NewResourceTemplate(uriTemplate, name, opts...)
	s.AddResourceTemplate(t, handler)
}
