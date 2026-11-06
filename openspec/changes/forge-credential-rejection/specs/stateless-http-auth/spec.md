<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# stateless-http-auth — delta for forge-credential-rejection

## ADDED Requirements

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
