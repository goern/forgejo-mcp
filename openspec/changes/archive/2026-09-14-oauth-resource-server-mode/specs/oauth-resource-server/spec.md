<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Purpose

This capability lets forgejo-mcp run on the `http` transport as an OAuth 2.0 resource server, as the MCP authorization specification defines it. Callers authenticate with a JWT access token from one configured OpenID Connect provider.

It covers:
- selecting the mode;
- refusing to start on an incomplete or contradictory configuration;
- validating inbound tokens;
- the `401` challenge;
- the public metadata that tells clients how to authorise.

## ADDED Requirements

### Requirement: Auth mode is selected explicitly and defaults to passthrough

The server SHALL accept an auth mode through `-auth-mode` or `FORGEJO_MCP_AUTH_MODE`, with the values `passthrough` and `resource-server`. When neither is set, the mode SHALL be `passthrough`. In `passthrough` mode, the server's behaviour on every transport SHALL be unchanged from before this capability existed. Any other value SHALL prevent the server from starting, with a message that names the setting and the accepted values.

A flag passed on the command line SHALL take precedence over the environment variable, even when the flag's value equals the default.

#### Scenario: No mode configured

- **WHEN** the server starts on `http` with neither `-auth-mode` nor `FORGEJO_MCP_AUTH_MODE` set
- **THEN** it SHALL run in `passthrough` mode
- **AND** it SHALL authenticate requests exactly as it did before this capability existed

#### Scenario: Unknown mode value

- **WHEN** the server is started with `-auth-mode oauth`
- **THEN** it SHALL refuse to start
- **AND** the message SHALL name `-auth-mode` and list `passthrough` and `resource-server`

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
- **Forgejo version.** The configured Forgejo instance does not report its version, or reports a version below 16.0.
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

### Requirement: passthrough mode refuses resource-server settings

In `passthrough` mode, the server SHALL refuse to start when any setting that only applies to `resource-server` mode is present, whether as a flag or as an environment variable. Those settings are:

- the authorization server;
- the resource and its audience;
- the supported scopes;
- the Forgejo audience claim;
- the Forgejo JWT issuer;
- the signing and published key files.

This catches a configuration that was meant to enable the mode but omitted the mode switch.

#### Scenario: Issuer configured without the mode

- **WHEN** the server starts with `FORGEJO_MCP_AUTHORIZATION_SERVER` set and no auth mode configured
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that the setting requires `-auth-mode resource-server`

### Requirement: The MCP endpoint requires a valid bearer JWT access token

In `resource-server` mode, every request to the MCP endpoint SHALL carry `Authorization: Bearer <token>`, with the scheme matched case-insensitively. The server SHALL accept the request only if the token is a JWT that satisfies all of these conditions:

- **Signature.** It is signed with an asymmetric algorithm, by a key the provider publishes, identified by the token's key ID. The algorithm is taken from the key, not from the token.
- **Issuer.** Its `iss` equals `-authorization-server` exactly.
- **Expiry.** It carries `exp`, and the token has not expired.
- **Validity window.** Its `nbf` and `iat`, when present, are not in the future beyond a tolerance of 60 seconds.
- **Audience.** Its `aud`, whether a string or an array, contains the configured resource audience. That audience is `-resource-audience` when set, otherwise the value of `-resource`.
- **Not an ID token.** It carries neither `nonce` nor `at_hash`.
- **Type header.** Its `typ` header, when present, is `at+jwt` or `JWT`.

Requests with the `token` scheme, opaque tokens and tokens failing any condition SHALL be refused as specified in the challenge requirement below.

#### Scenario: Valid access token

- **WHEN** a request to the MCP endpoint carries a JWT access token signed by the provider, with the configured issuer, an unexpired `exp`, and an `aud` array containing the resource audience
- **THEN** the request SHALL be passed to MCP handling

#### Scenario: Audience names another resource

- **WHEN** a request carries an otherwise valid token whose `aud` does not contain the resource audience
- **THEN** the server SHALL respond `401`

#### Scenario: ID token presented as an access token

- **WHEN** a request carries a token signed by the provider with a matching `iss` and `aud` that also carries a `nonce` claim
- **THEN** the server SHALL respond `401`

#### Scenario: Symmetric algorithm

- **WHEN** a request carries a token whose header declares `HS256`
- **THEN** the server SHALL respond `401`

#### Scenario: Opaque token

- **WHEN** a request carries `Authorization: Bearer 8f3a...` that is not a JWT
- **THEN** the server SHALL respond `401`

#### Scenario: Forgejo token scheme

- **WHEN** a request carries `Authorization: token <valid JWT>`
- **THEN** the server SHALL respond `401`

### Requirement: Refusals use a uniform 401 challenge

