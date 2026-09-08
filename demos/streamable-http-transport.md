# Demo: Streamable HTTP Transport

*2026-03-28T00:00:00Z by Showboat 0.6.1*
<!-- showboat-id: a3f8b21c-streamable-http-demo-2026 -->

## What this feature does

Adds `--transport http` to start the MCP server using the **streamable HTTP** transport from MCP spec `2025-03-26`. This is the recommended transport for remote MCP servers and is required for compatibility with Claude.ai's custom MCP connector.

The three available transports are now:

| Transport | Flag | Use case |
|-----------|------|----------|
| `stdio` | `--transport stdio` (default) | Local CLI integration (Claude Code, etc.) |
| `sse` | `--transport sse` | Legacy remote server |
| `http` | `--transport http` | **Recommended** remote server (Claude.ai connectors, etc.) |

## Setup

Set `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` environment variables (or use direnv), then build:

```bash
make build
```

## Starting the server with streamable HTTP

### Default port (8080)

```bash
./forgejo-mcp --transport http --url $FORGEJO_URL --token $FORGEJO_ACCESS_TOKEN
```

```output
Starting Forgejo MCP Server ...
Starting MCP streamable HTTP server  {"port": 8080}
MCP server listening on loopback only  {"transport": "http", "address": "127.0.0.1:8080, [::1]:8080", "authentication": "every request must carry its own Authorization header", "reachable_from": "this machine only, unless a proxy forwards to it", "to_expose": "set -host to an address the network can reach, and declare -allowed-hosts"}
```

### Custom port

```bash
./forgejo-mcp --transport http --http-port 9090 --url $FORGEJO_URL --token $FORGEJO_ACCESS_TOKEN
```

```output
Starting Forgejo MCP Server ...
Starting MCP streamable HTTP server  {"port": 9090}
MCP server listening on loopback only  {"transport": "http", "address": "127.0.0.1:9090, [::1]:9090", "authentication": "every request must carry its own Authorization header", "reachable_from": "this machine only, unless a proxy forwards to it", "to_expose": "set -host to an address the network can reach, and declare -allowed-hosts"}
```

## Connecting with an MCP client

### Initialize session

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": { "name": "demo-client", "version": "1.0" }
    }
  }'
```

```output
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{"tools":{"listChanged":true},"logging":{}},"serverInfo":{"name":"Forgejo MCP Server","version":"..."}}}
```

### List available tools

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id-from-init>" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
```

```output
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"get_my_user_info","description":"Get current user info"}, ...]}}
```

### Call a tool

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: <session-id-from-init>" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "search_repos",
      "arguments": { "keyword": "forgejo-mcp" }
    }
  }'
```

```output
{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"goern/forgejo-mcp ..."}]}}
```

## Claude.ai custom connector

With the streamable HTTP transport, you can register forgejo-mcp as a custom MCP connector in Claude.ai:

1. Deploy forgejo-mcp with `--transport http` behind a public HTTPS endpoint.
   The listener binds loopback by default, so a deployment that must accept
   connections from outside its own machine or container needs `--host` set to a
   reachable address and `--allowed-hosts` set to the names clients use. If your
   proxy preserves the original `Host` header, declare that name; if it rewrites
   `Host` to the proxied target, the loopback default already accepts it.
2. Give the connector its own Forgejo token. Every request on this transport must
   carry an `Authorization` header — the server will not fall back to its own
   configured token, and answers `401` to a request without one.
3. In Claude.ai settings, add a custom MCP connector pointing to your endpoint
4. Claude.ai will discover all Forgejo tools automatically

See [Claude.ai MCP connector docs](https://support.claude.com/en/articles/11503834-building-custom-connectors-via-remote-mcp-servers) for setup details.

## Transport comparison

| Feature | stdio | SSE | Streamable HTTP |
|---------|-------|-----|-----------------|
| Local use | Yes | No | No |
| Remote use | No | Yes | Yes |
| Claude Code | Yes | Deprecated | Yes |
| Claude.ai connector | No | No | **Yes** |
| MCP spec status | Stable | Deprecated | **Recommended** |
| Stateful sessions | N/A | Yes | Yes |
