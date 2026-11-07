# stateless-http-auth

## Purpose

Enable per-request authentication for the HTTP and SSE transports by extracting a token from the `Authorization` header and constructing a request-scoped Forgejo client, while preserving the pre-existing global-token (singleton) behavior for the stdio transport. This allows a single server process to serve multiple distinct Forgejo identities concurrently without leaking credentials across requests.

## Requirements

### Requirement: HTTP transport extracts per-request token from Authorization header

What the `Authorization` header means depends on `-auth-mode`.

**In `passthrough` mode**, the default, it carries a Forgejo credential. When the server is started with `--transport http`, every incoming MCP request SHALL be inspected for an `Authorization` header. The header SHALL be parsed for one of two schemes, case-insensitively:

1. `token <X>` — Forgejo's native token scheme.
2. `Bearer <X>` — OAuth2-style bearer transport.

When a recognized scheme is present, the parsed token value SHALL be injected into the request `context.Context` via `forgejo.WithToken(ctx, token)`. The MCP handler invoked for that request receives the augmented context.

When the `Authorization` header is absent, empty, or carries an unrecognized scheme, the request `context.Context` SHALL NOT carry a token, and the request SHALL be refused with `401 Unauthorized` before it reaches an MCP handler — unless the operator has set `--allow-operator-token-fallback`, in which case the request is admitted and downstream code falls back to the global singleton client (see "Token-aware client factory" below).

The transport MUST NOT accept tokens without a scheme prefix; a header value that does not match one of the two named schemes SHALL be treated as if no header were present.

This check establishes only that a credential is present and carries a recognized scheme. It does not validate the credential: a request carrying an invalid token passes it, and the forge refuses that token on the first call that reaches it. A request that never reaches the forge — `initialize`, `tools/list`, or holding an event stream open — is therefore not protected against a caller presenting any well-formed credential.

**In `resource-server` mode**, the header carries an OAuth 2.0 access token addressed to this server, and never a Forgejo credential:

- Only the `Bearer` scheme SHALL be accepted. `token` SHALL be treated as carrying no bearer token.
- The token SHALL be validated as capability `oauth-resource-server` specifies, before the request reaches an MCP handler. The `401` on that endpoint is therefore an authentication result rather than a presence check, and the paragraph above about unvalidated credentials does not apply to it.
- The token SHALL NOT be injected into the request context and SHALL NOT be sent to Forgejo. The context receives the JWT this server mints for the caller instead, as capability `forgejo-jwt-issuer` specifies.
- `--allow-operator-token-fallback` and an operator token are refused at startup in this mode, so no configuration serves a request that carried no valid token.
- The mode runs on `http` only. `sse`, `stdio` and `--cli` refuse to start.
- The protected resource metadata, the OpenID discovery document and the key set SHALL be served to requests carrying no `Authorization` header at all, because a caller needs them before it can obtain a token. Host and Origin validation still applies to them, and every path other than those and the MCP endpoint SHALL receive `404`.

#### Scenario: Request with `token` scheme injects the token

- **WHEN** the server runs in `passthrough` mode
- **AND** an HTTP request arrives with header `Authorization: token abc123`
- **THEN** the system SHALL inject `abc123` into the request context via `forgejo.WithToken`
- **AND** the MCP handler SHALL see that token via `forgejo.Client(ctx)`

#### Scenario: Request with `Bearer` scheme injects the token

- **WHEN** the server runs in `passthrough` mode
- **AND** an HTTP request arrives with header `Authorization: Bearer abc123`
- **THEN** the system SHALL inject `abc123` into the request context

#### Scenario: Scheme matching is case-insensitive

- **WHEN** an HTTP request arrives with header `Authorization: bearer abc123` (lowercase)
- **OR** with header `Authorization: TOKEN abc123` (uppercase)
- **THEN** the system SHALL inject `abc123` into the request context the same as it would for the canonical-case form, in `passthrough` mode
- **AND** in `resource-server` mode the scheme SHALL be matched case-insensitively too, with `Bearer` accepted and `token` refused

