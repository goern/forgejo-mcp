<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# forgejo-jwt-issuer — delta for forge-credential-rejection

## ADDED Requirements

### Requirement: A rejected minted JWT is reported as a server-side failure

In `resource-server` mode, when Forgejo answers `401` to a JWT this server minted, the call SHALL fail with JSON-RPC code `-32603` and a fixed message saying that Forgejo rejected the credential this server issued for the caller. The response SHALL NOT carry status `401` or a `WWW-Authenticate` challenge. The server SHALL log the rejection for the operator, without the JWT.

#### Scenario: The integration was removed

- **WHEN** a caller with a valid access token makes a tool call
- **AND** Forgejo answers `401` to the minted JWT, for example because the caller's Authorized Integration was deleted
- **THEN** the HTTP response SHALL NOT have status `401` and SHALL NOT carry `WWW-Authenticate`
- **AND** the JSON-RPC error SHALL have code `-32603`
- **AND** its message SHALL say that Forgejo rejected the credential this server issued, not that the caller's sign-in failed

#### Scenario: The operator learns of the rejection

- **WHEN** Forgejo answers `401` to a minted JWT
- **THEN** the server SHALL log an entry at warning level or above naming the tool and the rejection
- **AND** that entry SHALL NOT contain the JWT or any part of its signature

#### Scenario: The echoed JWT never reaches the caller

- **WHEN** Forgejo's rejection message repeats the minted JWT
- **THEN** no part of the response to the caller SHALL contain the JWT
