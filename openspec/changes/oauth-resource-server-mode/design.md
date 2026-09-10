<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Context

See `proposal.md` for why this mode exists and what it adds. The design below is shaped by the current code and by three external contracts.

**Current code.**
- **Request guard.** `operation/listen.go` owns both network listeners. `guardRequests` checks, in order, Host, then Origin, then Authorization. A request without a usable `Authorization` header gets `401` before it reaches the MCP handler.
- **Token into context.** `requestTokenContextFunc` in `operation/operation.go` lifts the header's token into the request context, and mcp-go hands that context to every tool handler.
- **Credential lookup.** `forgejo.Client(ctx)` and the raw-HTTP helper both resolve their credential through one function, `tokenForRequest` in `pkg/forgejo/credential.go`.
- **Server version probe.** Every ephemeral SDK client probes `/api/v1/version` on construction, which costs one round trip per tool call today.
- **Streamable HTTP mount.** It is mounted at `/mcp` on a mux. Nothing else is served.

**Contract 1: Forgejo 16 Authorized Integrations.** Read from `forgejo/forgejo@50c5b98`, summarised in #582.
- **Integration lookup.** Forgejo finds the integration by exact `iss` and a single `aud`.
- **Signing.** A `kid` is required, and the algorithm must be RS\*, ES\* or EdDSA. The discovery document must list that algorithm in `id_token_signing_alg_values_supported`.
- **Discovery location.** Discovery is fetched from `issuerURL.JoinPath(".well-known/openid-configuration")`, over https. The `jwks_uri` must be on the same host. Redirects are refused, documents are capped at 16 KiB, and the whole issuer is validated when a user saves an integration.
- **Time claims.** `iat` and `nbf` are checked with zero leeway.
- **JWKS cache.** The JWKS is cached for `CACHE_TTL` (default 10 minutes). An unknown `kid` does not force a refetch.
- **Claim rules.** Rules are optional and compare string claims.

**Contract 2: MCP 2026-07-28 authorization.**
- **Metadata.** RFC 9728 metadata is mandatory. For a resource with a path it lives at `/.well-known/oauth-protected-resource/<path>`, with the root as fallback.
- **`401` challenge.** The `401` challenge carries `resource_metadata` and SHOULD carry `scope`.
- **Audience.** Tokens must be issued for the server, bound to its canonical resource URI.
- **No passthrough.** Tokens must not be passed through.

**Contract 3: the IdPs.** Many cannot mint a URI audience; Zitadel, for example, mints numeric IDs. Many put several values in `aud`, and some mint JWT access tokens only per client setting.

## Goals / Non-Goals

**Goals:**
- The mode is fully contained behind `--auth-mode resource-server`. In `passthrough`, the code paths and behaviour of 3.0.x are unchanged.
- Public routes are separated from authenticated ones **structurally**, not through an exemption list.
- forgejo-mcp holds no per-user state. Its only persistent input is key material supplied as configuration.
- Every misconfiguration that can be detected at startup refuses the start, with a message naming the setting.
- Failures reveal nothing useful. The client sees a uniform `401`; the reason goes to the log at debug level.

**Non-Goals:**
- Token introspection, opaque tokens, or more than one IdP issuer per process.
- Acting as an authorization server: no DCR, no CIMD, no authorize or token endpoints.
- Per-tool scopes or authorization. A valid token authorises every tool, and Forgejo's integration scopes are the actual permission boundary.
- Caching minted Forgejo tokens, handling refresh tokens, or keeping sessions.
- KMS or HSM key storage. Keys come from files. The file boundary is where a later change can plug in something stronger.
- SSE, stdio and `--cli` in this mode.
- A client-supplied Forgejo audience (see D5).

## Decisions

### D1: One JOSE library: `github.com/lestrrat-go/jwx/v3`

This change needs four things:
- JWT verification that takes the algorithm from the key, not from the token header;
- a JWKS fetcher with a cache and a refresh floor;
- JWT signing;
- JWK export and RFC 7638 thumbprints.

jwx v3 (MIT, v3.3.0, 2026-09-08) covers all four, so the security-critical pieces are not hand-written.

*Alternatives considered:*
- **`golang-jwt/jwt/v5` plus a hand-written JWKS cache.** It matches Forgejo's own parser, but the refetch bound and key-type handling would be ours to get right.
- **`golang-jwt` plus `MicahParks/keyfunc`.** Two dependencies for the same result.
- **`go-jose/v4`.** Lower level, with no JWKS fetch or cache.

Both MIT and Apache-2.0 are compatible with GPL-3.0-or-later. The transitive dependency graph is reviewed when the dependency is added.

### D2: Configuration surface

