<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

The planning artifacts and the implementation land together in one pull request. It keeps `passthrough` behaviour unchanged and CI green. Specs and design are the reference: `specs/oauth-resource-server/spec.md`, `specs/forgejo-jwt-issuer/spec.md` and `design.md` (D1–D8).

## 1. Prerequisites

- [x] 1.1 Get `openspec/changes/network-transport-hardening/` archived, either by the maintainer or by a small PR running `openspec archive network-transport-hardening`. Verify on `main`: `openspec/specs/network-transport-binding/spec.md` exists, and `openspec list` no longer shows the change. **Done 2026-09-12:** the maintainer archived it after #585 made its delta consistent and #586 fixed the loopback bind rule.
- [x] 1.2 Rebase this branch on `main`, then write `specs/stateless-http-auth/spec.md` as a delta. It covers two things: the credential source in `resource-server` mode, and unauthenticated access to the metadata, discovery and JWKS routes, while every other path keeps the 401-at-the-door rule. Verify with `openspec validate oauth-resource-server-mode --strict`, and by archiving into a scratch copy: `openspec validate --all --strict` must stay green and the resulting `stateless-http-auth` spec must keep every existing scenario.
- [x] 1.3 Add `github.com/lestrrat-go/jwx/v3` and review its transitive dependencies for licence compatibility with GPL-3.0-or-later. Verify that `make vendor` leaves `go.mod`/`go.sum` tidy and `make build` succeeds.

## 2. Configuration and keys

- [x] 2.1 Add the flags and environment variables from design D2 to `cmd/cmd.go` and `pkg/flag`, using the existing `flagWasPassed` precedence. Verify with tests in `cmd/auth_config_test.go`: every default; env-only; flag over env, including a flag explicitly set to its default value.
- [x] 2.2 Refuse to start in `passthrough` mode when any setting that applies only to `resource-server` mode is present. Verify with a test per setting, reproducing the scenario "Issuer configured without the mode".
- [x] 2.3 Implement key loading from PEM files:
  - key type to algorithm (EC P-256 → ES256, EC P-384 → ES384, Ed25519 → EdDSA, RSA ≥ 2048 bits → RS256);
  - RFC 7638 thumbprint as `kid`;
  - published keys;
  - rejection of duplicate `kid`s.

  Verify with tests that generate one key of each supported type. The tests must also show that RSA-1024 and an unsupported curve are rejected with a message naming the file.

## 3. Startup sequence

- [x] 3.1 Implement design D7 checks 1–4 before any listener is bound:
  - mode and transport, including `--cli`;
  - operator token and the fallback flag;
  - required settings and URL schemes;
  - host coverage of `-resource` and `-forgejo-jwt-issuer` by the Host policy;
  - the keys.

  Verify with a table test: one row per refusal condition in the requirement "resource-server mode refuses to start on an unusable configuration". Each row asserts that the message names the setting and that no socket was opened.
- [x] 3.2 Probe `GET /api/v1/version` without credentials, require Forgejo 16.0 or newer, and pass the version to every ephemeral SDK client via `SetForgejoVersion`. Verify with an `httptest` Forgejo that reports `15.0.3` (refused) and `16.0.3` (accepted). The test also asserts that a tool call in this mode makes no per-client `/version` request.
- [x] 3.3 Fetch the IdP's metadata at startup: OIDC discovery first, then RFC 8414. Require the issuer to match exactly and a `jwks_uri` to be present. Verify with tests for an unreachable IdP, a missing `jwks_uri`, and the scenario "Provider issuer mismatch" (a trailing slash).

## 4. Routing and the request guard

- [x] 4.1 Separate the public routes from the protected one. The routing lives in `operation/resource_server_http.go`; `operation/listen.go` keeps owning the listener and its Host and Origin checks:
  - the public group (protected resource metadata, discovery, JWKS) never passes through the authentication layer;
  - the MCP endpoint does;
  - Host and Origin checks apply to both groups;
  - routes are registered as exact paths, and every other path returns `404`.

  Verify three things: every existing test in `operation/listen_test.go` passes unchanged in `passthrough` mode; the conformance table runs in both modes; `TestEveryNetworkTransportIsCovered` still holds.
- [x] 4.2 Add tests for the scenarios "Root OpenID discovery" and "Trailing slash" (`404`, never a redirect), and for `OPTIONS *` with a forged Host in `resource-server` mode (`403`).

## 5. Inbound validation (`oauth-resource-server`)

