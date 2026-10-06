<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<!-- demos-anchored: true -->

# forgejo-jwt-issuer Specification

## Purpose
Defines how forgejo-mcp, in `resource-server` mode, obtains a Forgejo credential for the calling person. forgejo-mcp signs a short-lived JWT that Forgejo 16 accepts through Authorized Integrations, and it publishes the discovery document and key set that Forgejo needs to verify that JWT.

## Requirements

### Requirement: Forgejo calls carry a JWT signed by forgejo-mcp

In `resource-server` mode, every request the server makes to Forgejo while serving an authenticated MCP request SHALL carry, as its credential, one JWT that the server signed for that MCP request. This applies to every client path the server uses to reach Forgejo. No other credential SHALL be used for those requests, neither an operator token nor the inbound access token. The JWT SHALL be sent as `Authorization: token <jwt>`.

The server SHALL sign a new JWT for each authenticated MCP request and SHALL NOT reuse one across requests.

#### Scenario: A tool call reaches Forgejo

- **WHEN** an authenticated MCP request invokes a tool that calls the Forgejo API twice
- **THEN** both Forgejo requests SHALL carry `Authorization: token <jwt>`, with the same JWT signed by forgejo-mcp
- **AND** neither SHALL carry the inbound access token

#### Scenario: Two MCP requests

- **WHEN** the same caller sends two MCP requests in succession
- **THEN** the JWTs presented to Forgejo for them SHALL differ in `jti`

### Requirement: The outbound JWT has a fixed shape Forgejo accepts

The JWT's header SHALL carry:
- `alg`, determined by the signing key;
- `kid`, set to the RFC 7638 thumbprint of the signing key's public key;
- `typ`, set to `JWT`.

Its claims SHALL be exactly:
- `iss`: the value of `-forgejo-jwt-issuer`, byte for byte;
- `sub`: the `sub` of the verified inbound token;
- `aud`: a single string, the caller's Forgejo audience;
- `iat`: 60 seconds before the time of signing;
- `exp`: 300 seconds after the time of signing;
- `jti`: a random value of at least 128 bits.

The JWT SHALL NOT carry `nbf`.

#### Scenario: Claims of a minted JWT

- **WHEN** a caller whose inbound token has `sub` `388443616722288641` and Forgejo audience `u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42` makes an authenticated request, with `-forgejo-jwt-issuer` set to `https://mcp.example.org/issuer`
- **THEN** the JWT presented to Forgejo SHALL have `iss` `https://mcp.example.org/issuer`, `sub` `388443616722288641`, and `aud` the string `u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42`
- **AND** its `iat` SHALL be 60 seconds before signing and its `exp` 300 seconds after
- **AND** it SHALL carry no `nbf`

#### Scenario: Forgejo's clock is slightly behind

- **WHEN** Forgejo's clock is up to 60 seconds behind the server's clock
- **THEN** Forgejo SHALL accept the minted JWT, because its `iat` does not lie in Forgejo's future

### Requirement: The Forgejo audience comes from a claim, and its absence is refused

The server SHALL read the caller's Forgejo audience from the claim named by `-forgejo-audience-claim` (default `forgejo_aud`) in the verified inbound token, and from nowhere else.

The claim is usable only if it is present and is a single non-empty string of at most 256 characters, containing no whitespace and no control characters.

When the token is valid but its claim is not usable, the server SHALL:
- respond `403`, with a plain-text body that names the claim;
- send no `WWW-Authenticate` header, because a challenge would invite a re-authorization or scope step-up that cannot succeed;
- sign no JWT;
- make no request to Forgejo.

#### Scenario: Claim missing

- **WHEN** a request carries a valid inbound token without a `forgejo_aud` claim
- **THEN** the server SHALL respond `403` with a body naming `forgejo_aud`
- **AND** Forgejo SHALL receive no request

#### Scenario: Claim is an array

- **WHEN** a request carries a valid inbound token whose `forgejo_aud` is `["u:1:a", "u:2:b"]`
- **THEN** the server SHALL respond `403`

