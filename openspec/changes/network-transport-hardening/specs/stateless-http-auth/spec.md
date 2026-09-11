<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

### Requirement: HTTP transport extracts per-request token from Authorization header

When the server is started with `--transport http`, every incoming MCP request SHALL be
inspected for an `Authorization` header. The header SHALL be parsed for one of two
schemes, case-insensitively:

1. `token <X>` — Forgejo's native token scheme.
2. `Bearer <X>` — OAuth2-style bearer transport.

When a recognized scheme is present, the parsed token value SHALL be injected into the
request `context.Context` via `forgejo.WithToken(ctx, token)`. The MCP handler invoked
for that request receives the augmented context.

When the `Authorization` header is absent, empty, or carries an unrecognized scheme, the
request `context.Context` SHALL NOT carry a token, and the request SHALL be refused with
`401 Unauthorized` before it reaches an MCP handler — unless the operator has set
`--allow-operator-token-fallback`, in which case the request is admitted and downstream
code falls back to the global singleton client (see "Token-aware client factory" below).

The transport MUST NOT accept tokens without a scheme prefix; a header value that does
not match one of the two named schemes SHALL be treated as if no header were present.

This check establishes only that a credential is present and carries a recognized
scheme. It does not validate the credential: a request carrying an invalid token passes
it, and the forge refuses that token on the first call that reaches it. A request that
never reaches the forge — `initialize`, `tools/list`, or holding an event stream open —
is therefore not protected against a caller presenting any well-formed credential.

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

- **WHEN** the server runs with `--allow-operator-token-fallback`
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

- **WHEN** an HTTP request arrives with header `Authorization: token not-a-real-token`
- **THEN** the request SHALL be admitted past the `401` check
- **AND** the token SHALL be injected into the request context unchanged, so that the
  forge, not this server, decides whether it is valid

### Requirement: SSE transport extracts per-request token from Authorization header

When the server is started with `--transport sse`, the SSE server SHALL apply the same
`Authorization`-header rules as the HTTP transport (above) — extraction via the SDK's
`WithSSEContextFunc`, and refusal with `401 Unauthorized` of a request carrying no
usable credential unless `--allow-operator-token-fallback` is set. The rules SHALL apply
to every request, including each message posted to an existing session. All scenarios
of the HTTP requirement apply identically.

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

- **WHEN** the server runs on `http` with `--allow-operator-token-fallback`
- **AND** a request arrives with no `Authorization` header
- **THEN** the request SHALL be served using the configured token, as before this
  change
- **AND** the startup log SHALL state that anonymous requests are served with this
  server's own credential

### Requirement: Raw HTTP path respects per-request token

The raw-HTTP helpers in `pkg/forgejo/rawhttp.go` (`DoJSON`, `DoJSONList`, `DoAPIRaw`,
`DoMultipart`, `DoRaw` and their variants) SHALL resolve the credential for an
outbound call through the same function the token-aware client factory uses, and
SHALL prefer the token carried in `ctx` over the global `flag.Token` value when setting
the `Authorization` header.

When no token is present in `ctx`, a helper SHALL fall back to `flag.Token` only where
the token-aware client factory would. Otherwise it SHALL return `ErrNoRequestToken`
without setting an `Authorization` header and without sending the request.

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
