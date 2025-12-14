# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
go build -v         # Alternative direct build
```

## Running the Server

```bash
# stdio mode (for MCP client integration)
./forgejo-mcp --transport stdio --url https://forgejo.example.org --token <token>

# SSE mode (for HTTP-based clients)
./forgejo-mcp --transport sse --url https://forgejo.example.org --token <token> --sse-port 8080
```

Environment variables: `FORGEJO_URL`, `FORGEJO_ACCESS_TOKEN`, `FORGEJO_DEBUG`

## Architecture

This is an MCP (Model Context Protocol) server that exposes Forgejo API operations as tools for AI assistants.

### Core Flow

```
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (tool handlers)
```

### Key Directories

- `operation/` - MCP tool definitions and handlers, organized by domain (issue, pull, repo, search, user, version)
- `pkg/forgejo/` - Singleton Forgejo SDK client wrapper
- `pkg/to/` - Response formatting helpers (`TextResult`, `ErrorResult`)
- `pkg/params/` - Shared parameter descriptions for tool definitions
- `pkg/flag/` - Global configuration state
- `pkg/log/` - Structured logging utilities

### Adding a New Tool

1. Create or modify a file in `operation/{domain}/`
2. Define tool constants and `mcp.NewTool()` definitions with parameters
3. Implement handler function: `func ToolNameFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)`
4. Register in the domain's `RegisterTool(s *server.MCPServer)` function
5. If new domain, import and call `{domain}.RegisterTool(s)` in `operation/operation.go`

### Tool Implementation Pattern

```go
// Tool definition
var MyTool = mcp.NewTool(
    "tool_name",
    mcp.WithDescription("Description"),
    mcp.WithString("param", mcp.Required(), mcp.Description(params.SomeParam)),
    mcp.WithNumber("num", mcp.Description("..."), mcp.DefaultNumber(1)),
)

// Handler
func MyToolFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    param, _ := req.Params.Arguments["param"].(string)
    num, _ := req.Params.Arguments["num"].(float64)  // Numbers are float64

    result, _, err := forgejo.Client().SomeMethod(param)
    if err != nil {
        return to.ErrorResult(fmt.Errorf("operation failed: %v", err))
    }
    return to.TextResult(result)
}
```

### Dependencies

- `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2` - Forgejo API client
- `github.com/mark3labs/mcp-go` - MCP protocol implementation

## Blocked Features

Some features are blocked on upstream API/SDK support. See `docs/plans/` for:

- `wiki-support.md` - Wiki API (blocked on forgejo-sdk)
- `projects-support.md` - Projects/Kanban API (blocked on Gitea 1.26.0)

## Repository Labels

Labels for goern/forgejo-mcp on Codeberg:

| ID | Name | Color | Description |
|----|------|-------|-------------|
| 335058 | Kind/Feature | 0288d1 | New functionality |
| 335061 | Kind/Enhancement | 84b6eb | Improve existing functionality |
| 335091 | Status/Blocked | 880e4f | Something is blocking this issue or pull request |
| 335103 | Priority/Medium | e64a19 | The priority is medium |

### Usage with Codeberg MCP

When adding labels via the `mcp__codeberg__add_issue_labels` tool, use the numeric ID:

```
mcp__codeberg__add_issue_labels(
  owner: "goern",
  repo: "forgejo-mcp",
  index: <issue_number>,
  labels: "<label_id>"  # e.g., "335091" for Status/Blocked
)
```
