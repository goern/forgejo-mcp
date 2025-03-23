# Gitea MCP Server

**Gitea MCP Server** is an integration plugin designed to connect Gitea with Model Context Protocol (MCP) systems. This allows for seamless command execution and repository management through an MCP-compatible chat interface.

## 🚧 Installation

There is currently no official release. You will need to build the Gitea MCP Server from source.

### 🔧 Build from Source

You can download the source code by cloning the repository using Git:

```bash
git clone https://gitea.com/gitea/gitea-mcp.git
```

Before building, make sure you have the following installed:

- make
- Golang (Go 1.24 or later recommended)

Then run:

```bash
make build
```

### 🛠️ Add to PATH

After building, copy the binary gitea-mcp to a directory included in your system's PATH. For example:

```bash
cp gitea-mcp /usr/local/bin/
```

## 🚀 Usage

This example is for Cursor, you can also use plugins in VSCode.
To configure the MCP server for Gitea, add the following to your MCP configuration file:

- **stdio mode**
```json
{
  "mcpServers": {
    "gitea": {
      "command": "gitea-mcp",
      "args": [
        "-t", "stdio",
        "--host", "https://gitea.com"
        // "--token", "<your personal access token>"
      ],
      "env": {
        // "GITEA_HOST": "https://gitea.com",
        "GITEA_ACCESS_TOKEN": "<your personal access token>"
      }
    }
  }
}
```

- **sse mode**
```json
{
  "mcpServers": {
    "gitea": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

> [!NOTE]
> You can provide your Gitea host and access token either as command-line arguments or environment variables.
> Command-line arguments have the highest priority

Once everything is set up, try typing the following in your MCP-compatible chatbox:

```text
list all my repositories
```

## 🐛 Debugging

To enable debug mode, add the `-d` flag when running the Gitea MCP Server with sse mode:
```sh
./gitea-mcp -t sse --token <your personal access token> -d
```

Enjoy exploring and managing your Gitea repositories via chat!
