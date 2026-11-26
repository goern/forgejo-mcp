<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Spec Delta

## MODIFIED Requirements

### Requirement: resource-server mode refuses to start on an unusable configuration

In `resource-server` mode, the server SHALL check its configuration before binding any listener. If any of the following holds, the server SHALL refuse to start, with a message that names the offending setting:

- **Transport.** The transport is not `http` (for example `stdio` or `sse`), or the server runs in `--cli` mode.
- **Operator credential.** An operator token is configured (`-token`, `FORGEJO_ACCESS_TOKEN` or its deprecated alias), or the operator-token fallback is enabled.
- **Missing setting.** Any of `-authorization-server`, `-resource`, `-forgejo-jwt-issuer` or `-forgejo-jwt-signing-key-file` is unset.
- **Forgejo issuer scheme.** The URL in `-forgejo-jwt-issuer` does not use `https`.
- **Forgejo issuer shape.** The URL in `-forgejo-jwt-issuer` ends with `/`, or carries a query, a fragment or user information. Forgejo compares `iss` byte for byte, so only one spelling of the issuer may exist.
- **Other URL schemes.** `-authorization-server` or `-resource` uses a scheme other than `https`. The only exception is `http` with a loopback host.
- **Host not answered.** The host of `-resource` or of `-forgejo-jwt-issuer` is not a host this server answers to under its Host policy.
- **Resource path.** The path of `-resource` is not `/mcp`, the path of the MCP endpoint. The protected resource metadata names `-resource` as the resource, so any other path publishes a resource this server does not serve.
- **Signing key.** The signing key or a published key cannot be loaded, is not of a supported type (see capability `forgejo-jwt-issuer`), or two published keys have the same key ID.
- **Forgejo version.** `--forgejo-version` is unset and the anonymous probe of capability `forgejo-version-detection` reads no version, or the version in use, probed or set, is below 16.0, or `--forgejo-version` is set and Forgejo is not reachable. When the probe read no version, the message SHALL name `--forgejo-version`.
- **Identity provider metadata.** The provider's metadata cannot be retrieved, its `issuer` differs from `-authorization-server` by even one character, or it names no `jwks_uri`.

#### Scenario: Incompatible transport

- **WHEN** the server is started with `-auth-mode resource-server` and `-transport sse`
- **THEN** it SHALL refuse to start before binding any listener
- **AND** the message SHALL say that `resource-server` mode requires the `http` transport

#### Scenario: Operator token present

- **WHEN** the server is started in `resource-server` mode with `FORGEJO_ACCESS_TOKEN` set
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that this mode takes no operator token

#### Scenario: Forgejo too old

- **WHEN** the server is started in `resource-server` mode and the Forgejo instance reports version `15.0.3`
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that `resource-server` mode requires Forgejo 16.0 or newer

#### Scenario: Forgejo requires sign-in, version read from swagger

- **WHEN** the server is started in `resource-server` mode, Forgejo answers `403` on `/api/v1/version`, and `/swagger.v1.json` carries `info.version` `16.0.7+gitea-1.22.0`
- **THEN** the version check SHALL pass with version `16.0.7+gitea-1.22.0`

#### Scenario: Forgejo requires sign-in and publishes no swagger

- **WHEN** the server is started in `resource-server` mode without `--forgejo-version`, Forgejo answers `403` on `/api/v1/version`, and the swagger fallback reads no version
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that Forgejo requires sign-in to read its version and name `--forgejo-version`

#### Scenario: Version supplied by the operator

- **WHEN** the server is started in `resource-server` mode with `--forgejo-version 16.0.7`
- **THEN** the version check SHALL pass without reading the version from Forgejo
- **AND** the server SHALL check, with one anonymous request, that Forgejo is reachable

#### Scenario: Supplied version too old

- **WHEN** the server is started in `resource-server` mode with `--forgejo-version 15.0.11`
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that `resource-server` mode requires Forgejo 16.0 or newer

#### Scenario: Provider issuer mismatch

- **WHEN** `-authorization-server` is `https://id.example.org` and the provider's metadata reports `issuer` as `https://id.example.org/`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL show both values

#### Scenario: Issuer URL with a trailing slash

- **WHEN** `-forgejo-jwt-issuer` is `https://mcp.example.org/issuer/`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL say that the issuer URL must not end with a slash

#### Scenario: Issuer host not answered

- **WHEN** `-forgejo-jwt-issuer` is `https://mcp.example.org/issuer`, the listener is not loopback-only, and `-allowed-hosts` does not include `mcp.example.org`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL name `-allowed-hosts`

#### Scenario: Resource names another path

- **WHEN** `-resource` is `https://mcp.example.org/api/mcp`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL name `-resource` and the endpoint path `/mcp`