#### Scenario: Absent header falls through to global client

- **WHEN** the server runs in `passthrough` mode with `--allow-operator-token-fallback`
- **AND** an HTTP request arrives with no `Authorization` header
- **THEN** the request context SHALL NOT carry a token
- **AND** `forgejo.Client(ctx)` SHALL return the process-wide singleton initialized from the `--token` flag

#### Scenario: Bare token (no scheme) is rejected

- **WHEN** an HTTP request arrives with header `Authorization: abc123` (no scheme prefix)
- **THEN** the system SHALL treat the request as if no `Authorization` header were present
- **AND** the request SHALL be refused or fall through to the global singleton client
  exactly as a request with no `Authorization` header would

#### Scenario: Absent header on a network transport is refused

- **WHEN** the server runs on `http` or `sse` without
  `--allow-operator-token-fallback`, and a request arrives with no `Authorization`
  header
- **THEN** the request SHALL be refused with `401 Unauthorized` before it reaches an
  MCP handler
- **AND** `forgejo.Client(ctx)` SHALL return `ErrNoRequestToken` if reached by any
  other path

#### Scenario: Bare token (no scheme) is refused on a network transport

- **WHEN** an HTTP request arrives with header `Authorization: abc123` (no scheme)
- **THEN** the system SHALL treat it as carrying no credential
- **AND** on a network transport the request SHALL be refused rather than served with
  the server's own credential

#### Scenario: A well-formed credential passes the door unvalidated

- **WHEN** the server runs in `passthrough` mode
- **AND** an HTTP request arrives with header `Authorization: token not-a-real-token`
- **THEN** the request SHALL be admitted past the `401` check
- **AND** the token SHALL be injected into the request context unchanged, so that the
  forge, not this server, decides whether it is valid

#### Scenario: The inbound access token is not a Forgejo credential

- **WHEN** the server runs in `resource-server` mode
- **AND** a request to the MCP endpoint carries a valid access token
- **THEN** the request context SHALL carry the JWT this server minted for the caller, not the access token
- **AND** no request to Forgejo SHALL carry the access token

#### Scenario: Metadata and issuer documents are served without a credential

- **WHEN** the server runs in `resource-server` mode
- **AND** a request carrying no `Authorization` header asks for the protected resource metadata, the OpenID discovery document or the key set
- **THEN** the server SHALL answer it, rather than refusing with `401`
- **AND** a request carrying no `Authorization` header to the MCP endpoint SHALL still be refused with `401`

### Requirement: SSE transport extracts per-request token from Authorization header

When the server is started with `--transport sse`, the SSE server SHALL apply the same `Authorization`-header rules as the HTTP transport (above) — extraction via the SDK's `WithSSEContextFunc`, and refusal with `401 Unauthorized` of a request carrying no usable credential unless `--allow-operator-token-fallback` is set. The rules SHALL apply to every request, including each message posted to an existing session. All scenarios of the HTTP requirement apply identically.

#### Scenario: SSE request with `Bearer` scheme injects the token

- **WHEN** an SSE request arrives with header `Authorization: Bearer abc123`
- **THEN** the system SHALL inject `abc123` into the request context the same as the HTTP transport would

#### Scenario: SSE request with no Authorization header is refused

- **WHEN** the server runs on `sse` without `--allow-operator-token-fallback`
- **AND** a request arrives with no `Authorization` header
- **THEN** the request SHALL be refused with `401 Unauthorized` before it reaches an
  MCP handler

### Requirement: Token-aware client factory selects ephemeral or singleton client

When the supplied `ctx` carries a non-empty token, `forgejo.Client(ctx)` SHALL return
an ephemeral client bound to that token.

When the supplied `ctx` carries no token, the outcome SHALL depend on the transport:

