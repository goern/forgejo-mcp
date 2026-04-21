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
        "FORGEJO_ACCESS_TOKEN": "<your personal access token>",
        "FORGEJO_USER_AGENT": "forgejo-mcp/1.0.0"
      }
    }
  }
}
```

**For streamable HTTP mode** (recommended for remote/Claude.ai):

```json
{
  "mcpServers": {
    "forgejo": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

When using streamable HTTP mode, start the server first:

```bash
forgejo-mcp --transport http --url https://your-forgejo-instance.org --token <your-token>
```

**For SSE mode** (legacy HTTP-based):

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
| `check_notifications` | Check and list user notifications |
| `get_notification_thread` | Get detailed info on a single notification thread |
| `mark_notification_read` | Mark a single notification thread as read |
| `mark_all_notifications_read` | Acknowledge all notifications |
| `list_repo_notifications` | Filter notifications scoped to a single repository |
| `mark_repo_notifications_read` | Mark all notifications in a specific repo as read |
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
| `add_issue_labels` | Add labels to an issue (requires numeric label IDs) |
| `remove_issue_labels` | Remove labels from an issue (requires numeric label IDs) |
| `update_issue` | Update an existing issue (requires numeric milestone ID) |
| `issue_state_change` | Open or close an issue |
| `list_repo_milestones` | List milestones with their IDs (use with `update_issue`) |
| `list_repo_labels` | List labels with their IDs (use with `add_issue_labels`, `remove_issue_labels`) |
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
| **Time Tracking** | |
| `list_issue_tracked_times` | List tracked time entries on an issue or PR |
| `list_repo_tracked_times` | List tracked time entries across a repository |
| `list_my_tracked_times` | List your own tracked time entries |
| `add_issue_time` | Log time against an issue or PR (accepts seconds or duration like `15m`) |
| `reset_issue_time` | Delete ALL tracked time entries on an issue or PR (destructive) |
| `delete_issue_time_entry` | Delete a single tracked time entry by ID |
| `start_issue_stopwatch` | Start a stopwatch on an issue or PR |
| `stop_issue_stopwatch` | Stop a running stopwatch and record the elapsed time |
| `cancel_issue_stopwatch` | Cancel a running stopwatch without recording |
| `list_my_stopwatches` | List currently running stopwatches |
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
| `--transport` | - | Transport mode: `stdio`, `sse`, or `http` |
| `--sse-port` | - | Port for SSE mode (default: 8080) |
| `--http-port` | - | Port for streamable HTTP mode (default: 8080) |
| `--cli` | - | Enter CLI mode for direct tool invocation |
| `--user-agent` | `FORGEJO_USER_AGENT` | HTTP User-Agent header (default: `forgejo-mcp/<version>`) |

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

**Custom User-Agent**: If your Forgejo instance or proxy blocks the default `go-http-client` user agent, set a custom one:

```bash
# Via environment variable
export FORGEJO_USER_AGENT="forgejo-mcp/1.0.0"

# Or via CLI flag
forgejo-mcp --user-agent "forgejo-mcp/1.0.0" --transport sse --url <url> --token <token>
```

## Getting Help

- [Report issues](https://codeberg.org/goern/forgejo-mcp/issues)
- [View source code](https://codeberg.org/goern/forgejo-mcp)

## For Developers

See [DEVELOPER.md](DEVELOPER.md) for build instructions, architecture overview, and contribution guidelines.

## Known Issues

- **`go install ...@latest` fails** — The `go.mod` contains a `replace` directive (for a forked Forgejo SDK), which prevents remote `go install`. Use the clone-and-build workflow shown in [Quick Start](#quick-start) instead. Tracked in [#67](https://codeberg.org/goern/forgejo-mcp/issues/67).

## Contributors

forgejo-mcp is shaped by everyone who files issues, writes code, reviews PRs, and pushes the project forward. Thank you all. 🙏

### Code contributors

| Contributor | Highlights |
|-------------|------------|
| [goern](https://codeberg.org/goern) (Christoph Görn) | Project creator and maintainer |
| Ronmi Ren | Co-creator; SSE/HTTP transport, issue blocking, CI/CD improvements, logo, Glama spec |
| [twstagg](https://codeberg.org/twstagg) (Tristin Stagg) | User agent configuration support (PR #89) |
| [mattdm](https://codeberg.org/mattdm) (Matthew Miller) | Logging improvements, FORGEJO_* migration, README, URL refactor |
| [byteflavour](https://codeberg.org/byteflavour) | `check_notifications` + full notification management API (PR #84, #86); feature requests #80, #85 |
| [jesterret](https://codeberg.org/jesterret) | Pull request reviews and comments support (PR #51) |
| [appleboy](https://codeberg.org/appleboy) | Custom SSE port support, bug fixes |
| [ignasgil](https://codeberg.org/ignasgil) | `remove_issue_labels` tool (PR #96) |
| [dmikushin](https://codeberg.org/dmikushin) (Dmitry Mikushin) | Fix string-encoded number parameter parsing from MCP clients (PR #93) |
| [jiriks74](https://codeberg.org/jiriks74) | mcp-go v0.44.0 dependency update (PR #90) |
| [th](https://codeberg.org/th) (Tomi Haapaniemi) | `update_pull_request` tool |
| [hiifong](https://codeberg.org/hiifong) | Early bug fixes and updates |
| [Lunny Xiao](https://codeberg.org/lunny) | Early contributions |
| [techknowlogick](https://codeberg.org/techknowlogick) | Early contributions |
| [yp05327](https://codeberg.org/yp05327) | Early contributions |
| [mw75](https://codeberg.org/mw75) | Owner/org support for repo creation (PR #18) |
| [Dax Kelson](https://codeberg.org/dkelson) | Issue comment management (PR #34) |
| [Guruprasad Kulkarni](https://codeberg.org/comdotlinux) | Arch Linux AUR installation docs (PR #69) |
| [Mario Wolff](https://codeberg.org/mariowolff) | Contributions |
| [Massimo Fraschetti](https://codeberg.org/fraschetti) | Contributions |

### Community contributors

Issue reporters and discussion participants who shaped the direction of the project:

| Contributor | Contributions |
|-------------|--------------|
| [byteflavour](https://codeberg.org/byteflavour) | Filed #80 (milestone/label discovery), #85 (notification API proposal); active reviewer in discussions |
| [choucavalier](https://codeberg.org/choucavalier) | Filed #82 (fix skill), #70 (macOS arm64 releases), #62 (binary releases & mise support) |
| [MalcolmMielle](https://codeberg.org/MalcolmMielle) | Filed #59 (PR review tools — since implemented) |
| [redbeard](https://codeberg.org/redbeard) | Filed #60 (Actions support — since implemented) |
| [c6sepl6p](https://codeberg.org/c6sepl6p) | Filed #72 (base64 encoding), #54 (merge pull request — since implemented) |
| [malik](https://codeberg.org/malik) | Filed #73 (version flag), #47 (Nix build fix) |
| [a2800276](https://codeberg.org/a2800276) | Filed #74 (OpenAI compatibility) |
| [simenandre](https://codeberg.org/simenandre) | Filed #49 (go install support) |
| [BasdP](https://codeberg.org/BasdP) | Filed #42 (Projects support) |
| [BoBeR182](https://codeberg.org/BoBeR182) | Filed #32 (wiki support) |
| [ignasgil](https://codeberg.org/ignasgil) | Filed #95 (`remove_issue_labels` feature request) |
| [Vokuar](https://codeberg.org/Vokuar) | Filed #99 (streamable HTTP transport support) |
| [janbaer](https://codeberg.org/janbaer) | Filed #98 (reply to review comment) |
| [fraschm98](https://codeberg.org/fraschm98) | Early issue reports |

### Cyborg contributors

This project also received contributions from AI coding agents — submitted as regular PRs, reviewed by humans:

| Agent | Role | Contributions |
|-------|------|---------------|
| [brenner-axiom](https://codeberg.org/brenner-axiom) (b4-dev, B4arena) | AI dev agent | Organization management tools (PR #94); showboat demos (PR #97); `list_repo_milestones`, `list_repo_labels` tools (PR #83); race condition fix (PR #78); contributors docs (PR #87, #88); filed #76; code reviews |
| opencode | AI dev agent | Pull request reviews and comments support (PR #51) |
| b4mad-release-bot | Release automation | Automated changelog and release tagging |
| the #B4mad Renovate bot | Dependency updates | Automated dependency upgrades |

Want to contribute? Open an issue or pull request — all are welcome.


## License

This project is open source. See the repository for license details.
