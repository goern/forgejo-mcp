# Developer Guide

This guide covers building, developing, and contributing to the Forgejo MCP Server.

## Prerequisites

- Go 1.24 or later
- make (optional, for convenience commands)

## Building

### Using Make

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
```

### Using Go Directly

```bash
go build -v         # Build the binary
go mod tidy         # Tidy dependencies
```

## Running Locally

```bash
# stdio mode (for MCP client integration)
./forgejo-mcp --transport stdio --url https://forgejo.example.org --token <token>

# SSE mode (for HTTP-based clients)
./forgejo-mcp --transport sse --url https://forgejo.example.org --token <token> --sse-port 8080

# With debug logging
./forgejo-mcp --transport sse --url <url> --token <token> --debug
```

Environment variables: `FORGEJO_URL`, `FORGEJO_ACCESS_TOKEN`, `FORGEJO_DEBUG`

## Architecture

This is an MCP (Model Context Protocol) server that exposes Forgejo API operations as tools for AI assistants.

### Core Flow

```
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (tool handlers)
```

### Directory Structure

| Directory | Purpose |
|-----------|---------|
| `cmd/` | CLI entry point and command parsing |
| `operation/` | MCP tool definitions and handlers, organized by domain |
| `operation/issue/` | Issue-related tools |
| `operation/pull/` | Pull request tools |
| `operation/repo/` | Repository and branch tools |
| `operation/search/` | Search tools (users, repos, teams) |
| `operation/user/` | User info tools |
| `operation/version/` | Server version tool |
| `pkg/forgejo/` | Singleton Forgejo SDK client wrapper |
| `pkg/to/` | Response formatting helpers (`TextResult`, `ErrorResult`) |
| `pkg/params/` | Shared parameter descriptions for tool definitions |
| `pkg/flag/` | Global configuration state |
| `pkg/log/` | Structured logging utilities |

## Adding a New Tool

### Step 1: Create or Modify a Domain File

Tools are organized by domain in `operation/{domain}/`. Create a new file or add to an existing one.

### Step 2: Define the Tool

```go
package mydomain

import (
    "context"
    "fmt"

    "codeberg.org/goern/forgejo-mcp/v2/pkg/forgejo"
    "codeberg.org/goern/forgejo-mcp/v2/pkg/params"
    "codeberg.org/goern/forgejo-mcp/v2/pkg/to"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// Tool definition
var MyTool = mcp.NewTool(
    "my_tool_name",
    mcp.WithDescription("What this tool does"),
    mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
    mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
    mcp.WithNumber("limit", mcp.Description("Page size"), mcp.DefaultNumber(20)),
)

// Handler function
func MyToolFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // Extract parameters (numbers come as float64)
    owner, _ := req.Params.Arguments["owner"].(string)
    repo, _ := req.Params.Arguments["repo"].(string)
    limit, _ := req.Params.Arguments["limit"].(float64)

    // Call Forgejo API
    result, _, err := forgejo.Client().SomeMethod(owner, repo, int(limit))
    if err != nil {
        return to.ErrorResult(fmt.Errorf("operation failed: %v", err))
    }

    // Return formatted result
    return to.TextResult(result)
}
```

### Step 3: Register the Tool

Add registration in the domain's file:

```go
func RegisterTool(s *server.MCPServer) {
    s.AddTool(MyTool, MyToolFn)
}
```

### Step 4: Wire Up New Domains

If you created a new domain, import and register it in `operation/operation.go`:

```go
import "codeberg.org/goern/forgejo-mcp/v2/operation/mydomain"

func RegisterTools(s *server.MCPServer) {
    // ... existing registrations
    mydomain.RegisterTool(s)
}
```

## Key Patterns

### Parameter Handling

- String parameters: `value, _ := req.Params.Arguments["param"].(string)`
- Number parameters: `value, _ := req.Params.Arguments["num"].(float64)` (always float64)
- Optional with defaults: Check if value exists before using

### Response Formatting

Use helpers from `pkg/to/`:

```go
// Success response
return to.TextResult(data)

// Error response
return to.ErrorResult(fmt.Errorf("something went wrong: %v", err))
```

### Shared Parameter Descriptions

Reuse descriptions from `pkg/params/` for consistency:

```go
mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner))
mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo))
mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1))
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2` | Forgejo API client |
| `github.com/mark3labs/mcp-go` | MCP protocol implementation |
| `github.com/spf13/cobra` | CLI framework |

## Testing

Run with debug mode to troubleshoot issues:

```bash
FORGEJO_DEBUG=true ./forgejo-mcp --transport stdio --url <url> --token <token>
```

## Blocked Features

Some planned features are blocked on upstream API or SDK support:

| Feature | Status | Details |
|---------|--------|---------|
| Wiki support | Blocked | Waiting for forgejo-sdk wiki API |
| Projects/Kanban | Blocked | Requires Gitea 1.26.0 API |

See `docs/plans/` for detailed status:
- `wiki-support.md` - Wiki API implementation plan
- `projects-support.md` - Projects/Kanban implementation plan

## Contributing

1. Fork the repository on Codeberg
2. Create a feature branch
3. Make your changes following the patterns above
4. Test locally with both stdio and SSE modes
5. Submit a pull request

### Code Style

- Follow standard Go conventions
- Use meaningful variable names
- Add tool descriptions that clearly explain what each tool does
- Reuse parameter descriptions from `pkg/params/` where applicable
