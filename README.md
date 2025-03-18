# Gitea MCP Server

## Usage

**MCP Server Config**
```json
{
  "mcpServers": {
    "gitea": {
      "command": "gitea-mcp",
      "args": {
        "-t": "stdio",
        "--host": "https://gitea.com",
        "--token": "<your personal access token>"
      },
      "env": {
        "GITEA_HOST": "https://gitea.com",
        "GITEA_ACCESS_TOKEN": "<your personal access token>"
      }
    }
  }
}
```

- Cursor config
```json
{
  "mcpServers": {
    "gitea": {
      "command": "gitea-mcp",
      "args": [
        "-t": "stdio",
        "--host": "https://gitea.com",
        "--token": "<your personal access token>"
      ]
    }
  }
}
```