- [x] 5.1 Implement the IdP key-set cache: a minimum refresh interval, and at most one refetch per interval triggered by unknown `kid`s. Verify with the scenario "Burst of unknown key IDs": an `httptest` IdP counts fetches, and there is exactly one.
- [x] 5.2 Implement token validation as specified:
  - `Bearer` only;
  - asymmetric algorithm taken from the key;
  - exact `iss`, with `exp` required;
  - `nbf`/`iat` leeway of 60 s;
  - `aud` containment for both string and array;
  - refusal of tokens carrying `nonce` or `at_hash`;
  - the `typ` rule.

  Verify with a table test covering every scenario of "The MCP endpoint requires a valid bearer JWT access token".
- [x] 5.3 Implement the uniform `401` challenge:
  - `resource_metadata`;
  - `scope` when configured;
  - `error="invalid_token"` only when a token was presented;
  - an empty body;
  - a debug log line through the rate-limited refusal logger.

  Verify the scenarios "No token" and "Expired and wrongly signed tokens are indistinguishable" by comparing the two responses byte for byte.
- [x] 5.4 Serve the RFC 9728 protected resource metadata at the path-specific location and at the root. Verify the scenarios "Metadata at the path-specific location" and "Metadata with a forged Host", and that `scopes_supported` appears only when configured.

## 6. Outbound issuer (`forgejo-jwt-issuer`)

- [x] 6.1 Extract the Forgejo audience from the configured claim and apply the "usable" rules. A valid token without a usable audience gets `403` with a body naming the claim, no `WWW-Authenticate` error, and no Forgejo request. Verify the scenarios "Claim missing", "Claim is an array" and "Custom claim name".
- [x] 6.2 Mint the outbound JWT for each authenticated request:
  - header `alg`, thumbprint `kid`, `typ` `JWT`;
  - exact `iss`, `sub` and a single `aud`;
  - `iat` = now − 60 s, `exp` = now + 300 s;
  - a random 128-bit `jti`;
  - no `nbf`.

  Verify the scenarios "Claims of a minted JWT" and "Two MCP requests" by decoding the minted tokens in a test.
- [x] 6.3 Hand the minted token to the Forgejo clients. The auth layer stores it in the request context. In `resource-server` mode `requestTokenContextFunc` injects it with `forgejo.WithToken` and never reads the `Authorization` header. Verify with an `httptest` Forgejo that records every `Authorization` header on the typed SDK path and on the raw-HTTP path: both carry `token <minted jwt>`, and the inbound token never appears. This covers the scenarios "A tool call reaches Forgejo" and "Forgejo never sees the inbound token".
- [x] 6.4 Serve the discovery document and the JWKS under the issuer path:
  - exactly three discovery members, including a `jwks_uri` on the same host;
  - `use`/`alg`/`kid` on every key, and no private key material;
  - additional published keys included;
  - a size of at most 16 KiB and `Cache-Control: public, max-age=300`.

  Verify the scenarios "Forgejo fetches discovery", "Private key material never published" and "Key published ahead of use".

## 7. Logging

- [x] 7.1 Log one warning at every start in `resource-server` mode, naming the `kid` and the issuer and stating the key's reach. Verify with the scenario "Startup warning" in a test that captures the log.
- [x] 7.2 Ensure that neither the inbound token nor the minted JWT, nor any part of their signatures, is ever logged, including at debug level. `sub` and the audience appear at debug level only. Verify the scenarios "Debug logging of a refusal" and "Minted JWTs are not logged" by scanning the captured logs of a full test request cycle for both tokens.

## 8. Documentation

- [x] 8.1 README, "Configuration Options": document `-auth-mode` and every D2 flag with its environment variable and default. Add a short "Remote operation as an OAuth resource server" section that links to the guides. Verify that every flag from `cmd/cmd.go` appears in the README.
- [x] 8.2 Add an operator guide under `docs/`. It covers:
  - IdP requirements: JWT access tokens, the audience override, a custom claim from a user attribute;
  - deployment order: forgejo-mcp must be live before users save integrations;
  - proxy rules: no redirects on the metadata paths, and `-allowed-hosts`;
  - key files and the rotation procedure with Forgejo's `CACHE_TTL` margin;
  - Forgejo's private-network restriction;
  - a plain statement of the signing key's reach;
  - a worked Zitadel example, noting its reliance on Actions v1.

  Verify the guide by following it once for the deployment in 9.1.
- [x] 8.3 Add a user guide. It covers:
  - creating a Generic JWT Authorized Integration with the forgejo-mcp issuer;
  - the **mandatory** `sub` claim rule, and why a missing rule lets anyone holding the audience act as that user;
  - narrow scopes;
  - storing the generated audience in the IdP attribute;
  - configuring an MCP client with a pre-registered client ID.

  Verify by onboarding one user with it in 9.1.
