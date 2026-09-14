<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

forgejo-mcp can already run as a shared remote service over the `http` transport. It authenticates by forwarding the caller's `Authorization` header to Forgejo unchanged. MCP authorization is optional, so this is permitted. It is also exactly the *token passthrough* that the MCP specification forbids for any server that does authorize: "The MCP server **MUST NOT** pass through the token it received from the MCP client". Until now there was no conformant alternative, because Forgejo only accepted credentials it had issued itself.

Two things changed:

- **Forgejo 16 added Authorized Integrations.** Forgejo accepts a JWT signed by an external issuer and acts as the user who owns the integration.
- **MCP 2026-07-28 removed sessions.** forgejo-mcp already speaks that revision through mcp-go v1, so nothing per-connection has to be tied to an identity.

Together they make a conformant remote mode possible without per-user state in forgejo-mcp. The research and decisions behind this change are recorded in agentic-forges/forgejo-mcp#582.

The mode has a cost that operators must accept knowingly. forgejo-mcp holds a signing key that is a credential for every user whose integration trusts its issuer. A compromise of forgejo-mcp is a compromise of all of those users, bounded by each integration's scopes and repository selection.

## What Changes

- **New opt-in mode `--auth-mode resource-server`** (with `FORGEJO_MCP_AUTH_MODE`). The default stays `passthrough`, which is today's behaviour. `stdio` and `--cli` are untouched. Nothing existing breaks.
- **forgejo-mcp validates callers as an OAuth 2.0 resource server.**
  - Callers present a JWT access token from one configured OIDC issuer.
  - forgejo-mcp checks the signature, `iss`, `exp` and `aud` locally against the issuer's JWKS.
  - The expected audience defaults to the canonical resource URI. It can be set to another value for IdPs that cannot mint a URI audience. The value is accepted when it is contained in `aud`.
  - Opaque tokens are refused; there is no introspection.
  - Requests with no token, or with a token that fails any of these checks, receive `401` with a `WWW-Authenticate` challenge.
  - forgejo-mcp serves RFC 9728 protected resource metadata.
  - `scopes_supported` is configurable, because some IdPs refuse an authorize request without a scope.
- **forgejo-mcp signs its own outbound JWT for Forgejo and never forwards the caller's token.**
  - Each call to Forgejo carries a short-lived JWT, signed with an operator-provided asymmetric key in an algorithm Forgejo accepts.
  - Its `sub` is the caller's verified `sub`.
  - Its single `aud` is the caller's Forgejo integration audience, taken from a configurable claim in the caller's token.
  - A caller whose valid token carries no usable audience is refused at the door with `403`. forgejo-mcp never mints a token without one and never falls back to any other credential.
  - forgejo-mcp serves the OpenID discovery document and JWKS that Forgejo needs to verify these tokens. The issuer URL presented to Forgejo lives under a dedicated path of forgejo-mcp's public origin, not at the origin root. That way the origin is not mistaken for an OpenID provider. The URL is permanent once users have saved integrations.
- **No per-user state in forgejo-mcp.** There is no mapping table and no token store. The only persistent input is the signing key, supplied as configuration and rotatable.
- **Fail fast at startup.** In `resource-server` mode, forgejo-mcp refuses to start when any of these hold:
  - the configuration is incomplete;
  - the transport is `sse` or `stdio`, or `--cli` is used;
  - `--allow-operator-token-fallback` is set, or an operator token (`--token`, `FORGEJO_ACCESS_TOKEN`) is present;
  - the issuer URL is not https;
  - the signing algorithm is not one Forgejo accepts (RS256/384/512, ES256/384/512, EdDSA);
  - the configured Forgejo instance is older than 16.

  In `passthrough` mode it refuses to start when issuer-related settings are present. The startup connection test runs unauthenticated in `resource-server` mode.
- **Documentation for operators and users.** It must state plainly that the Forgejo claim rule on `sub` is the control. Forgejo accepts integrations with no rules and looks an integration up by `iss` and `aud` alone. Audiences are not secret: Forgejo documents them as safe to commit. So without a `sub` rule, any caller of forgejo-mcp's issuer who presents a user's audience acts as that user. forgejo-mcp cannot verify that the rule exists; the documentation is the control.

Out of scope, rejected on the record in #582:
- token exchange at the IdP (RFC 8693), because no IdP can mint Forgejo's per-user audience reliably;
- Forgejo's own OAuth2 provider as the issuer, because its access tokens carry no `aud`, `iss` or `sub`;
- admin `Sudo` impersonation;
- a user mapping inside forgejo-mcp;
- serving several forges from one process.

## Capabilities

### New Capabilities

- `oauth-resource-server`: the `resource-server` auth mode on the `http` transport.
  - Mode selection, and every startup fail-fast condition of the mode (including those that concern the signing key and the Forgejo version).
  - JWT access-token validation against one configured issuer.
  - Audience and issuer matching rules.
  - JWKS caching, with a bound on refetches triggered by unknown key IDs.
  - The `401` challenge and its parameters, including `scope`.
  - RFC 9728 protected resource metadata and a configurable `scopes_supported`.
