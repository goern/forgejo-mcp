## Context

forgejo-mcp currently supports two transports: stdio and SSE. Both run a full MCP server. The CLI is parsed in `cmd/cmd.go` using Go's `flag` package, with an early escape hatch for the `version` subcommand (detected before `flag.Parse()`). All 46 tools are registered on a `*server.MCPServer` via domain-specific `RegisterTool()` functions in `operation/`.

The mcp-go library (v0.43.2) exposes `MCPServer.ListTools()` and `MCPServer.GetTool(name)` which return `*ServerTool` containing both the tool definition (`mcp.Tool`) and the handler function (`ToolHandlerFunc`). This means we can enumerate and invoke tools programmatically without maintaining a parallel registry.

## Goals / Non-Goals

**Goals:**

- Allow any registered MCP tool to be invoked directly from the command line
- Output structured JSON to stdout for machine consumption (skills, scripts, pipelines)
- Reuse existing tool handlers with zero duplication
- Keep the `--cli` path lightweight — no MCP protocol overhead

**Non-Goals:**

- Interactive/TUI mode (shell, tab completion, prompts)
- Tool aliasing or shorthand commands (e.g., `forgejo-mcp issues` instead of `forgejo-mcp --cli get_issue_by_index`)
- Streaming or long-running tool support
- Authentication beyond existing `--token` / `FORGEJO_ACCESS_TOKEN` mechanism
- Replacing or competing with `fj` (forgejo-cli) — this is a complementary approach reusing MCP tool handlers

## Decisions

### 1. Use `--cli` as a global flag, not a subcommand

**Choice:** `forgejo-mcp --cli <tool> --args '{}'`
**Rejected:** `forgejo-mcp cli <tool> --args '{}'` (subcommand style)

**Rationale:** Matches the google_workspace_mcp pattern. A flag is simpler to detect early in `init()` alongside the existing `version` escape hatch. No need for a subcommand framework.

### 2. Detect `--cli` before `flag.Parse()` using `os.Args` inspection

**Choice:** Check `os.Args` for `--cli` in `cmd/cmd.go`'s `init()`, similar to the existing `version` check. When detected, skip the MCP-server-specific flag setup and parse only CLI-relevant flags.

**Rejected:** Adding `--cli` as a regular flag and branching in `Execute()`. This would still run all the MCP init logic (URL validation, token setup) before we know we're in CLI mode.

**Rationale:** The `version` subcommand already establishes this pattern. CLI mode needs the Forgejo URL and token but does NOT need transport setup. We reuse the existing URL/token parsing but skip transport flags.

### 3. Reuse `MCPServer` as the tool registry — no parallel registry

**Choice:** Build the `MCPServer`, register all tools via `RegisterTool()`, then use `ListTools()` / `GetTool()` to enumerate and invoke.

**Rejected:** Building a separate `map[string]ToolHandlerFunc` registry.

**Rationale:** mcp-go already provides thread-safe, exported methods for tool introspection. A parallel registry means maintaining two registrations per tool. The MCPServer is cheap to construct — we just skip calling `ServeStdio()` / `SSEServer.Start()`.

### 4. New file `cmd/cli.go` for CLI dispatch logic

**Choice:** A single new file `cmd/cli.go` containing:

- `RunCLI(version string)` — entry point called from `Execute()` when `--cli` detected
- `cliList(server)` — enumerate tools, print as table or JSON
- `cliHelp(server, toolName)` — print tool parameters
- `cliExec(server, toolName, argsJSON)` — invoke handler, print result

**Rationale:** Keeps `cmd.go` clean. One file covers all CLI dispatch. Follows the existing pattern where `cmd.go` handles flag parsing and `operation/` handles tool execution.

### 5. JSON args via `--args` flag or stdin pipe

**Choice:** Support both:

- `--args '{"owner":"goern","repo":"forgejo-mcp"}'` — inline JSON
- `echo '{"owner":"goern"}' | forgejo-mcp --cli tool_name` — piped stdin

When both are present, `--args` wins. When neither is present and tool requires arguments, error with usage hint.

**Rationale:** Inline is convenient for one-liners. Stdin pipe is essential for scripting and when args are large or contain special characters.

### 6. Output format

**Choice:**

- Tool results: JSON to stdout (the `CallToolResult.Content` array, serialized)
- `list` output: table by default, JSON with `--json` flag
- Errors: text to stderr, non-zero exit code
- `--help` per tool: human-readable parameter listing to stdout

**Rejected:** Always-JSON output (harder to scan for `list`), always-text output (can't pipe to `jq`).

**Rationale:** JSON for tool results enables `| jq` pipelines and skill consumption. Human-readable table for `list` matches `gh` CLI UX. `--json` flag on `list` supports automation.

## Risks / Trade-offs

**[Risk] Tool handlers assume MCP context** → Some handlers may depend on MCP request context (headers, session). Mitigation: `CallToolRequest` can be constructed manually; context.Background() is sufficient since handlers only use `forgejo.Client()` which reads from package-level config.

**[Risk] Flag parsing complexity** → Adding early `os.Args` inspection creates a second parsing path. Mitigation: Keep it minimal — just detect `--cli` and branch. CLI-specific flags (`--args`, `--json`) are parsed with a separate `flag.FlagSet` inside `RunCLI()`.

**[Trade-off] MCPServer construction overhead** → We build the full MCPServer even in CLI mode just to access the registry. This is negligible (no I/O, no goroutines) but could be eliminated later if needed by extracting tool registration to a standalone registry.

**[Trade-off] No tab completion or aliases** → Users must type exact tool names like `get_issue_by_index`. Mitigation: `--cli list` makes discovery easy. Aliases can be added later without breaking changes.
