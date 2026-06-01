# stateless-http-auth

## Purpose

Enable per-request authentication for the HTTP and SSE transports by extracting a token from the `Authorization` header and constructing a request-scoped Forgejo client, while preserving the pre-existing global-token (singleton) behavior for the stdio transport. This allows a single server process to serve multiple distinct Forgejo identities concurrently without leaking credentials across requests.

## Requirements

### Requirement: HTTP transport extracts per-request token from Authorization header

When the server is started with `--transport http`, every incoming MCP request SHALL be inspected for an `Authorization` header. The header SHALL be parsed for one of two schemes, case-insensitively:

1. `token <X>` — Forgejo's native token scheme.
2. `Bearer <X>` — OAuth2-style bearer transport.

When a recognized scheme is present, the parsed token value SHALL be injected into the request `context.Context` via `forgejo.WithToken(ctx, token)`. The MCP handler invoked for that request receives the augmented context.

When the `Authorization` header is absent, empty, or carries an unrecognized scheme, the request `context.Context` SHALL NOT carry a token. Downstream code falls back to the global singleton client (see "Token-aware client factory" below).

The transport MUST NOT accept tokens without a scheme prefix; a header value that does not match one of the two named schemes SHALL be treated as if no header were present.

#### Scenario: Request with `token` scheme injects the token

- **WHEN** an HTTP request arrives with header `Authorization: token abc123`
- **THEN** the system SHALL inject `abc123` into the request context via `forgejo.WithToken`
- **AND** the MCP handler SHALL see that token via `forgejo.Client(ctx)`

#### Scenario: Request with `Bearer` scheme injects the token

- **WHEN** an HTTP request arrives with header `Authorization: Bearer abc123`
- **THEN** the system SHALL inject `abc123` into the request context

#### Scenario: Scheme matching is case-insensitive

- **WHEN** an HTTP request arrives with header `Authorization: bearer abc123` (lowercase)
- **OR** with header `Authorization: TOKEN abc123` (uppercase)
- **THEN** the system SHALL inject `abc123` into the request context the same as it would for the canonical-case form

#### Scenario: Absent header falls through to global client

- **WHEN** an HTTP request arrives with no `Authorization` header
- **THEN** the request context SHALL NOT carry a token
- **AND** `forgejo.Client(ctx)` SHALL return the process-wide singleton initialized from the `--token` flag

#### Scenario: Bare token (no scheme) is rejected

- **WHEN** an HTTP request arrives with header `Authorization: abc123` (no scheme prefix)
- **THEN** the system SHALL treat the request as if no `Authorization` header were present
- **AND** the system SHALL fall through to the global singleton client

### Requirement: SSE transport extracts per-request token from Authorization header

When the server is started with `--transport sse`, the SSE server SHALL apply the same `Authorization`-header extraction rules as the HTTP transport (above), via the SDK's `WithSSEContextFunc`. All scheme-matching scenarios apply identically.

#### Scenario: SSE request with `Bearer` scheme injects the token

- **WHEN** an SSE request arrives with header `Authorization: Bearer abc123`
- **THEN** the system SHALL inject `abc123` into the request context the same as the HTTP transport would

### Requirement: Token-aware client factory selects ephemeral or singleton client

The function `forgejo.Client(ctx context.Context) *forgejo.Client` SHALL be the single entry point used by every tool handler to obtain a Forgejo SDK client.

When the supplied `ctx` carries a non-empty token (set via `forgejo.WithToken`), `Client` SHALL construct a fresh, request-scoped SDK client via `forgejo.NewClient(flag.URL, forgejo.SetToken(token))` and return it. This client SHALL NOT be cached, shared, or reused across requests.

When the supplied `ctx` carries no token, `Client` SHALL return the process-wide singleton client. The singleton SHALL be initialized lazily on first call via `sync.Once`, using `flag.Token` as the credential. This preserves the pre-existing stdio behavior verbatim.

#### Scenario: Context with token returns ephemeral client

- **WHEN** `forgejo.Client(ctx)` is called with a context that carries `token=abc123`
- **THEN** the system SHALL return a freshly constructed `*forgejo.Client` configured with `abc123`
- **AND** the returned client SHALL NOT be the package singleton

#### Scenario: Context without token returns singleton

- **WHEN** `forgejo.Client(ctx)` is called with a context that carries no token
- **THEN** the system SHALL return the package singleton constructed from `flag.Token`
- **AND** consecutive calls SHALL return the same pointer

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

The `pkg/forgejo/rawhttp.go` helper used for attachment upload and download SHALL read the request token from `ctx` (via `tokenFromContext`) and prefer it over the global `flag.Token` value when setting the `Authorization` header on outbound raw HTTP calls. When no token is present in `ctx`, the helper SHALL fall back to `flag.Token`.

Attachment tool handlers in `operation/attachment/` SHALL pass their request `ctx` into this helper so per-request identity is honored for binary operations.

#### Scenario: Attachment upload uses per-request token

- **WHEN** an HTTP MCP request with `Authorization: token abc123` invokes `create_issue_attachment`
- **THEN** the outbound HTTP POST to Forgejo's attachment endpoint SHALL carry `Authorization: token abc123` in its headers, not the global `--token` value

### Requirement: Stdio transport preserves pre-existing global-token behavior

When the server is started with `--transport stdio`, no `Authorization` extraction SHALL be performed. Tool handlers under stdio receive a context that carries no token; `forgejo.Client(ctx)` returns the package singleton initialized from `--token`. All pre-existing stdio behavior, including tests, SHALL remain valid.

#### Scenario: Stdio handler receives global-token client

- **WHEN** the server is run with `--transport stdio --token xyz789`
- **AND** a tool handler invokes `forgejo.Client(ctx)`
- **THEN** the system SHALL return the package singleton client constructed from `xyz789`
- **AND** consecutive calls within and across handlers SHALL return the same client pointer