- `forgejo-jwt-issuer`: how forgejo-mcp obtains a Forgejo credential in that mode.
  - Minting the outbound JWT: claims, lifetime, and time claims that tolerate Forgejo's zero clock skew.
  - The source claim for the audience, and the `403` response when that claim is absent, empty, not a string or multi-valued.
  - Key configuration and rotation.
  - Serving the OpenID discovery document and JWKS in the form Forgejo 16 Authorized Integrations requires.

### Modified Capabilities

- `stateless-http-auth`. In `resource-server` mode:
  - **Credential source.** The token-aware client factory and the raw-HTTP helper take their Forgejo credential from the issuer, never from the request's `Authorization` header. The header's value is not injected as a Forgejo token. The outbound header scheme stays `token`.
  - **A validated `401`, not a shape check.** In `passthrough` the door establishes only that a credential is present and carries a recognised scheme; the forge decides whether it is real. On the MCP endpoint in this mode the `401` is an authentication result: the token is verified against the provider's key set before anything else runs. That closes, for this mode only, the gap recorded in #588.
  - **Unauthenticated metadata routes.** The protected resource metadata, the OpenID discovery document and the JWKS are served to requests that carry no `Authorization` header. Host and Origin validation still apply to them. Every other path keeps the rule that a request without a usable credential is refused with 401 before it reaches a handler.
  `passthrough` behaviour stays as specified. The logging rules for the inbound access token and the outbound JWT are new behaviour of this mode. They are specified in `oauth-resource-server` and `forgejo-jwt-issuer`, not in this delta.

## Impact

- **Code:**
  - `cmd/cmd.go`, `cmd/auth_config.go`: flags, environment variables and their precedence.
  - `operation/authmode.go`: the startup checks, and preparing the mode before anything is bound.
  - `operation/resource_server_http.go`: exact routing, the metadata and issuer documents, and the authentication layer on `/mcp`.
  - `operation/operation.go`: mode wiring, and `requestTokenContextFunc` handing the minted token to the Forgejo clients.
  - `operation/listen.go`: the passthrough door check is skipped in this mode; Host and Origin validation still apply.
  - `pkg/forgejo/forgejo.go`, `pkg/forgejo/version.go`: the unauthenticated version probe, passed to every client.
  - New packages: `pkg/oauthrs` for inbound token validation, `pkg/jwtissuer` for outbound signing, discovery and the key set.
- **Dependencies:** one JOSE/JWT library for verification, signing and JWKS handling. `go.sum` has none today, and mcp-go v1 ships only client-side OAuth.
- **Deployment order for operators and users:**
  - The operator needs an OIDC IdP that issues JWT access tokens and can add a per-user claim holding the Forgejo audience, and Forgejo 16 or newer.
  - forgejo-mcp must already be running at its public https issuer URL, serving a non-empty JWKS, before any user can save an Authorized Integration. Forgejo validates the issuer when the integration is saved, refuses redirects, and caps each document at 16 KiB, so the reverse proxy must not redirect these paths.
  - Forgejo fetches those documents through a client that, with `[authorized_integration] ALLOWED_DOMAINS` empty, reaches public addresses only. An issuer resolving to a loopback or private address therefore needs either a non-empty `ALLOWED_DOMAINS` naming its host, or `ALLOW_LOCALNETWORKS = true`.
  - Each user then creates an integration with a `sub` claim rule and makes its audience available to forgejo-mcp.
- **Sequencing:** `network-transport-hardening` also modified `stateless-http-auth`, including the 401 rule amended above. It was archived on 2026-09-12, after #585 made its delta consistent and #586 fixed the loopback bind rule, so this change's delta is written against the current requirement text.
- **Validation before specs are written.** A spike on 2026-09-10, against Forgejo 16.0.3 with Zitadel 4.17.3 as the IdP, confirmed all five points below. The measurements are recorded in `design.md` (Context):
  1. The audience claim can be placed in the JWT **access** token.
  2. Forgejo accepts the self-signed JWT on its REST API.
  3. A wrong `sub` is rejected.
  4. Discovery and JWKS are reachable at an issuer URL with a path, and key rotation behaves as Forgejo's source indicates.
  5. Forgejo rejects a token whose `iat` lies in its future, and accepts one with `iat` omitted or backdated.

  Preconditions for Zitadel, so that a negative result is attributable:
  - a pre-registered app with JWT access tokens enabled (DCR clients receive opaque tokens);
  - an MCP client that supports a pre-registered client ID;
  - the expected-audience override, because Zitadel mints only numeric audiences.
- **Where the audience comes from** is decided in `design.md` (D5): from the IdP claim only, in this change.
  - On Zitadel 4.x this relies on the deprecated Actions v1, which Zitadel v5 removes.
  - A client-supplied audience is left to a later, separate change. It needs no IdP feature, but it makes the `sub` rule the sole authorization control.

  forgejo-mcp fails closed when the audience is missing.