#### Scenario: Custom claim name

- **WHEN** `-forgejo-audience-claim` is `urn:example:forgejo` and a valid inbound token carries that claim with a usable value
- **THEN** the minted JWT's `aud` SHALL be that value

### Requirement: The signing key's type determines the algorithm

The signing key and any published keys SHALL be read from PEM files. A supported key type SHALL map to exactly one algorithm:

| Key type | Algorithm |
|---|---|
| EC P-256 | ES256 |
| EC P-384 | ES384 |
| Ed25519 | EdDSA |
| RSA with a modulus of at least 2048 bits | RS256 |

Any other key type or size SHALL prevent `resource-server` mode from starting, with a message that names the file.

#### Scenario: P-256 key

- **WHEN** the signing key file holds an EC P-256 private key
- **THEN** minted JWTs SHALL declare `alg` `ES256`

#### Scenario: Unsupported key

- **WHEN** the signing key file holds an RSA key with a 1024-bit modulus
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL name `-forgejo-jwt-signing-key-file`

### Requirement: An OpenID discovery document is published under the issuer path

The server SHALL serve a JSON discovery document, without authentication, at the path of `-forgejo-jwt-issuer` followed by `/.well-known/openid-configuration`. It SHALL contain exactly these members:
- `issuer`: equal to `-forgejo-jwt-issuer`, byte for byte;
- `jwks_uri`: equal to `-forgejo-jwt-issuer` followed by `/jwks.json`;
- `id_token_signing_alg_values_supported`: containing the signing key's algorithm.

The document SHALL advertise no endpoints. It SHALL be at most 16 KiB, SHALL be served with `Cache-Control: public, max-age=300`, and SHALL be served without redirect.

#### Scenario: Forgejo fetches discovery

- **WHEN** `-forgejo-jwt-issuer` is `https://mcp.example.org/issuer` and a request without `Authorization` asks for `/issuer/.well-known/openid-configuration`
- **THEN** the server SHALL respond `200` with `issuer` `https://mcp.example.org/issuer` and `jwks_uri` `https://mcp.example.org/issuer/jwks.json`
- **AND** the document SHALL contain no other members

### Requirement: A key set is published for verification

The server SHALL serve a JSON Web Key Set, without authentication, at the path of `-forgejo-jwt-issuer` followed by `/jwks.json`. It SHALL contain:
- the public part of the signing key;
- the public part of every key in `-forgejo-jwt-published-key-files`.

Each key SHALL carry its `kid` (its RFC 7638 thumbprint), `use` `sig` and its `alg`. The key set SHALL NOT contain private key material. It SHALL be at most 16 KiB, SHALL be served with `Cache-Control: public, max-age=300`, and SHALL be served without redirect.

#### Scenario: Private key material never published

- **WHEN** the key set is requested and a published key file contains a private key
- **THEN** the key set SHALL contain only that key's public parameters

#### Scenario: Key published ahead of use

- **WHEN** a second key is listed in `-forgejo-jwt-published-key-files`
- **THEN** the key set SHALL contain both keys, each with a distinct `kid`
- **AND** minted JWTs SHALL still be signed with the signing key only

### Requirement: The operator is told the signing key's reach at every start

At every start in `resource-server` mode, the server SHALL log a warning that states:
- the signing key's `kid`;
- the value of `-forgejo-jwt-issuer`;
- that the key is a credential for every Forgejo user whose Authorized Integration trusts that issuer.

Neither the signing key nor any JWT the server mints SHALL ever be written to any log.

#### Scenario: Startup warning

- **WHEN** the server starts in `resource-server` mode
- **THEN** the log SHALL contain one warning naming the `kid` and the issuer URL, and stating that the key acts for every user who trusts that issuer

#### Scenario: Minted JWTs are not logged

- **WHEN** debug logging is enabled and a tool call reaches Forgejo
- **THEN** the log SHALL NOT contain the minted JWT
