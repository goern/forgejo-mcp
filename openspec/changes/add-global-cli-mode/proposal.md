## Why

forgejo-mcp has 46 MCP tools covering issues, PRs, repos, search, and more — but they're only accessible through the MCP protocol (stdio/SSE). Adding a `--cli` mode lets the binary invoke any tool directly from the command line, enabling use as a Claude Code skill, shell scripting, and CI/CD pipelines. This partially implements the vision of [forgejo-cli](https://codeberg.org/forgejo-contrib/forgejo-cli) (`fj`) by reusing the existing tool handlers as CLI commands, following the [pattern established by google_workspace_mcp](https://github.com/taylorwilsdon/google_workspace_mcp#cli-mode).

## What Changes

- Add a `--cli` global flag that bypasses MCP server startup and enters direct tool invocation mode
- `forgejo-mcp --cli list [--json]` — enumerate all registered tools with descriptions
- `forgejo-mcp --cli <tool-name> --args '{...}'` — invoke a tool with JSON arguments, output result to stdout
- `forgejo-mcp --cli <tool-name> --help` — show tool parameters and descriptions
- Support piped JSON via stdin as alternative to `--args`
- All CLI output is JSON to stdout; errors to stderr with non-zero exit code
- The existing MCP server (stdio/SSE) behavior is unchanged when `--cli` is not used

## Capabilities

### New Capabilities

- `cli-mode`: Global `--cli` flag that routes to direct tool invocation instead of MCP server startup. Covers tool listing, tool help, tool execution with JSON args (inline or stdin), and JSON output formatting.

### Modified Capabilities

_(none — MCP server behavior is unchanged)_

## Impact

- **cmd/cmd.go**: Flag parsing needs restructuring — `--cli` must be detected early (before MCP server init), similar to the existing `version` subcommand escape hatch
- **operation/**: The tool registry (`MCPServer.AddTool`) is currently opaque; CLI mode needs a way to enumerate tools and invoke handlers by name. This likely means building a parallel registry or using mcp-go's introspection capabilities.
- **New code**: A CLI executor (~1 new file) that resolves tool name → handler, deserializes JSON args into `mcp.CallToolRequest`, calls the handler, and formats the `*mcp.CallToolResult` as JSON output
- **Dependencies**: No new dependencies expected — mcp-go and the standard library should suffice
- **Binary size/behavior**: Negligible impact; the `--cli` path is a thin dispatch layer over existing handlers