- On `stdio`, and in `--cli` mode, `Client` SHALL return the process-wide singleton
  client initialised from the configured token. This is the behaviour the fallback was
  introduced for, and the trust boundary is the operating system's: the client
  launched this process.
- On `sse` and `http`, `Client` SHALL return `ErrNoRequestToken` and no client, unless
  the operator has set `--allow-operator-token-fallback`.

In `resource-server` mode the context always carries the JWT this server minted for the
request, so the factory takes the ephemeral path for every call. The singleton is never
constructed: that mode refuses to start with an operator token or the fallback flag, so
there is no credential for it to hold.

The same rule SHALL apply to the raw-HTTP helper, which SHALL refuse rather than send
the server's own credential. The decision SHALL live in one place shared by both, so a
future call path cannot reach a fallback that skipped it.

The policy SHALL NOT be derived from the address the listener bound. A loopback TCP
port is reachable by every local user account on the machine, and a reverse proxy in
front of a loopback listener makes the bound address say nothing about who can reach
the service — nginx and Apache both rewrite `Host` to the proxied target by default,
so a request from the internet arrives on a loopback socket carrying a loopback
`Host`.

#### Scenario: Context with token returns ephemeral client

- **WHEN** `forgejo.Client(ctx)` is called with a context that carries `token=abc123`
- **THEN** the system SHALL return a freshly constructed `*forgejo.Client` configured with `abc123`
- **AND** the returned client SHALL NOT be the package singleton

#### Scenario: Context without token returns singleton

- **WHEN** `forgejo.Client(ctx)` is called on `stdio`, or in `--cli` mode, with a context that carries no token
- **THEN** the system SHALL return the package singleton constructed from `flag.Token`
- **AND** consecutive calls SHALL return the same pointer
- **AND** on `sse` or `http` without `--allow-operator-token-fallback` the same call SHALL instead return `ErrNoRequestToken` and no client

#### Scenario: Absent header on stdio falls through to the global client

- **WHEN** the server runs on `stdio` and a request carries no token in its context
- **THEN** `forgejo.Client(ctx)` SHALL return the process-wide singleton initialised
  from the configured token

#### Scenario: The operator opts back in

- **WHEN** the server runs on `http` in `passthrough` mode with `--allow-operator-token-fallback`
- **AND** a request arrives with no `Authorization` header
- **THEN** the request SHALL be served using the configured token, as before this
  change
- **AND** the startup log SHALL state that anonymous requests are served with this
  server's own credential

#### Scenario: Every Forgejo call in resource-server mode uses a minted token

- **WHEN** the server runs in `resource-server` mode and an authenticated request invokes a tool
- **THEN** every Forgejo call the handler makes, through the typed client and through the raw-HTTP helpers, SHALL use the JWT minted for that request
- **AND** no call SHALL use the package singleton

### Requirement: Failed ephemeral construction MUST NOT fall through to the singleton

