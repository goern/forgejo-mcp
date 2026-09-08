<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

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

#### Scenario: The operator opts back in

- **WHEN** the server runs on `http` with `--allow-operator-token-fallback`
- **AND** a request arrives with no `Authorization` header
- **THEN** the request SHALL be served using the configured token, as before this
  change
- **AND** the startup log SHALL state that anonymous requests are served with this
  server's own credential
