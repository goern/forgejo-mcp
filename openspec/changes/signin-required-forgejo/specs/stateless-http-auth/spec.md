<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Spec Delta

## ADDED Requirements

### Requirement: Tokenless passthrough on a network transport starts on a sign-in-only Forgejo

In `passthrough` mode on `http` or `sse` with no operator token configured, the startup check SHALL be the anonymous version probe of capability `forgejo-version-detection`, not an SDK client. If Forgejo requires sign-in, the server SHALL log a warning and start. It SHALL refuse to start only when Forgejo was not usable. With `--forgejo-version` set it SHALL NOT read the version, but SHALL still check anonymously that Forgejo is reachable. Per-request authentication SHALL NOT change.

#### Scenario: Forgejo requires sign-in

- **WHEN** the server starts on `http` in `passthrough` mode with no operator token
- **AND** Forgejo answers `403` on `/api/v1/version` and the swagger fallback reads no version
- **THEN** the server SHALL start and accept connections
- **AND** the startup log SHALL contain a warning that Forgejo requires sign-in and that its version is unknown

#### Scenario: Version read through the fallback

- **WHEN** the server starts on `http` in `passthrough` mode with no operator token
- **AND** Forgejo answers `403` on `/api/v1/version` and `/swagger.v1.json` carries `info.version` `16.0.7+gitea-1.22.0`
- **THEN** the server SHALL start
- **AND** the startup log SHALL state version `16.0.7+gitea-1.22.0`

#### Scenario: Wrong URL still fails fast

- **WHEN** the server starts on `http` in `passthrough` mode with no operator token
- **AND** the host in `--url` does not resolve, `/api/v1/version` answers `404`, or it answers `403` with a body that is not a Forgejo API error
- **THEN** the server SHALL refuse to start before binding any listener, whether or not `--forgejo-version` is set

#### Scenario: Requests still need their own token

- **WHEN** the server has started against a Forgejo instance that requires sign-in
- **AND** a request arrives with no `Authorization` header
- **THEN** it SHALL be refused with `401`, as without this requirement

#### Scenario: Operator token configured

- **WHEN** the server starts on `http` in `passthrough` mode with an operator token configured
- **THEN** the startup check SHALL authenticate with that token as before
- **AND** a failure of that check SHALL refuse startup

#### Scenario: stdio is unchanged

- **WHEN** the server starts on `stdio` with no operator token and Forgejo answers `403` to anonymous requests
- **THEN** the server SHALL refuse to start, as before this requirement
