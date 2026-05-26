# Demo: Multi-tenant HTTP Mode

*2026-05-24T00:00:00Z by Showboat 0.6.1*
<!-- showboat-id: stateless-http-auth-demo-2026 -->

## What this feature does

Enables a single `forgejo-mcp` instance to serve multiple users or agents by extracting the Forgejo API token from the `Authorization` header of each individual MCP request. This makes the server completely stateless regarding authentication when using HTTP or SSE transports.

## Setup

Build the latest version:

```bash
make build
```

## Starting the server

Start the server without a global token (or with a fallback token):

```bash
./forgejo-mcp --transport http --url https://codeberg.org
```

```output
Starting Forgejo MCP Server ...
Starting MCP streamable HTTP server  {"port": 8080}
MCP streamable HTTP server ready for connections  {"port": 8080, "endpoint": "http://localhost:8080"}
```

## Multi-tenant usage with curl

In a second terminal, we can now make requests using different identities.

### 1. Initialize session

First, we need a session ID (standard for Streamable HTTP):

```bash
INIT_RESP=$(curl -s -i -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": { "name": "multi-tenant-demo", "version": "1.0" }
    }
  }')

SESSION_ID=$(echo "$INIT_RESP" | grep -i "mcp-session-id" | awk '{print $2}' | tr -d '\r')
echo "Session ID: $SESSION_ID"
```

### 2. Request as User A

Pass User A's token in the `Authorization` header:

```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -H "Authorization: token <TOKEN_A>" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "get_my_user_info",
      "arguments": {}
    }
  }' | jq '.result.content[0].text | fromjson | .login'
```

```output
"user-a"
```

### 3. Request as User B (same session)

Pass User B's token in the same session:

```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -H "Authorization: token <TOKEN_B>" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get_my_user_info",
      "arguments": {}
    }
  }' | jq '.result.content[0].text | fromjson | .login'
```

```output
"user-b"
```

### 4. Request with invalid token

```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -H "Authorization: token invalid_token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/call",
    "params": {
      "name": "get_my_user_info",
      "arguments": {}
    }
  }'
```

```output
{"jsonrpc":"2.0","id":4,"error":{"code":-32603,"message":"get user info err: token is required"}}
```

## Supported Header Formats

The server supports the following `Authorization` header formats:
- `Authorization: token <token>` (Forgejo/Gitea style)
- `Authorization: Bearer <token>` (Standard OAuth2 style)
- `Authorization: <token>` (Minimalist style)

Note: The schemes (`token` and `Bearer`) are matched case-insensitively (e.g., `bearer`, `Bearer`, `BEARER` all work).