When a request to the MCP endpoint carries no token, or carries one that fails validation, the server SHALL respond `401` with an empty body. The response SHALL carry this header:

`WWW-Authenticate: Bearer resource_metadata="<URL of the protected resource metadata>"`

The challenge SHALL include `scope="<scopes>"` when `-scopes-supported` is set. It SHALL include `error="invalid_token"` when a token was presented.

The response SHALL NOT reveal which condition failed; the reason SHALL be logged at debug level. The rate of refusal log lines SHALL be bounded.

#### Scenario: No token

- **WHEN** a request to the MCP endpoint carries no `Authorization` header
- **THEN** the server SHALL respond `401` with `WWW-Authenticate` carrying `resource_metadata`
- **AND** the challenge SHALL NOT carry an `error` parameter

#### Scenario: Expired and wrongly signed tokens are indistinguishable

- **WHEN** one request carries an expired token and another carries a token with an invalid signature
- **THEN** both responses SHALL be byte-identical
- **AND** both SHALL carry `error="invalid_token"`

### Requirement: Unknown key IDs trigger bounded key refetches

The server SHALL cache the provider's published keys. When a token names a key ID that is not in the cache, the server SHALL refetch the key set at most once per minimum refresh interval. It SHALL refuse the token if the key is still unknown. The server SHALL NOT fetch the provider's keys once per request.

A refetch SHALL run to completion even when the request that triggered it is cancelled. Otherwise a client that disconnects mid-fetch spends the interval without the key set being refreshed, and repeating that once per interval would keep the server from ever learning a rotated key.

#### Scenario: Burst of unknown key IDs

- **WHEN** many requests within one refresh interval carry tokens with different unknown key IDs
- **THEN** the server SHALL fetch the provider's key set at most once during that interval
- **AND** every such request SHALL receive `401`

#### Scenario: Caller disconnects during a refetch

- **WHEN** a request past the refresh interval names a key ID the provider has just added, and its client disconnects before the key set is fetched
- **THEN** the server SHALL complete the fetch
- **AND** a later request within the same interval SHALL find the new key without another fetch

### Requirement: Protected resource metadata is published without authentication

In `resource-server` mode, the server SHALL serve the protected resource metadata defined by RFC 9728. It SHALL be served both at `/.well-known/oauth-protected-resource` followed by the path of `-resource`, and at `/.well-known/oauth-protected-resource`.

The document SHALL contain:
- `resource`, equal to `-resource`;
- `authorization_servers`, containing `-authorization-server`;
- `bearer_methods_supported`, equal to `["header"]`;
- `scopes_supported`, only when `-scopes-supported` is set.

These routes SHALL NOT require an `Authorization` header, and SHALL remain subject to the server's Host and Origin validation.

#### Scenario: Metadata at the path-specific location

- **WHEN** `-resource` is `https://mcp.example.org/mcp` and a request without `Authorization` asks for `/.well-known/oauth-protected-resource/mcp`
- **THEN** the server SHALL respond `200` with a JSON document whose `resource` is `https://mcp.example.org/mcp`

#### Scenario: Metadata with a forged Host

- **WHEN** a request for the metadata carries a Host the server does not answer to
- **THEN** the server SHALL respond `403`

### Requirement: Only defined routes are served

In `resource-server` mode, the server SHALL answer only these paths:
- the MCP endpoint;
- the protected resource metadata routes;
- the discovery document and key set routes of capability `forgejo-jwt-issuer`.

It SHALL match these paths exactly, and SHALL NOT redirect a request to any of them. Every other path SHALL receive `404`, including `/.well-known/openid-configuration` at the root and any variant with a trailing slash.

#### Scenario: Root OpenID discovery

- **WHEN** a request asks for `/.well-known/openid-configuration` and the Forgejo JWT issuer has a path component
- **THEN** the server SHALL respond `404`

#### Scenario: Trailing slash

- **WHEN** a request asks for the metadata path with an added trailing slash
- **THEN** the server SHALL respond `404`
- **AND** it SHALL NOT respond with a redirect

### Requirement: Inbound tokens are never forwarded or logged

In `resource-server` mode, the inbound access token SHALL NOT be sent to Forgejo, made available to tool handlers, included in any response, or written to any log. The token's `sub` and the Forgejo audience derived from it MAY appear in logs at debug level only.

#### Scenario: Forgejo never sees the inbound token

- **WHEN** a tool call authenticated with an inbound access token makes requests to Forgejo
- **THEN** none of those requests SHALL carry the inbound access token

#### Scenario: Debug logging of a refusal

- **WHEN** the server refuses a token with debug logging enabled
- **THEN** the log SHALL NOT contain the token or any part of its signature
