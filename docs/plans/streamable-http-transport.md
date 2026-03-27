# Streamable HTTP Transport Support

**Issue**: [#99](https://codeberg.org/goern/forgejo-mcp/issues/99)
**Branch**: `feature/streamable-http-transport`

## Summary

Add `--transport http` to support the MCP spec `2025-03-26` streamable HTTP transport. This is needed for Claude.ai's custom MCP connector and replaces SSE as the recommended transport.

## Current State

- Two transports: `stdio` and `sse`
- Transport selected via `--transport` / `-t` flag (default: `stdio`)
- SSE uses `--sse-port` (default: `8080`) and `server.NewSSEServer`
- `mcp-go v0.44.0` already ships `server.NewStreamableHTTPServer` with the same `Start(addr)` API pattern

## Implementation

The change is straightforward since mcp-go already provides the server-side support.

### 1. `cmd/cmd.go` — Add `--http-port` flag, update help text

```go
var (
    transport string
    // ...
    ssePort   int
    httpPort  int   // NEW
)
```

Register the flag:

```go
fs.IntVar(
    &httpPort,
    "http-port",
    8080,
    "Port for streamable HTTP transport mode",
)
```

Update transport help text from `"stdio or sse"` to `"stdio, sse, or http"`.

Pass `httpPort` to `flagPkg.HTTPPort`.

### 2. `pkg/flag/flag.go` — Add `HTTPPort` variable

```go
var HTTPPort int
```

### 3. `operation/operation.go` — Add `http` case to transport switch

```go
case "http":
    httpServer := server.NewStreamableHTTPServer(mcpServer)
    log.Info("Starting MCP streamable HTTP server",
        log.IntField("port", flag.HTTPPort),
    )
    if err := httpServer.Start(fmt.Sprintf(":%d", flag.HTTPPort)); err != nil {
        log.Error("Failed to start streamable HTTP server",
            log.IntField("port", flag.HTTPPort),
            log.ErrorField(err),
        )
        return fmt.Errorf("failed to start streamable HTTP server: %w", err)
    }
    log.Info("MCP streamable HTTP server shutdown")
```

Update the default error case from `"stdio, sse"` to `"stdio, sse, http"`.

### 4. Update `README.md`

Document the new `--transport http` option and `--http-port` flag.

## Design Decisions

- **Flag name `http` not `streamable-http`**: Short, consistent with `stdio`/`sse`. The MCP spec name is verbose for a CLI flag.
- **Separate `--http-port` from `--sse-port`**: Avoids confusion if users switch transports — each transport has its own port flag. Could share a `--port` flag instead, but keeping them separate is clearer and backward-compatible.
- **No SSE deprecation yet**: SSE still works. A future PR can add a deprecation warning once streamable HTTP is proven stable.
- **No TLS/auth options in this PR**: `mcp-go` supports `WithTLSCert` and other options, but those are separate concerns. Ship the basic transport first, add TLS in a follow-up if needed.

## Open Questions

1. **Should `--sse-port` and `--http-port` share a single `--port` flag?** Simpler, but a breaking change for SSE users relying on `--sse-port`.
2. **Should SSE emit a deprecation warning?** The MCP spec has SSE on a deprecation path, but it's still widely used.

## Test Plan

- [ ] `forgejo-mcp --transport http --http-port 9090 --url ... --token ...` starts and accepts connections
- [ ] MCP client can discover tools via streamable HTTP
- [ ] Claude.ai custom connector can connect to the server
- [ ] `--transport stdio` and `--transport sse` still work as before
- [ ] `--transport invalid` still errors with updated valid options list
