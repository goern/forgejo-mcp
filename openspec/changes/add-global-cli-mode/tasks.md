## 1. Flag parsing and CLI entry point

- [x] 1.1 Add `--cli` detection in `cmd/cmd.go` `init()` — check `os.Args` before `flag.Parse()`, skip MCP transport flag setup when detected
- [x] 1.2 Add CLI branch in `Execute()` — when `--cli` detected, call `RunCLI(version)` instead of `operation.Run()`
- [x] 1.3 Parse CLI-specific flags in `RunCLI()` using a separate `flag.FlagSet`: `--args`, `--output`, `--help`, positional tool name
- [x] 1.4 Reuse existing URL/token parsing — CLI mode still needs `--url`/`FORGEJO_URL` and `--token`/`FORGEJO_ACCESS_TOKEN`

## 2. Tool registry setup for CLI

- [x] 2.1 In `RunCLI()`, construct `MCPServer` and call `operation.RegisterTool()` to populate the registry
- [x] 2.2 Build a domain-grouping map: tool name → domain string (user, repo, issue, pull, search, version) by inspecting each domain's `RegisterTool` or by naming convention

## 3. CLI list command

- [x] 3.1 Implement `cliList()` — text mode: print tools grouped by domain with name and description columns
- [x] 3.2 Implement `cliList()` — JSON mode (`--output=json`): emit JSON array with `name`, `description`, `domain` fields

## 4. CLI tool help

- [x] 4.1 Implement `cliHelp()` — given a tool name, extract `mcp.Tool.InputSchema` and print each parameter's name, type, required/optional, and description

## 5. CLI tool execution

- [x] 5.1 Implement `cliExec()` — resolve tool by name via `MCPServer.GetTool()`, error if not found
- [x] 5.2 Parse JSON args from `--args` flag or stdin (detect pipe via `os.Stdin.Stat()`)
- [x] 5.3 Construct `mcp.CallToolRequest` with parsed args, call `tool.Handler(ctx, req)`
- [x] 5.4 Format output: JSON mode (default) serializes `CallToolResult.Content`; text mode prints content text fields line-by-line
- [x] 5.5 Handle errors: `CallToolResult.IsError` and handler `error` return both write to stderr with exit code 1

## 6. Create `cmd/cli.go`

- [x] 6.1 Create `cmd/cli.go` containing `RunCLI()`, `cliList()`, `cliHelp()`, `cliExec()` and stdin-reading helper
- [x] 6.2 Wire up in `cmd/cmd.go` — minimal changes: early detection + call to `RunCLI()`

## 7. Testing and verification

- [x] 7.1 Manual smoke test: `forgejo-mcp --cli list` shows grouped tools
- [x] 7.2 Manual smoke test: `forgejo-mcp --cli get_my_user_info --args '{}'` returns JSON
- [x] 7.3 Manual smoke test: `echo '{}' | forgejo-mcp --cli get_my_user_info` works via stdin
- [x] 7.4 Manual smoke test: `forgejo-mcp --cli create_issue --help` shows parameter schema
- [x] 7.5 Verify unknown tool and invalid JSON produce stderr errors with non-zero exit
- [x] 7.6 Verify `--output=text` and `--output=json` work for both `list` and tool execution
- [x] 7.7 Verify existing MCP server mode (no `--cli`) is unaffected
