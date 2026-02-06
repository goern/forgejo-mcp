## ADDED Requirements

### Requirement: CLI mode flag bypasses MCP server
The binary SHALL accept a `--cli` global flag that routes execution to direct tool invocation instead of starting the MCP server. When `--cli` is present, the binary MUST NOT start a stdio or SSE transport.

#### Scenario: CLI mode entry
- **WHEN** the user runs `forgejo-mcp --cli list`
- **THEN** the binary enters CLI mode and does not start an MCP server

#### Scenario: Normal mode unaffected
- **WHEN** the user runs `forgejo-mcp` without `--cli`
- **THEN** the binary starts the MCP server as before (stdio or SSE)

### Requirement: CLI list enumerates tools grouped by domain
The `--cli list` subcommand SHALL print all registered tools to stdout, grouped by domain (user, repo, issue, pull, search, version). Each entry MUST show the tool name and its description.

#### Scenario: List tools in human-readable format
- **WHEN** the user runs `forgejo-mcp --cli list`
- **THEN** stdout contains all registered tools grouped under domain headings, with each tool showing its name and description

#### Scenario: List tools as JSON
- **WHEN** the user runs `forgejo-mcp --cli list --json`
- **THEN** stdout contains a JSON array of tool objects, each with `name`, `description`, and `domain` fields

### Requirement: CLI tool execution with inline args
The `--cli <tool-name> --args '<json>'` form SHALL invoke the named tool handler with the provided JSON arguments and print the result to stdout.

#### Scenario: Invoke tool with inline JSON args
- **WHEN** the user runs `forgejo-mcp --cli get_my_user_info --args '{}'`
- **THEN** the tool handler is called with the parsed JSON arguments and the result is printed as JSON to stdout

#### Scenario: Unknown tool name
- **WHEN** the user runs `forgejo-mcp --cli nonexistent_tool --args '{}'`
- **THEN** the binary prints an error to stderr naming the unknown tool, exits with non-zero code

#### Scenario: Invalid JSON in --args
- **WHEN** the user runs `forgejo-mcp --cli get_my_user_info --args 'not json'`
- **THEN** the binary prints a JSON parse error to stderr and exits with non-zero code

### Requirement: CLI tool execution with stdin pipe
The binary SHALL accept JSON arguments from stdin when `--args` is not provided and stdin is not a terminal.

#### Scenario: Pipe JSON via stdin
- **WHEN** the user runs `echo '{"owner":"goern","repo":"forgejo-mcp"}' | forgejo-mcp --cli list_repo_issues`
- **THEN** the tool handler is called with the piped JSON and the result is printed to stdout

#### Scenario: Inline args take precedence over stdin
- **WHEN** the user provides both `--args '{...}'` and pipes JSON to stdin
- **THEN** the `--args` value is used and stdin is ignored

### Requirement: CLI tool help
The `--cli <tool-name> --help` form SHALL print the tool's parameter schema in a human-readable format.

#### Scenario: Show tool parameters
- **WHEN** the user runs `forgejo-mcp --cli create_issue --help`
- **THEN** stdout lists each parameter with its name, type, required/optional status, and description

### Requirement: Global output format flag
The CLI SHALL accept `--output=json|text` to control output formatting. The default SHALL be `json` for tool execution and `text` for `list`. When `--output=text` is used with tool execution, the content text fields SHALL be printed as plain text (one per line). When `--output=json` is used with `list`, a JSON array of tool objects SHALL be printed.

#### Scenario: Explicit JSON output for list
- **WHEN** the user runs `forgejo-mcp --cli list --output=json`
- **THEN** stdout contains a JSON array of tool objects

#### Scenario: Explicit text output for tool execution
- **WHEN** the user runs `forgejo-mcp --cli get_my_user_info --args '{}' --output=text`
- **THEN** stdout contains the result content as plain text

#### Scenario: Default output for tool execution is JSON
- **WHEN** the user runs `forgejo-mcp --cli get_my_user_info --args '{}'` without `--output`
- **THEN** the result is printed as JSON to stdout

#### Scenario: Default output for list is text
- **WHEN** the user runs `forgejo-mcp --cli list` without `--output`
- **THEN** the tools are printed as a human-readable grouped table

### Requirement: CLI output format
Tool execution results SHALL be written to stdout. Errors SHALL be written as text to stderr with a non-zero exit code.

#### Scenario: Successful tool execution output
- **WHEN** a tool handler returns a successful `CallToolResult`
- **THEN** the result content is written to stdout and the exit code is 0

#### Scenario: Tool handler error
- **WHEN** a tool handler returns an error
- **THEN** the error message is printed to stderr and the exit code is non-zero

#### Scenario: Tool result with IsError flag
- **WHEN** a tool handler returns a `CallToolResult` with `IsError: true`
- **THEN** the result content is printed to stderr and the exit code is non-zero

### Requirement: CLI mode requires Forgejo connection config
CLI mode SHALL require the same Forgejo URL and access token configuration as MCP server mode. The `--url` flag and `FORGEJO_URL` / `FORGEJO_ACCESS_TOKEN` environment variables MUST work identically.

#### Scenario: Missing URL in CLI mode
- **WHEN** the user runs `forgejo-mcp --cli list` without providing a URL
- **THEN** the binary exits with an error indicating the URL is required

#### Scenario: Environment variable configuration
- **WHEN** `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` are set
- **THEN** CLI mode uses those values for the Forgejo connection
