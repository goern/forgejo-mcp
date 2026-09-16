<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

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
