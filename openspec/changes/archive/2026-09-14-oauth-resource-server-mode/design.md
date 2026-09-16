<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Context

See `proposal.md` for why this mode exists and what it adds. The design below is shaped by the current code and by three external contracts.

**Current code.**

- **Request guard.** `operation/listen.go` owns both network listeners. `guardRequests` checks, in order, Host, then Origin, then Authorization. A request without a usable `Authorization` header gets `401` before it reaches the MCP handler. That last check reads presence and scheme, not validity: the forge decides whether a token is real, so a well-formed but invalid credential still reaches `initialize`, `tools/list` and an open event stream (#588).
- **Token into context.** `requestTokenContextFunc` in `operation/operation.go` lifts the header's token into the request context, and mcp-go hands that context to every tool handler.
- **Credential lookup.** `forgejo.Client(ctx)` and the raw-HTTP helper both resolve their credential through one function, `tokenForRequest` in `pkg/forgejo/credential.go`.
- **Version probe.** Every ephemeral SDK client probes `/api/v1/version` on construction, which costs one round trip per tool call today.
- **Routing.** Streamable HTTP is mounted at `/mcp` on a mux. Nothing else is served.

**Contract 1: Forgejo 16 Authorized Integrations.** Read from `forgejo/forgejo@50c5b98`, summarised in #582.

- **Lookup.** Forgejo finds the integration by exact `iss` and a single `aud`.
- **Signing.** A `kid` is required. The algorithm must be RS\*, ES\* or EdDSA, and it must be listed in `id_token_signing_alg_values_supported`.
- **Discovery.** It is fetched from `issuerURL.JoinPath(".well-known/openid-configuration")`, over https. `jwks_uri` must be on the same host. Redirects are refused, documents are capped at 16 KiB, and the issuer is validated when a user saves an integration.
- **Time claims.** `iat` and `nbf` are checked with zero leeway.
- **JWKS cache.** The JWKS is cached for `CACHE_TTL` (default 10 minutes). An unknown `kid` does not force a refetch.
- **Claim rules.** They are optional, and they compare string claims.

**Contract 2: MCP 2026-07-28 authorization.**

- **Metadata.** RFC 9728 metadata is mandatory. For a resource with a path it lives at `/.well-known/oauth-protected-resource/<path>`, with the root as a fallback.
- **Challenge.** The `401` challenge carries `resource_metadata`, and SHOULD carry `scope`.
- **Audience.** Tokens must be issued for the server, bound to its canonical resource URI.
- **Passthrough.** Tokens must not be passed through.

**Contract 3: the IdPs.** Many cannot mint a URI audience, many put several values in `aud`, and some mint JWT access tokens only as a per-client setting. What was measured on Zitadel 4.17.3:

- **Audience.** `aud` holds the client ID and the project ID. Every client ID in the project is added, so a second client in the same project widens the audience of both.
- **Headers.** Access tokens and ID tokens from one login carry identical headers: `alg` RS256, `typ` `JWT`, a `kid`. `typ` never says `at+jwt`.
- **ID token vs access token.** Both tokens carry the identical `aud` and a `client_id`. Only the ID token carries `amr`, `at_hash`, `auth_time`, `azp`, `nonce` and `sid`. Only the access token carries `jti` and `nbf`, plus any claim added by an Action on "Pre access token creation".
- **Lifetimes.** Token lifetimes are instance-wide settings, with no per-project or per-app override. The defaults are 12 h for access and ID tokens.

**Validated before specs were written.** A spike on 2026-09-10 against Forgejo 16.0.3 and Zitadel 4.17.3 used a static discovery document, a JWKS and a hand-signed ES256 JWT. It confirmed every assumption this design rests on:

- a custom claim from user metadata reaches the JWT access token (Actions v1, "Complement Token" → "Pre access token creation"), without affecting other clients' tokens;
- Forgejo accepts the self-signed JWT on the REST API and on Git over HTTP, and enforces the integration's scopes;
- a wrong `sub` is refused with the rule named;
- an issuer URL with a path works, and discovery and JWKS need exactly the three fields in D3;
- Forgejo fetched them without any change to its settings;
- a JWT with `iat`/`nbf` in Forgejo's future is refused, and a backdated one is accepted;
- a newly published key was accepted only once Forgejo's cache expired and it refetched lazily on the next request, 10 min 9 s later. An unknown `kid` did not trigger a refetch.

## Goals / Non-Goals

**Goals:**

- The mode is fully contained behind `--auth-mode resource-server`. In `passthrough`, the code paths and behaviour of 3.0.x are unchanged.
- Public and authenticated routes are separated **structurally**, not through an exemption list.
- forgejo-mcp holds no per-user state. Its only persistent input is key material supplied as configuration.
- Every misconfiguration that can be detected at startup refuses the start, with a message naming the setting.
- Failures reveal nothing useful. The client sees a uniform `401`, and the reason is logged at debug level.

**Non-Goals:**

- **Token handling.** No token introspection, no opaque tokens, and no more than one IdP issuer per process.
- **Authorization-server duties.** forgejo-mcp does not act as an authorization server: no DCR, no CIMD, no authorize or token endpoints.
- **Per-tool authorization.** No per-tool scopes or authorization. A valid token authorises every tool, and Forgejo's integration scopes are the actual permission boundary.
- **Token lifetimes and state.** forgejo-mcp does not shorten inbound token lifetimes; the IdP owns them, see Risks. It also does not cache minted Forgejo tokens, handle refresh tokens or keep sessions.
- **Key storage.** No KMS or HSM. Keys come from files, and the file boundary is where a later change can plug in something stronger.
- **Transports.** No SSE, stdio or `--cli` in this mode.
- **Audience source.** No client-supplied Forgejo audience (see D5).

## Decisions

### D1: One JOSE library: `github.com/lestrrat-go/jwx/v3`

This change needs four things:

1. JWT verification that takes the algorithm from the key, not the token header;
2. a JWKS fetcher with a cache and a refresh floor;
3. JWT signing;
4. JWK export and RFC 7638 thumbprints.

jwx v3 (MIT, v3.3.0, 2026-09-08) covers all four, so none of the security-critical pieces is hand-written.

*Alternatives:*

- **`golang-jwt/jwt/v5` plus a hand-written JWKS cache.** It matches Forgejo's own parser, but the refetch bound and the key-type handling would be ours to get right.
- **`golang-jwt` plus `MicahParks/keyfunc`.** Two dependencies for the same result.
- **`go-jose/v4`.** Lower level, with no JWKS fetch or cache.

MIT and Apache-2.0 are both compatible with GPL-3.0-or-later. The transitive dependency graph is reviewed when the dependency is added.

### D2: Configuration surface

Every option is a flag with an environment variable. The precedence is the one in `cmd/cmd.go` today: a flag that was actually passed (`flagWasPassed`), then the environment variable, then the default.

| Flag | Env | Meaning | Default |
|---|---|---|---|
| `-auth-mode` | `FORGEJO_MCP_AUTH_MODE` | `passthrough` or `resource-server` | `passthrough` |
| `-authorization-server` | `FORGEJO_MCP_AUTHORIZATION_SERVER` | IdP issuer URL; published in `authorization_servers` | – |
| `-resource` | `FORGEJO_MCP_RESOURCE` | Canonical resource URI of the MCP endpoint, e.g. `https://mcp.example.org/mcp`; its path must be `/mcp` | – |
| `-resource-audience` | `FORGEJO_MCP_RESOURCE_AUDIENCE` | Value that must be contained in an inbound `aud` | value of `-resource` |
| `-scopes-supported` | `FORGEJO_MCP_SCOPES_SUPPORTED` | Space-separated scopes for metadata and challenge | unset: neither field emitted |
| `-forgejo-audience-claim` | `FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM` | Inbound claim holding the caller's Forgejo integration audience | `forgejo_aud` |
| `-forgejo-jwt-issuer` | `FORGEJO_MCP_FORGEJO_JWT_ISSUER` | Issuer URL forgejo-mcp presents to Forgejo, e.g. `https://mcp.example.org/issuer` | – |
| `-forgejo-jwt-signing-key-file` | `FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE` | PEM private key used to sign | – |
| `-forgejo-jwt-published-key-files` | `FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES` | Comma-separated PEM public or private keys published in the JWKS in addition to the signing key | unset |

- **Key files.** Keys are read from files, not from environment values. Files fit systemd `LoadCredential=`, sops-rendered secrets and Kubernetes secret mounts, and they keep PEM out of process listings and unit text.
- **Key IDs.** A key's `kid` is its RFC 7638 thumbprint, so it needs no configuration and cannot collide.
- **Algorithm.** The algorithm follows from the key type:
  - EC P-256 → ES256
  - EC P-384 → ES384
  - Ed25519 → EdDSA
  - RSA of at least 2048 bits → RS256

  Any other key type refuses the start.

*Alternative:* a single `-issuer` meaning both the IdP and forgejo-mcp's own issuer. Rejected: they are different parties with different trust, and conflating them was the most likely misconfiguration.

### D3: Routes, and how the guard treats them

Every route sits behind the existing Host and Origin checks. The routes split into two groups.

**Public group.** These routes never require a token.

- **Protected resource metadata** (RFC 9728), derived from the path of `-resource`. With `-resource https://h/mcp` it is served at both `/.well-known/oauth-protected-resource/mcp` and `/.well-known/oauth-protected-resource`, because MCP clients try them in that order.
- **OpenID discovery document**, derived from the path of `-forgejo-jwt-issuer`. With `-forgejo-jwt-issuer https://h/issuer` it is served at `/issuer/.well-known/openid-configuration`.
- **JWKS** at `/issuer/jwks.json`.

**Protected group.** The MCP endpoint, wrapped in the authentication layer of D4.

**The guard.** Authentication becomes a layer on the protected group only. It is no longer a final check that every path passes through. This follows jmap-mcp: "the document a caller reads to discover how to authorise cannot itself require authorization; saying so structurally beats an exemption list".

- **Unmatched paths.** Any other path gets `404`. Nothing is served at the origin root's `/.well-known/openid-configuration`.
- **Exact paths.** Routes are registered as exact paths only, never as subtree patterns ending in `/`. `ServeMux` redirects a request to a matching subtree pattern, and Forgejo refuses redirects. With exact paths, a mistyped request gets `404` instead.
- **Discovery document.** It contains exactly `issuer`, `jwks_uri` and `id_token_signing_alg_values_supported`. It advertises no endpoints, because forgejo-mcp is not an authorization server. The spike confirmed that Forgejo needs no more.
- **Cache headers.** Both documents are served with `Cache-Control: public, max-age=300`.

*Alternative:* one guard with an allow-list of public paths. Rejected: every future public route becomes a place to forget the exemption, or to widen it by accident.

### D4: Inbound validation

**At startup.**

- Fetch the IdP's discovery document: OIDC `/.well-known/openid-configuration` first, then the RFC 8414 location.
- Require `issuer` to equal `-authorization-server` byte for byte, and require a `jwks_uri`.
- An unreachable IdP is a startup error. Checking it once per request would turn one clear failure into a stream of confusing ones.

**Per request.**

1. **Header.** `Authorization: Bearer <jwt>` is required. The scheme is matched case-insensitively. `token` is not accepted in this mode.
2. **Signature.** Verify it with the key named by `kid`, from a JWKS cache with a refresh floor of 5 minutes. An unknown `kid` triggers at most one refetch per floor interval, so a stranger sending random key IDs cannot make forgejo-mcp hammer the IdP. The refetch is detached from the request that triggered it, so a caller that disconnects cannot fail it and still spend the interval.
3. **Algorithm.** Take it from the JWK, and accept only asymmetric algorithms. Zitadel signs with RS256.
4. **Issuer and time claims.** `iss` must match exactly, and `exp` is required. `nbf` and `iat` are honoured with 60 s of leeway; Forgejo's zero leeway applies to its own checks, not ours.
5. **Audience.** `aud` must contain `-resource-audience`, whether `aud` arrives as a string or as an array.
6. **ID tokens are refused.** A token that carries `nonce` or `at_hash` is rejected.
   - OIDC defines both claims for ID tokens only, so the rule works across IdPs.
   - On Zitadel it is required: an ID token carries the identical `aud` and `client_id`, and its `typ` header is identical to the access token's.
   - The JOSE `typ` header is therefore not used to tell the two apart. If present, it must be `at+jwt` or `JWT`, and anything else is refused.

**Refusals.**

- **Missing or invalid token.** Answered with `401` and an empty body.
  - The challenge is `WWW-Authenticate: Bearer resource_metadata="<url>"`.
  - It adds `scope="<scopes>"` when scopes are configured, and `error="invalid_token"` when a token was presented.
  - The reason is logged at debug level, rate-limited through the existing `logRefusal`.
- **Valid token, no usable Forgejo audience.** Answered with `403` and a plain-text body naming the claim.
  - "Usable" means the claim is present and is a single non-empty string of at most 256 characters, without whitespace or control characters.
  - The response carries no `WWW-Authenticate` header at all. An `insufficient_scope` hint would send spec-following clients into a step-up authorization loop that cannot succeed, because the fix is data in the IdP, not a scope.

### D5: Where the Forgejo audience comes from: the IdP claim only, in this change

The per-user audience is read only from the claim named by `-forgejo-audience-claim`, in the verified inbound token. A client-supplied audience, for example in a header, is **not** part of this change.

*Why decide it now.* A second source changes the specs. It needs its own request contract, and its own documentation of the stronger reliance on the `sub` rule.

*Why the claim.*

- The value arrives inside a token forgejo-mcp has verified, so a compromised client cannot swap it freely.
- It needs no extra client configuration.
- The spike proved it on Zitadel 4.17.3.

*Alternative kept for later:* a client-supplied audience, as a separate change. It is the fallback if an IdP cannot add a claim, or when Zitadel v5 removes Actions v1.

### D6: Outbound token and how it reaches the Forgejo client

In the auth layer, after D4 succeeds, forgejo-mcp mints a JWT.

**Header.**

- `alg` from the signing key
- `kid` = the key's thumbprint
- `typ` = `JWT`

**Claims.**

- `iss` = `-forgejo-jwt-issuer`, exactly
- `sub` = the inbound `sub`
- `aud` = the single audience string from D5
- `iat` = now − 60 s
- `exp` = now + 300 s
- `jti` = a random 128-bit value
- no `nbf`

Backdating `iat` and omitting `nbf` absorbs clock drift in the direction Forgejo rejects, and `exp` stays short. The spike confirmed that Forgejo refuses a token whose `iat` or `nbf` lies in its future, and accepts one backdated by 60 s.

**How the token reaches the Forgejo client.**

- **Context.** The auth layer stores the minted token in the HTTP request's context. In `resource-server` mode, `requestTokenContextFunc` injects that value with `forgejo.WithToken` and never reads the `Authorization` header. In `passthrough` it keeps reading the header, as it does today. The inbound token therefore never enters the context that tool handlers see.
- **Downstream.** `forgejo.Client(ctx)` and the raw-HTTP helper keep resolving their credential through `tokenForRequest`. The SDK keeps sending `Authorization: token <jwt>`, which Forgejo accepts.

**One token per request.** Exactly one token is minted per HTTP request, and it is not cached. An ES256 signature costs microseconds, and a cache would be per-user state.

*Alternative:* mint lazily inside `tokenForRequest`, only when a tool actually calls Forgejo. Rejected: minting per request is simpler to reason about, and every refusal, including the `403`, happens at the door before any MCP handling.

### D7: Startup sequence and the version floor

In `resource-server` mode, `operation.Run` runs these checks before binding, in this order, and refuses on the first failure.

1. **Mode and transport.** The mode value is valid, and the transport is `http`. `--cli`, `stdio` and `sse` refuse.
2. **No legacy credentials.** No operator token and no fallback flag.
3. **Required settings.** `-authorization-server`, `-resource`, `-forgejo-jwt-issuer` and the signing key file are all set.
   - `-forgejo-jwt-issuer` must be https, because Forgejo requires it. It must also not end with `/` or carry a query, a fragment or user information: `…/issuer/` would publish `…/issuer//jwks.json`, and Forgejo compares `iss` byte for byte.
   - `-authorization-server` and `-resource` must be https, or http on a loopback host only, for local development.
   - The hosts of `-resource` and `-forgejo-jwt-issuer` must be permitted by `-allowed-hosts`, or be loopback on a loopback bind. Otherwise the guard would refuse the very requests those routes exist for.
   - The path of `-resource` must be `/mcp`, where the endpoint is served. The metadata publishes `-resource` as the resource, and `-resource` is also the default inbound audience, so any other path starts cleanly and then names a resource the server does not serve.
4. **Keys.** The signing key loads and has a supported type. Published keys load, with no duplicate `kid`.
5. **Forgejo version.** `GET /api/v1/version`, unauthenticated, must report 16.0 or newer.
   - The reported version is kept and passed to every ephemeral SDK client as `SetForgejoVersion`.
   - In this mode that removes today's per-client version probe, which would otherwise hit Forgejo with a JWT once per tool call.
6. **IdP discovery**, as in D4.

**In `passthrough` mode**, startup refuses when any flag from D2 other than `-auth-mode` is set.

**The startup connection test.** In `resource-server` mode it is replaced by check 5. It no longer calls `forgejo.Client(context.Background())`, which would reach for an operator token that does not exist.

### D8: Logging

- **Tokens.** The inbound token and the minted token are never logged. The existing "never log the token" rule extends to both.
- **Allowed in logs.**
  - `sub` and the Forgejo audience, at debug level only, for diagnosing refusals.
  - The thumbprint `kid`, the IdP issuer URL and forgejo-mcp's issuer URL, at startup.
- **Startup warning.** One warning at every start states that the signing key is a credential for every user whose Forgejo integration trusts `-forgejo-jwt-issuer`. This mirrors how the fallback flag is announced today.

## Risks / Trade-offs

- **[Key compromise reaches every user who trusts the issuer]**
  - Keep `exp` short.
  - Require a `sub` rule in each integration.
  - Integration owners scope their integrations narrowly.
  - Supply keys as files with tight permissions.
  - The startup warning (D8) states the blast radius.
  - Rotation needs no downtime (Migration Plan), so a suspected leak is cheap to answer.
- **[A missing `sub` rule makes audiences spoofable]** A caller could set another user's audience in their own IdP profile. forgejo-mcp cannot list integrations, so it cannot detect a missing rule.
  - The operator and user documentation treat the rule as mandatory.
  - Where the IdP allows it, the documentation recommends that the attribute holding the audience be writable by administrators only.
- **[An ID token accepted as an access token]** Measured on Zitadel: an ID token carries the same `aud`, `client_id` and headers as an access token.
  - D4 step 6 refuses any token with `nonce` or `at_hash`.
  - The fail-closed audience claim happens to refuse today's Zitadel ID tokens as well, because the Action is bound only to access-token creation. That is a coincidence of one deployment's configuration and is not relied on.
- **[A widened inbound audience]** On Zitadel every client ID of a project lands in `aud`.
  - Deployment guidance: give each forgejo-mcp deployment a dedicated IdP project, and gate it with a role grant.
  - Set `-resource-audience` to the MCP client's own client ID if only that client should be accepted, or to the project ID to accept every client in the project.
- **[Long-lived inbound tokens]** The IdP decides how long a stolen access token grants access to forgejo-mcp. On Zitadel the default is 12 h, instance-wide, with no per-project override.
  - Documented as an operator decision.
  - A maximum-token-age check in forgejo-mcp is a possible later change; it is a Non-Goal here.
- **[Clock drift between forgejo-mcp and Forgejo]** `iat` is backdated and `nbf` omitted (D6). Both directions were confirmed in the spike.
- **[Forgejo's JWKS cache ignores new key IDs]** Rotation publishes a new key before signing with it (Migration Plan). The spike measured a lazy refetch on the first request after `CACHE_TTL` expired, and no refetch on an unknown `kid`.
- **[Zitadel removes Actions v1 in v5]** On Zitadel, D5 depends on a deprecated feature.
  - The chiba deployment records a decision gate before any Zitadel v5 upgrade.
  - A client-supplied audience, or an Actions v2 target, is a later and separate change. forgejo-mcp does not embed an IdP-specific webhook.
- **[New dependency surface]** jwx v3 and its transitive dependencies. They are reviewed when added, pinned by `go.sum`, and covered by the existing Renovate and OSV scanning.
- **[Metadata routes exposed to strangers]** They are static, small and cache-controlled, sit behind the Host and Origin checks, and cost no IdP or Forgejo call. They add no refusal path that could write unbounded log lines.

## Migration Plan

**Deploy (operator).**

1. Deploy forgejo-mcp in `resource-server` mode at its public origin, behind a proxy that does not redirect the metadata paths.
2. Verify that discovery and JWKS answer at the configured issuer path.
3. Only then can users create integrations, because Forgejo validates the issuer when an integration is saved.

**Onboard a user.**

1. Create a Generic JWT Authorized Integration with issuer `-forgejo-jwt-issuer`, a claim rule `sub eq <IdP subject>`, and narrow permissions.
2. Copy the generated audience into the IdP attribute the claim is built from.

**Rotate the key.**

1. Add the new key to `-forgejo-jwt-published-key-files` and restart.
2. Wait at least Forgejo's `CACHE_TTL` plus a margin: 15 minutes with the default of 10. Forgejo refetches lazily, on the first request after the cache expires.
3. Make the new key the signing key, keeping the old one published.
4. After the token lifetime has passed (5 minutes), remove the old key and restart.

**Rollback.** Set `-auth-mode passthrough` and remove the D2 flags. Users' integrations stay in Forgejo, unused. Previously minted tokens expire within 5 minutes.

**Upstream sequencing.** `network-transport-hardening` was archived on 2026-09-12, so the `stateless-http-auth` delta is written against the current requirement text — including the paragraph #585 added, which states that the door check is presence-and-shape rather than validation.

## Open Questions

None remain open. The two raised while planning are settled:

- **Refresh floor for the IdP JWKS cache:** 5 minutes, as proposed (`DefaultMinRefreshInterval` in `pkg/oauthrs/keyset.go`). It can still be tuned without touching specs or tasks.
- **A startup warning for RSA keys below 3072 bits:** not added. RSA keys are accepted from 2048 bits without comment; the operator guide's example generates an EC P-256 key.