- [x] 8.4 `SECURITY.md`, "Deployment notes": add a note on `resource-server` mode. forgejo-mcp then holds a signing key instead of a forge token, and that key acts for every user whose integration trusts it. Verify by review in the PR.

## 9. Live deployment and end-to-end verification

- [x] 9.1 Deploy the implementation branch in `resource-server` mode at `https://forgejo-mcp.byteflavour.dev`:
  - forgejo-mcp takes over the discovery and JWKS routes from the spike's static documents;
  - a production signing key replaces the disposable spike keys;
  - Zitadel project `forgejo-mcp` is the IdP.

  This is tracked in nixos-config, where the change `chiba-forgejo-mcp` holds the follow-ups for the spike keys. Verify that discovery and JWKS answer from forgejo-mcp itself, and that the existing integration still authenticates after the key change, observing the rotation procedure.
- [x] 9.2 End to end with Claude Code against the deployment. Verify:
  - login through Zitadel succeeds;
  - `get_my_user_info` returns the integration owner;
  - a user without a grant is refused;
  - a token whose `forgejo_aud` claim is missing gets `403`.

  **Done 2026-09-13** with a throwaway Zitadel user, `forgejo-mcp-test`, and a separate Claude Code server entry, so the operator's account and token stayed untouched. Without a role grant, Zitadel accepted the password but refused the authorization with `Errors.User.GrantRequired` and never redirected back to the client, so no token was issued. With the grant but without `forgejo_aud` metadata, Zitadel issued a token, and forgejo-mcp answered the first MCP request with `403` and a body naming `forgejo_aud`.
- [x] 9.3 Regression check of `passthrough`: run the existing test suite, then one `stdio` and one `http` session with a PAT against a Forgejo instance. Verify that both behave as in 3.0.x.

## 10. Showboat demos (anchored)

- [x] 10.1 Add `<!-- demos-anchored: true -->` before the first H2 of `specs/oauth-resource-server/spec.md`. Create `specs/oauth-resource-server/oauth-resource-server.demo.md` following the anchored-mode convention:
  - replay setup with `${FORGEJO_MCP_BIN:-forgejo-mcp}`;
  - provenance markers;
  - one proof block per `#### Scenario:`, in one consistent block shape.

  Use `showboat-cli` or captured `curl` evidence against the 9.1 deployment where the behaviour is observable from outside. Use `test-invocation` for scenarios only a test can trigger, such as a symmetric-algorithm token or a burst of unknown key IDs. Verify that `make check-demos` exits 0.
- [x] 10.2 Do the same for `specs/forgejo-jwt-issuer/spec.md` with `specs/forgejo-jwt-issuer/forgejo-jwt-issuer.demo.md`. Live evidence includes the discovery document and JWKS fetched from the deployment, and a decoded minted JWT (claims only, never the signature). Verify that `make check-demos` exits 0.
- [x] 10.3 Prove the scenarios of the `stateless-http-auth` delta. Agree with the maintainer whether `openspec/specs/stateless-http-auth/spec.md` becomes anchored, since that would also require proofs for its existing scenarios. Otherwise add a non-anchored section to `oauth-resource-server.demo.md`. Verify that `make check-demos` exits 0 and the PR description links every demo.

## 11. Final verification

- [x] 11.1 Run all quality gates on the final branch and confirm each exits 0:
  - `go vet ./...`
  - `go test ./...`
  - `make build`
  - `scripts/ci/check-api-path-escaping.sh`
  - `openspec validate oauth-resource-server-mode --strict`
  - `make check-demos`
- [x] 11.2 Take the PR out of WIP once the checks pass and every task above is checked. Verify that the PR description lists the demos and carries the planning questions for the maintainer. **Done 2026-09-13**, after 9.2.

## 12. Review follow-ups

- [x] 12.1 Refuse to start when the path of `-resource` is not `/mcp`, the path the endpoint is served at. Verify with rows in `TestValidateAuthConfig` for another path and for a trailing slash, the scenario "Resource names another path", and a refusal captured from the binary in the demo.
- [x] 12.2 Detach a key-set refetch from the request that triggered it, so a caller that disconnects cannot fail it and still spend the interval. Verify with `TestARefetchCompletesWhenTheRequestThatTriggeredItIsCancelled` and the scenario "Caller disconnects during a refetch", and check that the test fails with the fetch bound to the caller's context.
- [x] 12.3 Record in the code why a valid token without a usable audience gets a `403` with no `WWW-Authenticate` header, and state that header rule in the `forgejo-jwt-issuer` spec. Verify that the existing `403` test still asserts the absent header.
- [x] 12.4 State in the operator guide that forgejo-mcp fetches the provider's `jwks_uri`, which may be on another host, at startup and on every refresh.
