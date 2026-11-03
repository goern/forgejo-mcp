<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Design: forge-credential-rejection

## Context

See `proposal.md` for the motivation, and the delta specs for the required behaviour.

The current state that shapes the approach:

- **Tool errors carry no status.** About 160 tool handlers report failures as `(nil, err)`, mostly via `to.ErrorResult`, which logs `err.Error()` and returns it. Most wrap the SDK error with `%v`, so its type and HTTP status are gone by the time anything outside the handler sees it. mcp-go turns a plain handler error into `-32603`. Recognising a `401` from error text after the fact would mean matching on Forgejo's wording.
- **SDK errors are built from Forgejo's body.** The SDK's `statusCodeToErr` reads the response body and puts Forgejo's `message` into the error. That message echoes the presented token (verified on Forgejo 16.0.5 and Codeberg; every API path answers an unknown token with `401`, including `/api/v1/version`).
- **Where the failure appears depends on the version probe.** The ephemeral SDK client (`pkg/forgejo.Client`) is built per request. When the server version is unknown, `NewClient` probes `/version` with the token, and the rejection appears as "create ephemeral client: …". When the version is known (`SetServerVersion`, done in `resource-server` mode today and proposed for every mode in #710), the rejection appears at the first real API call instead.
- **The raw helpers already know the status.** They build `*HTTPError` with an `ErrUnauthorized` sentinel and a body snippet. #683 redacts the token from that snippet and from the construction error.
- **mcp-go offers the hooks this needs.** `server.WithToolHandlerMiddleware` and `server.WithResourceHandlerMiddleware` wrap every handler. `server.NewHandlerError` lets a handler return a typed JSON-RPC error with its own code. The streamable-HTTP transport writes the status of a plain JSON response only after `HandleMessage` returns; it upgrades to an event stream (status `200` sent early) only when notifications are emitted during the call. `subscriptions/listen` requires a response writer that can stream (`http.Flusher`).

## Goals / Non-Goals

**Goals:**

- Recognise a credential rejection by HTTP status at the one place every Forgejo request passes through, independent of error wording, handler style or where the version probe happens.
- Make the credential unreachable in error text by construction, not by redacting each code path.
- Keep each handler unchanged. The mapping happens around handlers, not in them.

**Non-Goals:**

- Validating credentials at the door (#588 stays as decided).
- Changing how `403`, `404` or other Forgejo errors are reported.
- A `401` on the `sse` transport. Its POST is answered before the result is delivered on the stream, so only the JSON-RPC code applies there.
- Retrying, refreshing or caching credentials.

## Decisions

### D1. Detect at the HTTP transport, with an `http.RoundTripper`

A small `RoundTripper` wraps the transport of every `http.Client` that talks to Forgejo: the ephemeral SDK client (installed with `SetHTTPClient`), the singleton SDK client, and `rawHTTPClient`. On a response with status `401` it:

1. replaces the response body with a fixed JSON body, `{"message":"Forgejo rejected the access token"}`, closing the original one unread beyond a bound;
2. if the request's context carries a rejection marker (D2), marks it.

Replacing the body before the SDK or the raw helpers read it is what makes the "never echoes" requirement hold everywhere: every error string, log line and JSON-RPC message built from the response downstream can only contain the fixed text. It also covers the operator token at startup (the `connection test failed` exit line seen in practice), which no per-path redaction does.

For the request context to reach the `RoundTripper`, the ephemeral SDK client is built with `SetContext(ctx)`. The SDK then creates every request, including the version probe, with `NewRequestWithContext(c.ctx, …)`. The raw helpers already pass `ctx`.

*Alternatives considered:*

- **Match `401` / "access token does not exist" in the error text in a middleware.** Rejected: brittle against Forgejo wording, and it cannot remove the token from the log line `to.ErrorResult` already wrote inside the handler.
- **Redact at every error site (the #683 approach, extended to SDK calls).** Rejected as the primary mechanism: about 160 handlers wrap SDK errors themselves, so complete coverage would be a per-handler audit that new tools would silently fall out of. #683's redaction stays as a second line of defence.
- **Validate the token with one extra Forgejo call per request.** Rejected: that is the door check #588 decided against, and it adds latency to every call.

### D2. A per-request rejection marker in the context

An HTTP middleware in front of the MCP endpoint puts a fresh marker (a small struct holding an atomic flag and the auth mode) into the request context before the transport sees the request. `requestTokenContextFunc` already receives the `*http.Request`, so it copies the marker into the context the handlers get. The SSE transport gets the same marker per posted message.

Only calls that use a **per-request** credential carry a marker: the caller's token in `passthrough`, or the minted JWT in `resource-server` mode. The operator token never marks anything, so a stdio or fallback call can never produce `-32001` or a `401`.

*Alternative:* return a typed error from the `RoundTripper` and let it propagate. Rejected: handlers flatten errors with `%v`, so the type would not survive.

### D3. One middleware maps a marked call to its JSON-RPC error

A tool handler middleware and a resource handler middleware run after the handler. If the marker is set, they discard whatever the handler returned (error or result) and return:

- in `passthrough`: `server.NewHandlerError` with code `-32001` and message `Forgejo rejected the access token`;
- in `resource-server`: a plain error with the fixed server-side message (so mcp-go reports `-32603`), plus a warning-level log line naming the tool. No JWT is logged, since the body was already replaced.

Discarding a successful result is deliberate. A handler that treats a `401` as "nothing found" (as `DoJSONList` does for `404`) must not turn a revoked credential into an empty answer.

`-32001` follows the server-defined range (`-32000` to `-32099`) and lines up with `-32002` (403) and `-32003` (404) that `operation/resource` already uses.

### D4. A `ResponseWriter` wrapper turns `-32001` into a `401` on `http`

The same HTTP middleware wraps the `ResponseWriter`. When the transport calls `WriteHeader(200)` and the marker is set in `passthrough` mode, the wrapper writes `401` with `WWW-Authenticate: Bearer error="invalid_token"` instead. The JSON-RPC body passes through unchanged. If the status was already sent (an upgraded event stream), nothing changes and the client sees `-32001` in the stream.

The wrapper MUST keep `http.Flusher` (and expose `Unwrap` for `http.ResponseController`). Hiding it would make mcp-go refuse `subscriptions/listen` and silently stop upgrading responses to event streams.

The challenge carries no `resource_metadata`. In `passthrough` mode this server is not an OAuth resource server and publishes no metadata. `error="invalid_token"` is what tells OAuth clients to refresh or re-authorize against wherever their token came from.

*Alternative:* buffer the whole response and rewrite it. Rejected: it breaks streaming and the heartbeat.

### D5. `resource-server` mode never challenges on a forge rejection

D3 and D4 branch on the marker's auth mode. In `resource-server` mode, the inbound token was validated at the door, so a forge `401` means the minted JWT was refused. The usual causes are a deleted Authorized Integration, a changed claim rule, or a key Forgejo has not fetched yet. A challenge would make the client re-authorize against the IdP, get a token that validates again, and fail again in a loop. The failure is reported as server-side instead.

## Risks / Trade-offs

- **[A `401` that is not about the credential]** Forgejo answers `401` only for authentication. A reverse proxy in front of Forgejo might send its own `401`. → It is still a refusal of this request's credential from the client's point of view, and the fixed message stays accurate enough. Proxy setups that need a different credential are out of scope.
- **[A call that hits Forgejo several times]** Some tools make more than one request. Once one `401` marks the call, the call fails as a whole. → That is the intended outcome: the credential is the same for every request in the call.
- **[A JSON-RPC batch]** One HTTP request can carry several calls (older protocol versions). If any of them marks the shared marker, the whole response gets `401`. → They all carry the same credential, and the client has to re-authorize either way.
- **[The wrapper hides streaming capability]** → D4 requires `Flusher` and `Unwrap`, and a test asserts that `subscriptions/listen` still streams through the wrapper.
- **[Forgejo's diagnostic detail is lost]** The original rejection text (malformed token, unknown token, task token) is no longer visible. → It is not safe to show, since it is the credential itself. The status and the fixed message suffice for every client action. Operators debugging Forgejo-side configuration still have Forgejo's own logs.

## Migration Plan

No configuration and no flags. Rollback is reverting the change. A client that ignored `200`-with-error before now receives `401` in `passthrough` on `http`. All MCP clients that support OAuth handle `401` by design. Others report a failed request, as before.

This change is merged unarchived. A follow-up PR archives it once it is merged, which syncs the deltas into `openspec/specs/`. Because `forgejo-jwt-issuer` is demo-anchored, that follow-up also adds anchored proofs for the new scenarios to `forgejo-jwt-issuer.demo.md`.

## Open Questions

- Whether to keep #683's redaction helpers after this lands, or fold them in. That is a clean-up question; the behaviour is the same either way.
