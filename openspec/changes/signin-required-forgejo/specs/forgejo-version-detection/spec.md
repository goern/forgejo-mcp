<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Spec Delta

## Purpose

How forgejo-mcp learns the version of the Forgejo instance it serves before any caller arrives: an anonymous probe that also works on instances that require sign-in, and an operator setting that replaces the probe.

## ADDED Requirements

### Requirement: The version is probed anonymously, with a swagger fallback

To learn the Forgejo version at startup, the server SHALL request `/api/v1/version` without any credential. If Forgejo answers `401` or `403` with a Forgejo API error (a JSON object with a non-empty `message`), the server SHALL read `info.version` from `/swagger.v1.json`, also without a credential. The probe reports either a version, or that Forgejo requires sign-in, or that Forgejo was not usable.

#### Scenario: Version endpoint open

- **WHEN** `/api/v1/version` answers `200` with `{"version":"16.0.7+gitea-1.22.0"}`
- **THEN** the probe SHALL report version `16.0.7+gitea-1.22.0`
- **AND** it SHALL NOT request `/swagger.v1.json`

#### Scenario: Instance requires sign-in, swagger published

- **WHEN** `/api/v1/version` answers `403`
- **AND** `/swagger.v1.json` answers `200` with `info.version` `15.0.11+gitea-1.22.0`
- **THEN** the probe SHALL report version `15.0.11+gitea-1.22.0`

#### Scenario: Instance requires sign-in, swagger disabled

- **WHEN** `/api/v1/version` answers `401` or `403` with a Forgejo API error
- **AND** `/swagger.v1.json` does not answer `200` with a JSON document that carries a non-empty `info.version`
- **THEN** the probe SHALL report that Forgejo requires sign-in and that no version could be read

#### Scenario: Refusal that is not from Forgejo

- **WHEN** `/api/v1/version` answers `401` or `403` with a body that is not a Forgejo API error, such as an HTML page from a proxy
- **THEN** the probe SHALL report that Forgejo was not usable
- **AND** it SHALL NOT request `/swagger.v1.json`

#### Scenario: Forgejo not reachable or not Forgejo

- **WHEN** the request to `/api/v1/version` fails before a response arrives (DNS, connection or TLS error), or answers a status other than `200`, `401` or `403`
- **THEN** the probe SHALL report that Forgejo was not usable, with the cause
- **AND** it SHALL NOT request `/swagger.v1.json`

#### Scenario: No credential is sent

- **WHEN** the probe runs, whatever token or mode is configured
- **THEN** neither request SHALL carry an `Authorization` header

### Requirement: The probe is bounded and trusts only the instance

The probe SHALL read at most a bounded amount of `/swagger.v1.json` and SHALL finish within a bounded time. It SHALL NOT trust a swagger document reached through a redirect to outside the configured instance. No probe error SHALL contain user information from the configured URL.

#### Scenario: Oversized swagger document

- **WHEN** `/swagger.v1.json` answers with a body larger than the read bound before `info.version` is found
- **THEN** the probe SHALL stop reading and report that no version could be read

#### Scenario: Swagger redirected off the instance

- **WHEN** `/api/v1/version` answers `403` with a Forgejo API error
- **AND** `/swagger.v1.json` redirects to a URL outside the configured instance
- **THEN** the probe SHALL NOT use a version from that document

#### Scenario: User information in the URL

- **WHEN** `--url` carries user information, such as `https://user:secret@forge.example`
- **AND** the probe fails
- **THEN** its error SHALL NOT contain the user name or the password

### Requirement: An operator-supplied version replaces the probe

The server SHALL accept the Forgejo version as `--forgejo-version` or `FORGEJO_MCP_FORGEJO_VERSION`, the flag winning, in every mode, `--cli` included. When it is set, the server SHALL NOT read the version from Forgejo, SHALL use the value wherever a probed version would be used, and SHALL hand it to the Forgejo client so that later clients do not request the version. A value the Forgejo client cannot parse SHALL refuse startup.

#### Scenario: Setting given

- **WHEN** the server starts with `--forgejo-version 16.0.7`
- **THEN** it SHALL NOT request `/swagger.v1.json`
- **AND** any request to `/api/v1/version` SHALL serve only to check that Forgejo is reachable
- **AND** the startup log SHALL state the version and that it came from the setting

#### Scenario: Flag wins over environment

- **WHEN** `FORGEJO_MCP_FORGEJO_VERSION=15.0.11` is set and the server starts with `--forgejo-version 16.0.7`
- **THEN** the server SHALL use version `16.0.7`

#### Scenario: Unparsable value

- **WHEN** the server starts with `--forgejo-version banana`, or with `16.banana`, which has a numeric major but no version the Forgejo client can parse
- **THEN** it SHALL refuse to start before binding any listener
- **AND** the message SHALL name `--forgejo-version`

#### Scenario: Setting absent

- **WHEN** neither `--forgejo-version` nor `FORGEJO_MCP_FORGEJO_VERSION` is set
- **THEN** startup SHALL behave as without this setting, probing where the mode probes today

### Requirement: A stated version is still checked against Forgejo

Where startup checks Forgejo anonymously and `--forgejo-version` is set, the server SHALL still send one anonymous request to `/api/v1/version`, to check that Forgejo is reachable. When Forgejo reports a version, through that request or through an authenticated startup check, and it differs from the setting other than in build metadata, the server SHALL log a warning naming both and keep using the setting.

#### Scenario: Stated version differs

- **WHEN** the server starts with `--forgejo-version 16.0.6` and Forgejo reports `16.0.7+gitea-1.22.0`
- **THEN** the server SHALL start with version `16.0.6`
- **AND** the startup log SHALL contain a warning naming both versions

#### Scenario: Setting given, Forgejo unreachable

- **WHEN** the server starts on `http` without an operator token, with `--forgejo-version 16.0.7`, and the host in `--url` does not resolve
- **THEN** it SHALL refuse to start before binding any listener
