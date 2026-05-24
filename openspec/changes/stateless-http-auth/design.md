## Context

Before this change, `forgejo-mcp` instantiated a single `forgejo.Client` from the `--token` startup flag and held it in a package-level singleton guarded by `sync.Once`. Every handler reached for the same client. That is fine for `stdio` (one process, one user, one token by construction) and broken for HTTP/SSE (one process, many users, one token shared across all of them).

The HTTP and SSE transports terminate inside `operation/operation.go` (`server.NewStreamableHTTPServer`, `server.NewSSEServer`). Both support a per-request context hook (`WithHTTPContextFunc`, `WithSSEContextFunc`) that runs against the raw `http.Request` before the MCP handler. That hook is the natural place to extract `Authorization` and inject the token into the request context.

The constraint set:

- No SDK upgrade. The Forgejo SDK already supports a `Token` option on `NewClient`; we just call it per-request.
- Backward compatible. Stdio path must not change behavior. HTTP/SSE clients that don't send `Authorization` must keep working when a global token is configured.
- No new dependencies. `context.Context` is already threaded through every handler.
- No cross-request state. A token from request A must never leak into request B's client.

Codeberg PR [#138](https://codeberg.org/goern/forgejo-mcp/pulls/138) is the implementation in review. This openspec retrofits the spec onto the design that landed there, captures the open blockers from review as tasks, and gives future readers a single document explaining the why.

## Goals / Non-Goals

**Goals:**

- A single `forgejo-mcp` HTTP/SSE process can serve N distinct identities, each with its own Forgejo token.
- Per-request identity resolution: token in → ephemeral SDK client out, scoped to that request, garbage-collected at request end.
- Stdio mode is unchanged. `--token` keeps being the single source of auth.
- The handler refactor is mechanical: every existing handler passes its `ctx` to the client factory, no other logic changes.
- Failure modes are explicit: if a context token is supplied and the ephemeral client cannot be built, the request fails — it does NOT silently use the global token.

**Non-Goals:**

- Caching ephemeral clients across requests (every request pays the SDK's version-probe cost). Worth measuring before optimizing.
- Token refresh, rotation, or revocation tracking.
- OIDC / JWT bearer schemes beyond accepting `Bearer <X>` as a transport prefix.
- Audit logging of which identity made which call.
- Authorization (which token can call which tool). Forgejo enforces this server-side; this change is about authentication only.
- A `--no-token` startup mode that rejects requests without an `Authorization` header. Useful for strict multi-tenant deployments but a follow-up.

## Decisions

### D1. Token lives in `context.Context`, keyed by a private type

The token is per-request transient state. `context.Context` is built for exactly this. Inject via `forgejo.WithToken(ctx, token)`, read via an unexported key type so external packages cannot stomp on it or read it through reflection on string keys.

Alternatives considered:

- A package-level `sync.Map` keyed by request ID. Rejected: requires propagating a request ID separate from the context, and `sync.Map` cleanup on request end is error-prone.
- A wrapper struct passed explicitly through every handler signature. Rejected: would require a codebase-wide signature change beyond the already-mechanical `ctx` plumbing.

### D2. `Client(ctx)` is the single entry point

One factory, one rule: if `ctx` carries a token, return a fresh client built from that token; else return the singleton. Every handler calls `forgejo.Client(ctx)`. No handler reaches for `forgejo.client` directly, no handler builds its own SDK client.

Consequences:

- The package-level singleton remains for stdio (and for HTTP requests that omit the header against a server started with a global token). It is initialized via `sync.Once` exactly as before.
- The per-request client is constructed by `forgejo.NewClient(url, forgejo.SetToken(token))`. The SDK performs a version-probe HTTP call inside `NewClient`. See D3.

### D3. Failed ephemeral construction MUST NOT fall through to the singleton

When a context token is present and `forgejo.NewClient(...)` returns an error, the natural reflex is "fall back to the global client so the request doesn't break." That is wrong. The request explicitly named an identity; demoting it to the global identity changes who acts on whose behalf — silently. If the global token has higher permissions than the request token, that is a privilege escalation. Even if it does not, the audit trail lies.

Rule: when ctx-token is supplied and ephemeral construction fails, return the error to the caller. Let the request fail loudly. The handler will surface it as a tool error, the client will see the failure, the operator will see the log entry. Loud is safe; silent is dangerous.

This is the open blocker on PR #138 today.

### D4. Accept `token <X>` and `Bearer <X>`, case-insensitively

The Forgejo API itself accepts both `token <X>` and `Bearer <X>` (and historically `Basic` for user/pass). For consistency with what existing Forgejo clients do, the transport extractor SHALL accept both. RFC 7235 requires scheme matching to be case-insensitive (`bearer` == `Bearer`).

No bare-token branch. A token with no scheme prefix is rejected — the header must be one of the two named schemes. This is the second open blocker on PR #138 today (the bare-token branch was added without spec authorization).

### D5. Stdio path is unchanged

In `operation/operation.go`, the stdio server is constructed without a context func. Stdio requests therefore carry no context token, `Client(ctx)` returns the singleton, and the singleton was initialized from `--token` as before. Zero changes to stdio behavior; zero changes to existing stdio tests.

### D6. Test must record real `Authorization` headers, not just compare pointers

`pkg/forgejo/forgejo_test.go` today asserts `c1 != c2` (the two ephemeral clients have distinct addresses). That proves nothing about isolation — Go would happily allocate two distinct structs that both carry the same global token. The SDK's `accessToken` field is unexported, so the test cannot inspect it directly.

The real test stands up an `httptest.Server`, drives two concurrent goroutines that each call `Client(ctx)` with a distinct token, makes an outbound SDK call on each client, and the `httptest.Server` records the inbound `Authorization` header. Each goroutine's request must arrive at the server with its own token, not the other's, not the global.

This test would also catch a regression of D3 — if the ephemeral client failed silently and fell back to the singleton, the recorder would see the global token instead of the request token, and the test would fail. Defense in depth via test design.

This is the third open blocker on PR #138 today.

## Risks / Trade-offs

- **[SDK version-probe per request]** Every per-request client constructs by calling `loadServerVersion()` against Forgejo. That is an extra HTTP round-trip per MCP request. Forgejo is the same host the request is going to anyway; the cost is small but real. Mitigation today: none. Future: cache ephemeral clients by `(url, token-hash)` with a short TTL (out of scope per Non-Goals).
- **[Token in context lifetime]** A goroutine that captures the request `ctx` and outlives the request keeps the token in memory longer than expected. Standard Go context discipline applies. No specific mitigation; we don't spawn goroutines with the request context in this code.
- **[Logging discipline]** The SDK error from a failed ephemeral construction may include the request URL but must not include the token. Verified by audit of the diff; future SDK upgrades MUST be re-audited. Add a CI test that fails if any package logs a string containing the runtime token? Out of scope for this change.
- **[Misconfigured client silently downgrading]** If a client sends `Authorization: bearer xyz` (lowercase) against a server that only matches `Bearer ` literally, the request silently downgrades to the global identity. D4 fixes this with case-insensitive matching. Without D4 the failure is invisible.

## Migration Plan

Additive only. The change has three deployable shapes:

1. **Stdio user**: zero change. Same `--token` flag, same behavior.
2. **HTTP/SSE user with a global token, no per-request headers**: zero change. The server still uses the global token for all incoming requests.
3. **HTTP/SSE user with per-request `Authorization` headers**: NEW. The server uses the per-request token; the global `--token` becomes the fallback identity (or can be omitted entirely if no client ever skips the header).

Reverting: remove the context-extractor calls in `operation.go`, drop `Client(ctx)`'s ctx-aware branch, the rest of the handler refactor is no-op (passing an unused ctx through). Low-risk rollback.

## Open Questions

- Should the server log a structured event (without the token) every time it accepts a per-request token, for auditability? Probably yes, but format and destination are a separate follow-up. Out of scope here.
- Should there be a startup flag that REQUIRES per-request `Authorization` headers (rejecting requests that omit them)? Useful for strict deployments. Out of scope; file as a follow-up if requested.