Every option is a flag with an environment variable. Precedence works as in `cmd/cmd.go` today: a flag counts only if it was actually passed (`flagWasPassed`), otherwise the environment variable, otherwise the default.

| Flag | Env | Meaning | Default |
|---|---|---|---|
| `-auth-mode` | `FORGEJO_MCP_AUTH_MODE` | `passthrough` or `resource-server` | `passthrough` |
| `-authorization-server` | `FORGEJO_MCP_AUTHORIZATION_SERVER` | IdP issuer URL; published in `authorization_servers` | – |
| `-resource` | `FORGEJO_MCP_RESOURCE` | Canonical resource URI of the MCP endpoint, e.g. `https://mcp.example.org/mcp` | – |
| `-resource-audience` | `FORGEJO_MCP_RESOURCE_AUDIENCE` | Value that must be contained in an inbound `aud` | value of `-resource` |
| `-scopes-supported` | `FORGEJO_MCP_SCOPES_SUPPORTED` | Space-separated scopes for metadata and challenge | unset: neither field emitted |
| `-forgejo-audience-claim` | `FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM` | Inbound claim holding the caller's Forgejo integration audience | `forgejo_aud` |
| `-forgejo-jwt-issuer` | `FORGEJO_MCP_FORGEJO_JWT_ISSUER` | Issuer URL forgejo-mcp presents to Forgejo, e.g. `https://mcp.example.org/issuer` | – |
| `-forgejo-jwt-signing-key-file` | `FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE` | PEM private key used to sign | – |
| `-forgejo-jwt-published-key-files` | `FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES` | Comma-separated PEM public or private keys published in the JWKS in addition to the signing key | unset |

- **Keys are read from files, not environment values.** A file fits systemd `LoadCredential=`, sops-rendered secrets and Kubernetes secret mounts, and it keeps PEM out of process listings and unit text.
- **Key IDs.** `kid` is the RFC 7638 thumbprint of the public key, so it needs no configuration and cannot collide.
- **Algorithm.** It follows from the key type: EC P-256 means ES256, P-384 means ES384, Ed25519 means EdDSA, and RSA of at least 2048 bits means RS256. Any other key type refuses the start.

*Alternative:* a single `-issuer` meaning both the IdP and forgejo-mcp's own issuer. Rejected, because the two are different parties with different trust. One name for both was the most likely misconfiguration.

### D3: Routes, and how the guard treats them

Every route sits behind the existing Host and Origin checks. The routes split into two groups.

**Public group.** These routes never require a token:
- **Protected resource metadata** (RFC 9728), derived from the path of `-resource`. With `-resource https://h/mcp` it is served at both `/.well-known/oauth-protected-resource/mcp` and `/.well-known/oauth-protected-resource`, because MCP clients try those two locations in that order.
- **OpenID discovery document**, derived from the path of `-forgejo-jwt-issuer`. With `-forgejo-jwt-issuer https://h/issuer` it is served at `/issuer/.well-known/openid-configuration`.
- **JWKS** at `/issuer/jwks.json`.

**Protected group.** This is the MCP endpoint, wrapped in the authentication layer of D4.

The guard is restructured so that authentication is a layer on the protected group only. It is no longer a final check that every path passes through. This follows jmap-mcp's design: "the document a caller reads to discover how to authorise cannot itself require authorization; saying so structurally beats an exemption list". Any path that matches neither group gets `404`. Nothing is served at the origin root's `/.well-known/openid-configuration`.

- **Exact paths.** Routes are registered as exact paths only, never as subtree patterns ending in `/`. `ServeMux` redirects a request to a matching subtree pattern, Forgejo refuses redirects, and with exact paths a mistyped request gets `404` instead.
- **Discovery document contents.** Exactly `issuer`, `jwks_uri` and `id_token_signing_alg_values_supported`. It advertises no endpoints, because forgejo-mcp is not an authorization server; see Non-Goals.
- **Cache headers.** Both documents are served with `Cache-Control: public, max-age=300`.

*Alternative:* keep one guard and add an allow-list of public paths. Rejected, because every future public route becomes a place to forget the exemption, or to widen it by accident.

### D4: Inbound validation

**At startup:**
- Fetch the IdP's discovery document: OIDC `/.well-known/openid-configuration` first, then the RFC 8414 location.
- Require `issuer` to equal `-authorization-server` byte for byte, and require a `jwks_uri`.
- An unreachable IdP is a startup error. Checking it once per request would turn one clear failure into a stream of confusing ones.

