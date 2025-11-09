# Forgejo MCP Server

**Forgejo MCP Server** is an integration plugin designed to connect Forgejo with Model Context Protocol (MCP) systems. This allows for seamless command execution and repository management through an MCP-compatible chat interface.

## 🚧Installation


### 🔧Build from Source

You can download the source code by cloning the repository using Git:

```bash
git clone https://codeberg.org/goern/forgejo-mcp.git
```

Before building, make sure you have the following installed:

- make
- Golang (Go 1.24 or later recommended)

Then run:

```bash
make build
```

### 📁Add to PATH

After building, copy the binary forgejo-mcp to a directory included in your system's PATH. For example:

```bash
cp forgejo-mcp /usr/local/bin/
```

## 🚀Usage

This example is for Cursor, you can also use plugins in VSCode.
To configure the MCP server for Forgejo, add the following to your MCP configuration file:

- **stdio mode**

```json
{
  "mcpServers": {
    "forgejo": {
      "command": "forgejo-mcp",
      "args": [
        "--transport", "stdio",
        "--url", "https://forgejo.example.org"
        // "--token", "<your personal access token>"
      ],
      "env": {
        // "FORGEJO_URL": "https://forgejo.example.org",
        "FORGEJO_ACCESS_TOKEN": "<your personal access token>"
      }
    }
  }
}
```

- **sse mode**

```json
{
  "mcpServers": {
    "forgejo": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

> [!NOTE]
> You can provide your Forgejo URL and access token either as command-line arguments or environment variables.
> Command-line arguments have the highest priority. Environment variables have been updated:
> - Use `FORGEJO_URL` instead of the deprecated `GITEA_HOST`
> - Use `FORGEJO_ACCESS_TOKEN` instead of the deprecated `GITEA_ACCESS_TOKEN`
> - Use `FORGEJO_DEBUG` instead of the deprecated `GITEA_DEBUG`
>
> The old `GITEA_*` variables are still supported for backward compatibility but will show deprecation warnings.

Once everything is set up, try typing the following in your MCP-compatible chatbox:

```text
list all my repositories
```

## ✅Available Tools

The Forgejo MCP Server supports the following tools:

|  Tool  |  Scope  | Description  |
|:------:|:-------:|:------------:|
|get_my_user_info|User|Get the information of the authenticated user|
|create_repo|Repository|Create a new repository|
|fork_repo|Repository|Fork a repository|
|list_my_repos|Repository|List all repositories owned by the authenticated user|
|create_branch|Branch|Create a new branch|
|delete_branch|Branch|Delete a branch|
|list_branches|Branch|List all branches in a repository|
|list_repo_commits|Commit|List all commits in a repository|
|get_file_content|File|Get the content and metadata of a file|
|create_file|File|Create a new file|
|update_file|File|Update an existing file|
|delete_file|File|Delete a file|
|get_issue_by_index|Issue|Get an issue by its index|
|list_repo_issues|Issue|List all issues in a repository|
|create_issue|Issue|Create a new issue|
|create_issue_comment|Issue|Create a comment on an issue|
|list_issue_comments|Issue|List all comments on an issue or pull request|
|get_issue_comment|Issue|Get a specific comment by its ID|
|edit_issue_comment|Issue|Edit an existing comment|
|delete_issue_comment|Issue|Delete a comment|
|get_pull_request_by_index|Pull Request|Get a pull request by its index|
|list_repo_pull_requests|Pull Request|List all pull requests in a repository|
|create_pull_request|Pull Request|Create a new pull request|
|update_pull_request|Pull Request|Update an existing pull request|
|search_users|User|Search for users|
|search_org_teams|Organization|Search for teams in an organization|
|search_repos|Repository|Search for repositories|
|get_forgejo_mcp_server_version|Server|Get the version of the Forgejo MCP Server|

## 🐛Debugging

To enable debug mode, add the `-d` flag when running the Forgejo MCP Server with sse mode:

```sh
./forgejo-mcp --transport sse --url <forgejo-url> [--sse-port 8080] --token <your personal access token> --debug
```

Enjoy exploring and managing your Forgejo repositories via chat!
