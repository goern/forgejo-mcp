[![MseeP.ai Security Assessment Badge](https://mseep.net/pr/goern-forgejo-mcp-badge.png)](https://mseep.ai/app/goern-forgejo-mcp)

# Forgejo MCP Server

Connect your AI assistant to Forgejo repositories. Manage issues, pull requests, files, and more through natural language.

## What It Does

Forgejo MCP Server is an integration plugin that connects Forgejo with [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) systems. Once configured, you can interact with your Forgejo repositories through any MCP-compatible AI assistant like Claude, Cursor, or VS Code extensions.

**Example commands you can use:**
- "List all my repositories"
- "Create an issue titled 'Bug in login page'"
- "Show me open pull requests in my-org/my-repo"
- "Get the contents of README.md from the main branch"
- "Show me the latest Actions workflow runs in goern/forgejo-mcp"

## Quick Start

### 1. Install

**Option A: Using Go (Recommended)**

```bash
git clone https://codeberg.org/goern/forgejo-mcp.git
cd forgejo-mcp
go install .
```

Ensure `$GOPATH/bin` (typically `~/go/bin`) is in your PATH.

> **Note:** `go install codeberg.org/goern/forgejo-mcp/v2@latest` does not work currently. See [Known Issues](#known-issues).

**Option B: Download Binary**

Download the latest release from the [releases page](https://codeberg.org/goern/forgejo-mcp/releases).

For Arch Linux, use your favorite AUR helper:

```bash
yay -S forgejo-mcp      # builds from source
yay -S forgejo-mcp-bin  # uses pre-built binary
```

### 2. Get Your Access Token

1. Log into your Forgejo instance
2. Go to **Settings** → **Applications** → **Access Tokens**
3. Create a new token with the permissions you need (repo, issue, etc.)

### 3. Configure Your AI Assistant

Add this to your MCP configuration file:

**For stdio mode** (most common):

```json
{
  "mcpServers": {
    "forgejo": {
      "command": "forgejo-mcp",
      "args": [
        "--transport", "stdio",
        "--url", "https://your-forgejo-instance.org"
      ],
      "env": {
        "FORGEJO_ACCESS_TOKEN": "<your personal access token>"
      }
    }
  }
}
```

**For SSE mode** (HTTP-based):

```json
{
  "mcpServers": {
    "forgejo": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

When using SSE mode, start the server first:

```bash
forgejo-mcp --transport sse --url https://your-forgejo-instance.org --token <your-token>
```

### 4. Start Using It

Open your MCP-compatible AI assistant and try:

```
List all my repositories
```

## Available Tools

| Tool | Description |
|------|-------------|
| **User** | |
| `get_my_user_info` | Get information about the authenticated user |
| `search_users` | Search for users |
| **Repositories** | |
| `list_my_repos` | List all repositories you own |
| `create_repo` | Create a new repository |
| `fork_repo` | Fork a repository |
| `search_repos` | Search for repositories |
| **Branches** | |
| `list_branches` | List all branches in a repository |
| `create_branch` | Create a new branch |
| `delete_branch` | Delete a branch |
| **Files** | |
| `get_file_content` | Get the content of a file |
| `create_file` | Create a new file |
| `update_file` | Update an existing file |
| `delete_file` | Delete a file |
| **Commits** | |
| `list_repo_commits` | List commits in a repository |
| **Issues** | |
| `list_repo_issues` | List issues in a repository |
| `get_issue_by_index` | Get a specific issue |
| `create_issue` | Create a new issue |
| `add_issue_labels` | Add labels to an issue |
| `update_issue` | Update an existing issue |
| `issue_state_change` | Open or close an issue |
| **Comments** | |
| `list_issue_comments` | List comments on an issue or PR |
| `get_issue_comment` | Get a specific comment |
| `create_issue_comment` | Add a comment to an issue or PR |
| `edit_issue_comment` | Edit a comment |
| `delete_issue_comment` | Delete a comment |
| **Pull Requests** | |
| `list_repo_pull_requests` | List pull requests in a repository |
| `get_pull_request_by_index` | Get a specific pull request |
| `create_pull_request` | Create a new pull request |
| `update_pull_request` | Update an existing pull request |
| `list_pull_reviews` | List reviews for a pull request |
| `get_pull_review` | Get a specific pull request review |
| `list_pull_review_comments` | List comments on a pull request review |
| **Actions** | |
| `dispatch_workflow` | Trigger a workflow run via `workflow_dispatch` event |
| `list_workflow_runs` | List workflow runs with optional filtering by status, event, or SHA |
| `get_workflow_run` | Get details of a specific workflow run by ID |
| **Organizations** | |
| `search_org_teams` | Search for teams in an organization |
| **Server** | |
| `get_forgejo_mcp_server_version` | Get the MCP server version |

## CLI Mode

You can invoke any tool directly from the command line without running an MCP server. This is useful for shell scripts, CI/CD pipelines, and Claude Code skills.

```bash
# List all available tools (grouped by domain)
forgejo-mcp --cli list

# Invoke a tool with JSON arguments
forgejo-mcp --cli get_issue_by_index --args '{"owner":"goern","repo":"forgejo-mcp","index":1}'

# Pipe JSON arguments via stdin
echo '{"owner":"goern","repo":"forgejo-mcp"}' | forgejo-mcp --cli list_repo_issues

# List recent workflow runs (text output)
forgejo-mcp --cli list_workflow_runs \
  --args '{"owner":"goern","repo":"forgejo-mcp"}' \
  --output=text

# List only failed runs
forgejo-mcp --cli list_workflow_runs \
  --args '{"owner":"goern","repo":"forgejo-mcp","status":"failure"}' \
  --output=text

# Show a tool's parameters
forgejo-mcp --cli create_issue --help

# Control output format (json or text)
forgejo-mcp --cli list --output=json
forgejo-mcp --cli get_my_user_info --args '{}' --output=text
```

CLI mode requires the same `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` configuration as MCP server mode. Tool results are written as JSON to stdout by default; errors go to stderr with a non-zero exit code.

## Configuration Options

You can configure the server using command-line arguments or environment variables:

| CLI Argument | Environment Variable | Description |
|--------------|---------------------|-------------|
| `--url` | `FORGEJO_URL` | Your Forgejo instance URL |
| `--token` | `FORGEJO_ACCESS_TOKEN` | Your personal access token |
| `--debug` | `FORGEJO_DEBUG` | Enable debug mode |
| `--transport` | - | Transport mode: `stdio` or `sse` |
| `--sse-port` | - | Port for SSE mode (default: 8080) |
| `--cli` | - | Enter CLI mode for direct tool invocation |

Command-line arguments take priority over environment variables.

## Troubleshooting

**Enable debug mode** to see detailed logs:

```bash
forgejo-mcp --transport sse --url <url> --token <token> --debug
```

Or set the environment variable:

```bash
export FORGEJO_DEBUG=true
```

## Getting Help

- [Report issues](https://codeberg.org/goern/forgejo-mcp/issues)
- [View source code](https://codeberg.org/goern/forgejo-mcp)

## For Developers

See [DEVELOPER.md](DEVELOPER.md) for build instructions, architecture overview, and contribution guidelines.

## Known Issues

- **`go install ...@latest` fails** — The `go.mod` contains a `replace` directive (for a forked Forgejo SDK), which prevents remote `go install`. Use the clone-and-build workflow shown in [Quick Start](#quick-start) instead. Tracked in [#67](https://codeberg.org/goern/forgejo-mcp/issues/67).

## License

This project is open source. See the repository for license details.