When `forgejo.Client(ctx)` is called with a context that carries a token, but the SDK call `forgejo.NewClient(...)` returns an error (e.g. the SDK's server-version probe fails on a network error, slow response, or HTTP 5xx from Forgejo), the function MUST NOT return the package singleton. Returning the singleton would silently authenticate the request as the startup `--token` identity instead of the requested per-request identity — a silent identity downgrade with privilege-escalation implications.

Instead, the function SHALL surface the failure to the caller. The implementation MAY accomplish this by changing the function signature to return `(*forgejo.Client, error)`, or by returning `nil` and logging an error at `Error` level. The chosen mechanism MUST cause the calling tool handler to fail the MCP request rather than silently process it under the wrong identity.

#### Scenario: Ephemeral construction failure surfaces an error

- **WHEN** `forgejo.Client(ctx)` is called with `ctx` carrying token `abc123`
- **AND** the underlying `forgejo.NewClient(...)` call returns an error
- **THEN** the system SHALL NOT return the package singleton
- **AND** the system SHALL surface the error such that the calling tool handler fails the request

#### Scenario: Ephemeral construction failure does NOT log the token

- **WHEN** the failure scenario above occurs
- **THEN** any logged error message or wrapped error chain SHALL NOT contain the value of `abc123`

### Requirement: Per-request clients are isolated across concurrent requests

Two concurrent requests carrying distinct tokens `T_A` and `T_B` SHALL produce two distinct, independent SDK clients. Outbound HTTP requests made by request A's handler SHALL carry `Authorization: token T_A`; outbound requests made by request B's handler SHALL carry `Authorization: token T_B`. Under no circumstances SHALL request A's client carry `T_B`, or vice versa, or the global `--token` value when a per-request token was supplied.

#### Scenario: Two concurrent requests produce two distinct identities at Forgejo

- **GIVEN** two goroutines, each obtaining a client via `forgejo.Client(ctx)` with distinct context tokens `T_A` and `T_B`
- **WHEN** each goroutine makes an outbound SDK call concurrently
- **THEN** the request originating from goroutine A SHALL arrive at Forgejo carrying `Authorization: token T_A`
- **AND** the request originating from goroutine B SHALL arrive carrying `Authorization: token T_B`

### Requirement: Raw HTTP path respects per-request token

The raw-HTTP helpers in `pkg/forgejo/rawhttp.go` (`DoJSON`, `DoJSONList`, `DoAPIRaw`, `DoMultipart`, `DoRaw` and their variants) SHALL resolve the credential for an outbound call through the same function the token-aware client factory uses, and SHALL prefer the token carried in `ctx` over the global `flag.Token` value when setting the `Authorization` header.

When no token is present in `ctx`, a helper SHALL fall back to `flag.Token` only where the token-aware client factory would. Otherwise it SHALL return `ErrNoRequestToken` without setting an `Authorization` header and without sending the request.

Attachment tool handlers in `operation/attachment/` SHALL pass their request `ctx` into
these helpers so per-request identity is honored for binary operations.

#### Scenario: Attachment upload uses per-request token

- **WHEN** an HTTP MCP request with `Authorization: token abc123` invokes `create_issue_attachment`
- **THEN** the outbound HTTP POST to Forgejo's attachment endpoint SHALL carry `Authorization: token abc123` in its headers, not the global `--token` value

#### Scenario: Raw HTTP helper refuses rather than sending the server's own credential

- **WHEN** a raw-HTTP helper is called with a context that carries no token
- **AND** the server runs on `sse` or `http` without `--allow-operator-token-fallback`
- **THEN** the helper SHALL return `ErrNoRequestToken`
- **AND** no outbound request SHALL carry the configured token

### Requirement: Stdio transport preserves pre-existing global-token behavior

When the server is started with `--transport stdio`, no `Authorization` extraction SHALL be performed. Tool handlers under stdio receive a context that carries no token; `forgejo.Client(ctx)` returns the package singleton initialized from `--token`. All pre-existing stdio behavior, including tests, SHALL remain valid.

#### Scenario: Stdio handler receives global-token client

- **WHEN** the server is run with `--transport stdio --token xyz789`
- **AND** a tool handler invokes `forgejo.Client(ctx)`
- **THEN** the system SHALL return the package singleton client constructed from `xyz789`
- **AND** consecutive calls within and across handlers SHALL return the same client pointer

### Requirement: A forge rejection never echoes the credential

When Forgejo answers `401` to a request that carried a credential, no tool result, JSON-RPC error, resource response or log line produced from that answer SHALL contain the credential or Forgejo's rejection text. This SHALL hold on every path to Forgejo, for a per-request credential and for the operator's own token alike. The failure SHALL instead carry the fixed message `Forgejo rejected the access token`.

#### Scenario: Rejection on an SDK call after the client was built

- **WHEN** the server runs on `http` in `passthrough` mode
- **AND** a tool call carries `Authorization: token T`, and Forgejo answers `401` to a call made through the SDK after the per-request client was constructed
- **THEN** the JSON-RPC error SHALL NOT contain `T`
- **AND** the JSON-RPC error SHALL carry the message `Forgejo rejected the access token`
- **AND** no line the server logs for that call SHALL contain `T`

#### Scenario: Rejection while the per-request client is built

- **WHEN** the server version is not yet known, so building the per-request client sends the credential to Forgejo
- **AND** Forgejo answers that request with `401`
- **THEN** neither the returned error nor any logged line SHALL contain the credential

#### Scenario: Rejection on a raw-HTTP helper call

- **WHEN** a tool that uses the raw-HTTP helpers receives a `401` from Forgejo for the per-request credential
- **THEN** the returned error SHALL NOT contain the credential or Forgejo's rejection text
- **AND** the error SHALL still match the `ErrUnauthorized` sentinel

#### Scenario: The operator token is rejected at startup

- **WHEN** the server starts with `--token T` and Forgejo answers the startup connection test with `401`
- **THEN** the server SHALL refuse to start as before
- **AND** neither the exit message nor any logged line SHALL contain `T`

### Requirement: A rejected per-request credential has a dedicated JSON-RPC error code

In `passthrough` mode on `http` and `sse`, a tool call or resource read during which Forgejo answered `401` to the caller's per-request credential SHALL fail with JSON-RPC error code `-32004` and the message `Forgejo rejected the access token`, whatever error the handler itself returned. Every other failure SHALL keep its current code.

#### Scenario: A revoked token fails with -32004

- **WHEN** a `tools/call` request carries `Authorization: token T` and Forgejo answers `401` to `T`
- **THEN** the JSON-RPC response SHALL be an error with code `-32004`
- **AND** its message SHALL be `Forgejo rejected the access token`

#### Scenario: A resource read with a revoked token fails with -32004

- **WHEN** a `resources/read` request carries a credential that Forgejo answers with `401`
- **THEN** the JSON-RPC response SHALL be an error with code `-32004`

#### Scenario: Insufficient scope is not a rejection

- **WHEN** Forgejo answers `403` to a call made with the per-request credential
- **THEN** the call SHALL fail as it does today, with a code other than `-32004`

#### Scenario: The SSE transport reports the code

- **WHEN** the server runs on `sse` in `passthrough` mode and Forgejo answers `401` to the caller's credential during a tool call
- **THEN** the result delivered on the event stream SHALL be an error with code `-32004`

### Requirement: passthrough on http answers a rejected credential with 401

In `passthrough` mode on `http`, when a request's response is a JSON-RPC error with code `-32004` and its HTTP status has not yet been sent, the server SHALL send status `401` with `WWW-Authenticate: Bearer error="invalid_token"` instead of `200`, keeping the JSON-RPC body. This SHALL NOT add a request to Forgejo or validate any credential at the door.

#### Scenario: A revoked token yields 401 with a challenge

- **WHEN** the server runs on `http` in `passthrough` mode
- **AND** a `tools/call` request carries a token that Forgejo answers with `401`
- **THEN** the HTTP response SHALL have status `401`
- **AND** it SHALL carry `WWW-Authenticate: Bearer error="invalid_token"`
- **AND** its body SHALL be the JSON-RPC error with code `-32004`

#### Scenario: A request that never reaches Forgejo is not challenged

- **WHEN** a `tools/list` request carries a token Forgejo would reject
- **THEN** the HTTP response SHALL have status `200`, as the door check does not validate credentials

#### Scenario: A response already being streamed keeps its status

- **WHEN** the server has already sent the status of a response, because it streams that response
- **AND** Forgejo then answers `401` to the caller's credential
- **THEN** the status SHALL be unchanged
- **AND** the call's result SHALL be the JSON-RPC error with code `-32004`

#### Scenario: stdio is unchanged

- **WHEN** the server runs on `stdio` and Forgejo rejects the operator's token during a tool call
- **THEN** the call SHALL fail with the fixed message, and no transport-level status SHALL be involved
