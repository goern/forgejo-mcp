## Why

Codeberg issue [#137](https://codeberg.org/goern/forgejo-mcp/issues/137) reports that `forgejo-mcp` only supports a single global Forgejo token, set at startup via `--token`. Operators running the server as shared infrastructure (multiple agents, multiple humans, dynamically spawned clients) have two unhappy options today:

1. **Sidecar per identity** — one `forgejo-mcp` process per token. Operationally expensive, defeats the point of HTTP/SSE transports.
2. **One overly-permissive token + external ACL** — push access control into a reverse proxy. Pushes a security-critical concern outside the tool that knows the API best.

The HTTP/SSE transports already carry an `Authorization` header per request. We should honor it: extract the token, run that request under that identity, fall back to the global token when no header is supplied. `stdio` mode is single-process by definition and keeps using the global token.

## What Changes

- **Per-request token extraction.** The HTTP and SSE transports SHALL read `Authorization: token <X>` or `Authorization: Bearer <X>` from incoming requests and inject the token into the request `context.Context`.
- **Context-aware client factory.** `pkg/forgejo.Client(ctx)` SHALL build a fresh, request-scoped Forgejo SDK client when a context token is present, and SHALL fall back to the process-wide singleton when none is.
- **Handler propagation.** Every tool handler under `operation/**` SHALL pass its `context.Context` to `forgejo.Client(ctx)`. Any handler that uses the legacy token-less factory bypasses per-request auth and is a bug.
- **Raw HTTP path.** `pkg/forgejo/rawhttp.go` (attachment upload/download) SHALL read the request token from context before falling back to the global `--token` flag.
- **No silent fallback on identity.** When a request supplies a context token but the ephemeral SDK client fails to construct (e.g. the SDK's version-probe call errors), the tool SHALL return an error rather than silently demoting the request to the global token. Silent fallback is a privilege-escalation vector.
- **Backward compatibility.** Stdio mode keeps working with `--token` exactly as today. HTTP/SSE servers started with a global token also keep working for clients that don't send `Authorization`.

## Capabilities

### New Capabilities

- `stateless-http-auth`: per-request, context-scoped authentication for the HTTP and SSE transports. Token resolution, fallback semantics, and request-isolation guarantees live here.

### Modified Capabilities

None. Existing tool capabilities (`merge-pull-request`, `release-management`, etc.) gain no surface change; the auth resolution happens inside the shared `forgejo.Client(ctx)` factory below them.

## Impact

- **Code**: `pkg/forgejo/forgejo.go` (context-aware `Client(ctx)`, ephemeral client builder), `pkg/forgejo/rawhttp.go` (context-first token resolution), `operation/operation.go` (HTTP and SSE `WithHTTPContextFunc` / `WithSSEContextFunc` token extractors), every handler file under `operation/**` (signature plumbing — `ctx` passed through to `forgejo.Client(ctx)`).
- **API surface**: zero new MCP tool parameters. Auth is a transport-layer concern. The protocol surface to clients is the HTTP `Authorization` header — standard.
- **Docs**: new README section "Multi-tenant HTTP mode" with a copy-paste runnable demo (showboat). The PR #138 thread asks the contributor to land that in the same PR.
- **Tests**: unit test in `pkg/forgejo/forgejo_test.go` proves token extraction and per-request client isolation. Today the test asserts pointer inequality only; the spec requires a stronger test that records inbound `Authorization` headers via `httptest.Server` and confirms each goroutine's request reaches the server with its own token.
- **Out of scope** (deferred): per-request token caching (so repeated calls from the same identity reuse a client), OIDC/JWT bearer schemes, token introspection or refresh, audit logging of per-request identities. All standalone follow-ups; none block this change.