**Per request:**
1. `Authorization: Bearer <jwt>` is required. The scheme is case-insensitive; `token` is not accepted in this mode.
2. Verify the signature with the key named by `kid`, from a JWKS cache with a refresh floor of 5 minutes. An unknown `kid` triggers at most one refetch per floor interval, so a stranger sending random key IDs cannot make forgejo-mcp hammer the IdP.
3. Take the algorithm from the JWK. Accept only asymmetric algorithms.
4. `iss` must match exactly. `exp` is required. `nbf` and `iat` are honoured with 60 s of leeway; the zero-leeway constraint is Forgejo's, not ours.
5. `aud` must contain `-resource-audience`. Whether it arrives as a string or an array does not matter.
6. If the JOSE header `typ` is present, it must be `at+jwt` (RFC 9068) or `JWT`. Anything else is refused.

**Failure responses:**
- **No token or invalid token:** `401` with an empty body and `WWW-Authenticate: Bearer resource_metadata="<url>"`. The challenge adds `scope="<scopes>"` when configured, and `error="invalid_token"` when a token was presented. The reason is logged at debug level, rate-limited through the existing `logRefusal`.
- **Valid token, no usable Forgejo audience:** `403` with a plain-text body naming the claim, and no `WWW-Authenticate` error parameter. "Usable" means the claim is present, is a single non-empty string of at most 256 characters, and contains no whitespace or control characters. An `insufficient_scope` hint would send spec-following clients into a step-up authorization loop that can never succeed, because the fix is data in the IdP, not a scope.

### D5: Where the Forgejo audience comes from: the IdP claim only, in this change

The per-user audience is read only from the claim named by `-forgejo-audience-claim` in the verified inbound token. A client-supplied audience, for example a header, is **not** part of this change.

*Why decide it now:* a second source changes the specs. It would need a new request contract and its own documentation of the stronger reliance on the `sub` rule. So it cannot stay an open question.

*Why the claim:*
- **Verified.** The value arrives inside a token forgejo-mcp has verified, so a compromised client cannot swap it freely.
- **No extra configuration.** It needs no additional client configuration.
- **Proven on the spike IdP.** It works on the spike's IdP: Zitadel 4.17.3, Actions v1 "Complement Token".

*Alternative kept for later:* a client-supplied audience, as a separate change. It becomes the fallback if an IdP cannot add a claim, or when Zitadel v5 removes Actions v1.

### D6: Outbound token and how it reaches the Forgejo client

In the auth layer, after D4 succeeds, forgejo-mcp mints a JWT:
- **Header:** `alg` from the signing key, `kid` = the key's thumbprint, `typ` `JWT`.
- **Claims:**
  - `iss` = `-forgejo-jwt-issuer`, exactly;
  - `sub` = the inbound `sub`;
  - `aud` = the single audience string from D5;
  - `iat` = now − 60 s;
  - `exp` = now + 300 s;
  - `jti` = a random 128-bit value.
- **No `nbf`.**

Backdating `iat` and omitting `nbf` absorbs clock drift in the direction Forgejo rejects. `exp` stays short.

- **Into the request context.** The auth layer stores the minted token in the HTTP request's context. In `resource-server` mode, `requestTokenContextFunc` injects that value with `forgejo.WithToken` and never reads the `Authorization` header; in `passthrough` it keeps reading the header as today. The inbound token therefore never enters the context that tool handlers see.
- **Nothing downstream changes.** `forgejo.Client(ctx)` and the raw-HTTP helper keep resolving through `tokenForRequest`, and the SDK keeps sending `Authorization: token <jwt>`, which Forgejo accepts.

One token is minted per HTTP request, with no cache. An ES256 signature costs microseconds, and a cache would be per-user state.

*Alternative:* mint lazily inside `tokenForRequest`, only when a tool actually calls Forgejo. Rejected. Minting per request is simpler to reason about, because every refusal, including the `403`, happens at the door before any MCP handling.

### D7: Startup sequence and the version floor

In `resource-server` mode, `operation.Run` performs these checks before binding, in this order, and refuses on the first failure:
1. **Mode and transport.** The mode value is valid, and the transport is `http`. `--cli`, `stdio` and `sse` refuse.
2. **Unwanted credentials.** No operator token and no fallback flag.
3. **Required settings.** `-authorization-server`, `-resource`, `-forgejo-jwt-issuer` and the signing key file are all set.
   - `-forgejo-jwt-issuer` is https; Forgejo requires it.
   - `-authorization-server` and `-resource` are https, or http on a loopback host only, for local development.
   - The hosts of `-resource` and `-forgejo-jwt-issuer` are permitted by `-allowed-hosts`, or are loopback on a loopback bind. Otherwise the guard would refuse the very requests the routes exist for.
