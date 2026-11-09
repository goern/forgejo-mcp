# Spec Delta

## Purpose

How forgejo-mcp learns the version of the Forgejo instance it serves before any caller arrives: an anonymous probe that also works on instances that require sign-in, and an operator setting that replaces the probe.

## ADDED Requirements

### Requirement: The version is probed anonymously, with a swagger fallback

To learn the Forgejo version at startup, the server SHALL request `/api/v1/version` without any credential. If Forgejo answers `401` or `403`, the server SHALL read `info.version` from `/swagger.v1.json`, also without a credential. It SHALL read at most a bounded amount of that document. The probe reports either a version, or that Forgejo requires sign-in, or that Forgejo was not usable.

#### Scenario: Version endpoint open

- **WHEN** `/api/v1/version` answers `200` with `{"version":"16.0.7+gitea-1.22.0"}`
- **THEN** the probe SHALL report version `16.0.7+gitea-1.22.0`
- **AND** it SHALL NOT request `/swagger.v1.json`

#### Scenario: Instance requires sign-in, swagger published

- **WHEN** `/api/v1/version` answers `403`
- **AND** `/swagger.v1.json` answers `200` with `info.version` `15.0.11+gitea-1.22.0`
- **THEN** the probe SHALL report version `15.0.11+gitea-1.22.0`

#### Scenario: Instance requires sign-in, swagger disabled

- **WHEN** `/api/v1/version` answers `401` or `403`
- **AND** `/swagger.v1.json` does not answer `200` with a JSON document that carries a non-empty `info.version`
- **THEN** the probe SHALL report that Forgejo requires sign-in and that no version could be read

#### Scenario: Forgejo not reachable or not Forgejo

- **WHEN** the request to `/api/v1/version` fails before a response arrives (DNS, connection or TLS error), or answers a status other than `200`, `401` or `403`
- **THEN** the probe SHALL report that Forgejo was not usable, with the cause
- **AND** it SHALL NOT request `/swagger.v1.json`

#### Scenario: No credential is sent

- **WHEN** the probe runs, whatever token or mode is configured
- **THEN** neither request SHALL carry an `Authorization` header

#### Scenario: Oversized swagger document

- **WHEN** `/swagger.v1.json` answers with a body larger than the read bound before `info.version` is found
- **THEN** the probe SHALL stop reading and report that no version could be read

### Requirement: An operator-supplied version replaces the probe

The server SHALL accept the Forgejo version as `--forgejo-version` or `FORGEJO_VERSION`, the flag winning over the environment. The setting is optional in every mode. When it is set, the server SHALL NOT probe the version. It SHALL use the value wherever a probed version would be used, and SHALL hand it to the Forgejo client so that clients built later do not request the version themselves. A value whose major version cannot be read SHALL refuse startup.

#### Scenario: Setting given

- **WHEN** the server starts with `--forgejo-version 16.0.7`
- **THEN** it SHALL NOT request `/api/v1/version` or `/swagger.v1.json` to learn the version
- **AND** the startup log SHALL state the version and that it came from the setting

#### Scenario: Flag wins over environment

- **WHEN** `FORGEJO_VERSION=15.0.11` is set and the server starts with `--forgejo-version 16.0.7`
- **THEN** the server SHALL use version `16.0.7`

#### Scenario: Unparsable value

- **WHEN** the server starts with `--forgejo-version banana`
- **THEN** it SHALL refuse to start before binding any listener
- **AND** the message SHALL name `--forgejo-version`

#### Scenario: Setting absent

- **WHEN** neither `--forgejo-version` nor `FORGEJO_VERSION` is set
- **THEN** startup SHALL behave as without this setting, probing where the mode probes today