4. **Keys.** The signing key loads and has a supported type. Published keys load, with no duplicate `kid`.
5. **Forgejo version.** `GET /api/v1/version`, unauthenticated, must report 16.0 or newer. The reported version is kept and passed to every ephemeral SDK client as `SetForgejoVersion`. That removes today's per-client version probe in this mode, which would otherwise hit Forgejo with a JWT once per tool call.
6. **IdP discovery** as in D4.

In `passthrough` mode, startup refuses when any flag from D2 other than `-auth-mode` is set. Today's startup connection test is replaced in `resource-server` mode by check 5; it no longer calls `forgejo.Client(context.Background())`, which would reach for an operator token that does not exist.

### D8: Logging

- **Never logged:** the inbound token and the minted token. The existing "never log the token" rule extends to both.
- **Allowed in logs:**
  - at debug level, `sub` and the Forgejo audience, for diagnosing refusals;
  - the thumbprint `kid` and the IdP and forgejo-mcp issuer URLs, at startup.
- **Startup warning:** one warning at every start, naming the blast radius. It says that the signing key is a credential for every user whose Forgejo integration trusts `-forgejo-jwt-issuer`. This mirrors how the fallback flag is announced today.

## Risks / Trade-offs

- **[Key compromise reaches every trusting user]**
  - Short `exp`.
  - A `sub` rule in each integration.
  - Integrations scoped narrowly by their owners.
  - Keys supplied as files with tight permissions.
  - The startup warning (D8).
  - Rotation without downtime (Migration Plan), so a suspected leak is cheap to answer.
- **[A missing `sub` rule makes audiences spoofable]** A caller could set another user's audience in their own IdP profile. forgejo-mcp cannot list integrations, so it cannot detect a missing rule.
  - The operator and user documentation treat the rule as mandatory.
  - It recommends that the IdP attribute holding the audience be writable by administrators only, where the IdP allows that.
- **[An ID token accepted as an access token]** When `-resource-audience` is a client ID, as it tends to be on Zitadel, an ID token for that client passes the audience check.
  - The `typ` check in D4 rejects anything that is not declared as an access token.
  - The spike records the `typ` header and claims of a Zitadel access token versus an ID token. If they are indistinguishable, tasks add a documented rule, e.g. rejecting tokens that carry `nonce` or `at_hash`.
- **[Clock drift between forgejo-mcp and Forgejo]** Backdated `iat`, no `nbf` (D6). Spike check 5 proves both directions.
- **[Forgejo's JWKS cache ignores new key IDs]** Rotation publishes a new key before signing with it (Migration Plan). Nothing else depends on Forgejo refetching.
- **[Zitadel removes Actions v1 in v5]** On Zitadel, D5 depends on a deprecated feature.
  - The chiba deployment records a decision gate before any Zitadel v5 upgrade.
  - A client-supplied audience or an Actions v2 target is a later, separate change. forgejo-mcp does not embed an IdP-specific webhook.
- **[New dependency surface]** jwx v3 and its transitive dependencies. Reviewed at addition, pinned by `go.sum`, and covered by the existing Renovate and OSV scanning.
- **[Metadata routes exposed to strangers]** They are static, small and cache-controlled, behind the Host and Origin checks, and cost no IdP or Forgejo call. They add no refusal path that could write unbounded log lines.

## Migration Plan

**Deploy (operator).**
1. Deploy forgejo-mcp in `resource-server` mode at its public origin, behind a proxy that does not redirect the metadata paths.
2. Verify that discovery and JWKS answer at the configured issuer path.
3. Only then can users create integrations, because Forgejo validates the issuer on save.

**Onboard a user.**
1. Create a Generic JWT Authorized Integration with issuer `-forgejo-jwt-issuer`, a claim rule `sub eq <IdP subject>`, and narrow permissions.
2. Copy the generated audience into the IdP attribute that the claim is built from.

**Rotate the key.**
1. Add the new key to `-forgejo-jwt-published-key-files` and restart.
2. Wait at least Forgejo's `CACHE_TTL` (10 minutes by default).
3. Make the new key the signing key, keeping the old one published.
4. After the token lifetime has passed (5 minutes), remove the old key and restart.

**Rollback.** Set `-auth-mode passthrough` and remove the D2 flags. Users' integrations stay in Forgejo, unused. Tokens previously minted expire within 5 minutes.

**Upstream sequencing.** `network-transport-hardening` is archived before this change's specs are written, so the `stateless-http-auth` delta is written against the current requirement text.

## Open Questions

- **Refresh floor.** The exact refresh floor for the IdP JWKS cache (5 minutes proposed). It can be tuned without touching specs or tasks.
- **Minimum RSA key size.** Whether RSA keys below 3072 bits should draw a startup warning, in addition to being accepted from 2048 bits.
