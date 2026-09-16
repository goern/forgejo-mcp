<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# OAuth resource server (oauth-resource-server)

*Captured: 2026-09-10 via Showboat 0.6.1*
<!-- captured-for: PR #584 -->
<!-- captured-at: 2026-09-10 -->
<!-- captured-against: 8d525d7 (byteflavour/feat/oauth-rs-issuer-core); live evidence against https://forgejo-mcp.byteflavour.dev running 7d48e99 (tag deploy/chiba-20260910); review follow-ups and every refusal re-captured 2026-09-14 against the same branch after 5b065d6 -->
<!-- re-capture changed only the log line of each refusal, cmd/cmd.go:294 to :295, moved by later commits to cmd/cmd.go -->

Proves the `oauth-resource-server` capability of the change `oauth-resource-server-mode` against its [spec](./spec.md).

The evidence comes from three places, and each proof names its kind:

- **The deployment.** `https://forgejo-mcp.byteflavour.dev` runs this implementation in `resource-server` mode, with Zitadel as the identity provider and Forgejo 16.0.3 behind it. The `curl` commands replay against it without credentials. One proof is an MCP session from Claude Code against the same deployment.
- **The binary.** A refused start needs no network, no key and no identity provider, so those proofs run `forgejo-mcp` itself.
- **The tests.** From outside, every refused token gets the same `401`, by design, so the reason for a refusal is visible only in a test. The tests run against an in-process Forgejo and identity provider.

No access token, minted JWT, signing key or other credential appears in this file.

## Replay setup

```bash
# Run from the repository root. Needs bash, Go, curl and jq.
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-forgejo-mcp}"   # a local build: export FORGEJO_MCP_BIN=./forgejo-mcp
export FORGEJO_MCP_LIVE="${FORGEJO_MCP_LIVE:-https://forgejo-mcp.byteflavour.dev}"
set -o pipefail

# proof runs the named tests and keeps their verdicts and log lines.
proof() {
  go test -count=1 -v "$@" 2>&1 \
    | grep -E -- '^\s*--- (PASS|FAIL|SKIP)|^\s+[A-Za-z0-9_]+_test\.go:[0-9]+: |^(ok|FAIL)\s' \
    | sed -E 's/ \([0-9.]+s\)$//; s/\t[0-9.]+s$//'
}

# refusal starts the binary, which must refuse before binding anything, and
# prints the refusal without timestamp and colour codes, then the exit status.
refusal() {
  timeout 5 "$FORGEJO_MCP_BIN" "$@" 2>&1 \
    | sed -E 's/\x1b\[[0-9;]*m//g; s/^[0-9-]+ [0-9:.]+\t//' | grep -m1 FATAL
  echo "exit status ${PIPESTATUS[0]}"
}
```

## Scenarios

<!-- spec-scenario: oauth-resource-server#no-mode-configured -->
**Proves:** [spec.md → Scenario: No mode configured](./spec.md#scenario-no-mode-configured)

#### Scenario: No mode configured
- **WHEN** the server starts on `http` with neither `-auth-mode` nor `FORGEJO_MCP_AUTH_MODE` set
- **THEN** it SHALL run in `passthrough` mode
- **AND** it SHALL authenticate requests exactly as it did before this capability existed

<!-- evidence-kind: test-invocation -->
*Proof:* with neither `-auth-mode` nor `FORGEJO_MCP_AUTH_MODE` set, the resolved mode is `passthrough` and no resource-server setting counts as given.

```bash
proof ./cmd/ -run '^TestAuthModeDefaultsToPassthroughWithNothingGiven$'
```

```output
--- PASS: TestAuthModeDefaultsToPassthroughWithNothingGiven
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/cmd
```

<!-- evidence-kind: unit-test-output -->
*Proof:* the `sse` and `http` rows of the listener conformance table run in `passthrough` mode. They still refuse an anonymous request, admit a request that carries a token, and honour the operator fallback, as before this capability.

```bash
proof ./operation/ -run '^Test(AnonymousRequestIsRefusedAtTheDoor|AuthenticatedRequestPassesTheDoor|OperatorFallbackOptInReAdmitsAnonymousRequests)$/^(sse|http)$'
```

```output
--- PASS: TestAnonymousRequestIsRefusedAtTheDoor
    --- PASS: TestAnonymousRequestIsRefusedAtTheDoor/sse
    --- PASS: TestAnonymousRequestIsRefusedAtTheDoor/http
--- PASS: TestOperatorFallbackOptInReAdmitsAnonymousRequests
    --- PASS: TestOperatorFallbackOptInReAdmitsAnonymousRequests/sse
    --- PASS: TestOperatorFallbackOptInReAdmitsAnonymousRequests/http
--- PASS: TestAuthenticatedRequestPassesTheDoor
    --- PASS: TestAuthenticatedRequestPassesTheDoor/sse
    --- PASS: TestAuthenticatedRequestPassesTheDoor/http
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#unknown-mode-value -->
**Proves:** [spec.md → Scenario: Unknown mode value](./spec.md#scenario-unknown-mode-value)

#### Scenario: Unknown mode value
- **WHEN** the server is started with `-auth-mode oauth`
- **THEN** it SHALL refuse to start
- **AND** the message SHALL name `-auth-mode` and list `passthrough` and `resource-server`

<!-- evidence-kind: showboat-cli -->
*Proof:* the binary refuses `-auth-mode oauth`, naming the flag and both accepted values.

```bash
refusal -url https://forgejo.example.org -transport http -auth-mode oauth
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: -auth-mode \"oauth\" is not valid; use passthrough or resource-server"}
exit status 1
```

<!-- spec-scenario: oauth-resource-server#incompatible-transport -->
**Proves:** [spec.md → Scenario: Incompatible transport](./spec.md#scenario-incompatible-transport)

#### Scenario: Incompatible transport
- **WHEN** the server is started with `-auth-mode resource-server` and `-transport sse`
- **THEN** it SHALL refuse to start before binding any listener
- **AND** the message SHALL say that `resource-server` mode requires the `http` transport

<!-- evidence-kind: showboat-cli -->
*Proof:* the binary refuses `resource-server` mode on the `sse` transport.

```bash
refusal -url https://forgejo.example.org -transport sse -auth-mode resource-server
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: -auth-mode resource-server requires the http transport, not \"sse\""}
exit status 1
```

<!-- evidence-kind: test-invocation -->
*Proof:* the same refusal through `Run`. The test then binds the configured port itself, which fails if the refused start had bound it.

```bash
proof ./operation/ -run '^TestRefusedConfigurationNeverBinds$'
```

```output
--- PASS: TestRefusedConfigurationNeverBinds
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#operator-token-present -->
**Proves:** [spec.md → Scenario: Operator token present](./spec.md#scenario-operator-token-present)

#### Scenario: Operator token present
- **WHEN** the server is started in `resource-server` mode with `FORGEJO_ACCESS_TOKEN` set
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that this mode takes no operator token

<!-- evidence-kind: showboat-cli -->
*Proof:* the binary refuses `resource-server` mode while `FORGEJO_ACCESS_TOKEN` is set. The value is a decoy.

```bash
FORGEJO_ACCESS_TOKEN=decoy-operator-value refusal -url https://forgejo.example.org -transport http -auth-mode resource-server
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: -auth-mode resource-server takes no operator token; unset -token and FORGEJO_ACCESS_TOKEN (and GITEA_ACCESS_TOKEN)"}
exit status 1
```

<!-- spec-scenario: oauth-resource-server#forgejo-too-old -->
**Proves:** [spec.md → Scenario: Forgejo too old](./spec.md#scenario-forgejo-too-old)

#### Scenario: Forgejo too old
- **WHEN** the server is started in `resource-server` mode and the Forgejo instance reports version `15.0.3`
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that `resource-server` mode requires Forgejo 16.0 or newer

<!-- evidence-kind: test-invocation -->
*Proof:* a Forgejo that reports `15.0.3` makes the startup refuse. The test asserts that the message says `requires Forgejo 16.0 or newer` and names the reported version.

```bash
proof ./operation/ -run '^TestPrepareResourceServerRefusesForgejoBefore16$'
```

```output
--- PASS: TestPrepareResourceServerRefusesForgejoBefore16
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#provider-issuer-mismatch -->
**Proves:** [spec.md → Scenario: Provider issuer mismatch](./spec.md#scenario-provider-issuer-mismatch)

#### Scenario: Provider issuer mismatch
- **WHEN** `-authorization-server` is `https://id.example.org` and the provider's metadata reports `issuer` as `https://id.example.org/`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL show both values

<!-- evidence-kind: test-invocation -->
*Proof:* provider metadata whose `issuer` is the configured value plus a trailing slash is refused. Discovery asserts that the message shows both values; the startup sequence asserts that it names `-authorization-server`.

```bash
proof ./pkg/oauthrs/ ./operation/ -run '^(TestDiscoveryRefusesAnIssuerThatDiffersByATrailingSlash|TestPrepareResourceServerRefusesAProviderIssuerMismatch)$'
```

```output
--- PASS: TestDiscoveryRefusesAnIssuerThatDiffersByATrailingSlash
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
--- PASS: TestPrepareResourceServerRefusesAProviderIssuerMismatch
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#issuer-url-with-a-trailing-slash -->
**Proves:** [spec.md → Scenario: Issuer URL with a trailing slash](./spec.md#scenario-issuer-url-with-a-trailing-slash)

#### Scenario: Issuer URL with a trailing slash
- **WHEN** `-forgejo-jwt-issuer` is `https://mcp.example.org/issuer/`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL say that the issuer URL must not end with a slash

<!-- evidence-kind: showboat-cli -->
*Proof:* the binary refuses an issuer URL that ends with a slash, before it reads the key file, which does not exist here.

```bash
refusal -url https://forgejo.example.org -transport http -host 0.0.0.0 \
  -auth-mode resource-server -authorization-server https://id.example.org \
  -forgejo-jwt-signing-key-file signing.pem \
  -allowed-hosts mcp.example.org -resource https://mcp.example.org/mcp \
  -forgejo-jwt-issuer https://mcp.example.org/issuer/
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: -forgejo-jwt-issuer: issuer URL \"https://mcp.example.org/issuer/\" must not end with a slash"}
exit status 1
```

<!-- spec-scenario: oauth-resource-server#issuer-host-not-answered -->
**Proves:** [spec.md → Scenario: Issuer host not answered](./spec.md#scenario-issuer-host-not-answered)

#### Scenario: Issuer host not answered
- **WHEN** `-forgejo-jwt-issuer` is `https://mcp.example.org/issuer`, the listener is not loopback-only, and `-allowed-hosts` does not include `mcp.example.org`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL name `-allowed-hosts`

<!-- evidence-kind: showboat-cli -->
*Proof:* the listener is not loopback-only, and `-allowed-hosts` covers the resource host but not the issuer host.

```bash
refusal -url https://forgejo.example.org -transport http -host 0.0.0.0 \
  -auth-mode resource-server -authorization-server https://id.example.org \
  -forgejo-jwt-signing-key-file signing.pem \
  -allowed-hosts proxy.example.org -resource https://proxy.example.org/mcp \
  -forgejo-jwt-issuer https://mcp.example.org/issuer
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: the host of -forgejo-jwt-issuer (mcp.example.org) is not one this server answers to; add it to -allowed-hosts"}
exit status 1
```

<!-- spec-scenario: oauth-resource-server#resource-names-another-path -->
**Proves:** [spec.md → Scenario: Resource names another path](./spec.md#scenario-resource-names-another-path)

#### Scenario: Resource names another path
- **WHEN** `-resource` is `https://mcp.example.org/api/mcp`
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL name `-resource` and the endpoint path `/mcp`

<!-- evidence-kind: showboat-cli -->
*Proof:* the binary refuses a resource URL whose host is answered but whose path is not `/mcp`, before it reads the key file, which does not exist here.

```bash
refusal -url https://forgejo.example.org -transport http -host 0.0.0.0 \
  -auth-mode resource-server -authorization-server https://id.example.org \
  -forgejo-jwt-signing-key-file signing.pem \
  -allowed-hosts mcp.example.org -resource https://mcp.example.org/api/mcp \
  -forgejo-jwt-issuer https://mcp.example.org/issuer
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: the path of -resource (\"/api/mcp\") must be \"/mcp\", the MCP endpoint this server serves"}
exit status 1
```

<!-- spec-scenario: oauth-resource-server#issuer-configured-without-the-mode -->
**Proves:** [spec.md → Scenario: Issuer configured without the mode](./spec.md#scenario-issuer-configured-without-the-mode)

#### Scenario: Issuer configured without the mode
- **WHEN** the server starts with `FORGEJO_MCP_AUTHORIZATION_SERVER` set and no auth mode configured
- **THEN** it SHALL refuse to start
- **AND** the message SHALL say that the setting requires `-auth-mode resource-server`

<!-- evidence-kind: showboat-cli -->
*Proof:* the binary refuses to start in `passthrough` mode while `FORGEJO_MCP_AUTHORIZATION_SERVER` is set.

```bash
FORGEJO_MCP_AUTHORIZATION_SERVER=https://id.example.org refusal -url https://forgejo.example.org -transport http
```

```output
FATAL	cmd/cmd.go:295	Invalid authentication configuration	{"error": "refusing to start: FORGEJO_MCP_AUTHORIZATION_SERVER requires -auth-mode resource-server; remove it, or enable that mode"}
exit status 1
```

<!-- evidence-kind: test-invocation -->
*Proof:* every resource-server setting, given as a flag or as an environment variable, is refused the same way, naming itself and `-auth-mode resource-server`.

```bash
proof ./cmd/ -run '^TestPassthroughRefusesEveryResourceServerSetting$'
```

```output
--- PASS: TestPassthroughRefusesEveryResourceServerSetting
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_AUTHORIZATION_SERVER
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-authorization-server
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_RESOURCE
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-resource
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_RESOURCE_AUDIENCE
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-resource-audience
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_SCOPES_SUPPORTED
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-scopes-supported
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-forgejo-audience-claim
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_FORGEJO_JWT_ISSUER
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-forgejo-jwt-issuer
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-forgejo-jwt-signing-key-file
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES
    --- PASS: TestPassthroughRefusesEveryResourceServerSetting/-forgejo-jwt-published-key-files
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/cmd
```

<!-- spec-scenario: oauth-resource-server#valid-access-token -->
**Proves:** [spec.md → Scenario: Valid access token](./spec.md#scenario-valid-access-token)

#### Scenario: Valid access token
- **WHEN** a request to the MCP endpoint carries a JWT access token signed by the provider, with the configured issuer, an unexpired `exp`, and an `aud` array containing the resource audience
- **THEN** the request SHALL be passed to MCP handling

<!-- evidence-kind: external-artifact -->
*Proof:* a tool call from Claude Code, authenticated only by the Zitadel login, returns the Forgejo user. Profile fields other than those shown are elided as `…`.

```text
# Claude Code, MCP server entry "chiba-forge" -> https://forgejo-mcp.byteflavour.dev/mcp
# Logged in through Zitadel. No Forgejo token is configured anywhere.
tools/call get_forgejo_mcp_server_version {}
tools/call get_my_user_info {}
```

```output
{"Result":"Forgejo MCP Server version: 0-unstable-2026-09-10+7d48e99"}
{"Result":{"id":1,"login":"synapse",…,"html_url":"https://git.byteflavour.dev/synapse",…,"is_admin":false,…}}
```

<!-- evidence-kind: test-invocation -->
*Proof:* a provider-signed token with the configured issuer, an unexpired `exp` and an `aud` array holding the resource audience is accepted, with `typ` `at+jwt`, `JWT` and none.

```bash
proof ./pkg/oauthrs/ -run '^TestValidAccessTokenIsAccepted$'
```

```output
--- PASS: TestValidAccessTokenIsAccepted
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#audience-names-another-resource -->
**Proves:** [spec.md → Scenario: Audience names another resource](./spec.md#scenario-audience-names-another-resource)

#### Scenario: Audience names another resource
- **WHEN** a request carries an otherwise valid token whose `aud` does not contain the resource audience
- **THEN** the server SHALL respond `401`

<!-- evidence-kind: test-invocation -->
*Proof:* the validator refuses the token as invalid, which the MCP endpoint answers with `401` (see "Expired and wrongly signed tokens are indistinguishable").

```bash
proof ./pkg/oauthrs/ -run '^TestInvalidTokensAreRefused$/^audience_names_another_resource$'
```

```output
--- PASS: TestInvalidTokensAreRefused
    --- PASS: TestInvalidTokensAreRefused/audience_names_another_resource
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#id-token-presented-as-an-access-token -->
**Proves:** [spec.md → Scenario: ID token presented as an access token](./spec.md#scenario-id-token-presented-as-an-access-token)

#### Scenario: ID token presented as an access token
- **WHEN** a request carries a token signed by the provider with a matching `iss` and `aud` that also carries a `nonce` claim
- **THEN** the server SHALL respond `401`

<!-- evidence-kind: test-invocation -->
*Proof:* a provider-signed token with matching `iss` and `aud` but a `nonce` claim is refused as invalid; so is one with `at_hash`.

```bash
proof ./pkg/oauthrs/ -run '^TestInvalidTokensAreRefused$/^ID_token_with_(nonce|at_hash)$'
```

```output
--- PASS: TestInvalidTokensAreRefused
    --- PASS: TestInvalidTokensAreRefused/ID_token_with_nonce
    --- PASS: TestInvalidTokensAreRefused/ID_token_with_at_hash
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#symmetric-algorithm -->
**Proves:** [spec.md → Scenario: Symmetric algorithm](./spec.md#scenario-symmetric-algorithm)

#### Scenario: Symmetric algorithm
- **WHEN** a request carries a token whose header declares `HS256`
- **THEN** the server SHALL respond `401`

<!-- evidence-kind: test-invocation -->
*Proof:* a token whose header declares `HS256` is refused as invalid; so is `alg` `none`.

```bash
proof ./pkg/oauthrs/ -run '^TestInvalidTokensAreRefused$/^(HS256|alg_none)$'
```

```output
--- PASS: TestInvalidTokensAreRefused
    --- PASS: TestInvalidTokensAreRefused/HS256
    --- PASS: TestInvalidTokensAreRefused/alg_none
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#opaque-token -->
**Proves:** [spec.md → Scenario: Opaque token](./spec.md#scenario-opaque-token)

#### Scenario: Opaque token
- **WHEN** a request carries `Authorization: Bearer 8f3a...` that is not a JWT
- **THEN** the server SHALL respond `401`

<!-- evidence-kind: test-invocation -->
*Proof:* a bearer value that is not a JWT is refused as invalid.

```bash
proof ./pkg/oauthrs/ -run '^TestInvalidTokensAreRefused$/^opaque_token$'
```

```output
--- PASS: TestInvalidTokensAreRefused
    --- PASS: TestInvalidTokensAreRefused/opaque_token
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#forgejo-token-scheme -->
**Proves:** [spec.md → Scenario: Forgejo token scheme](./spec.md#scenario-forgejo-token-scheme)

#### Scenario: Forgejo token scheme
- **WHEN** a request carries `Authorization: token <valid JWT>`
- **THEN** the server SHALL respond `401`

<!-- evidence-kind: unit-test-output -->
*Proof:* a valid provider token sent as `token <jwt>` counts as no bearer token at all, which the MCP endpoint answers with `401`. The same token sent as `bearer` or `BEARER` is accepted.

```bash
proof ./pkg/oauthrs/ -run '^TestAuthorizationSchemes$'
```

```output
--- PASS: TestAuthorizationSchemes
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#no-token -->
**Proves:** [spec.md → Scenario: No token](./spec.md#scenario-no-token)

#### Scenario: No token
- **WHEN** a request to the MCP endpoint carries no `Authorization` header
- **THEN** the server SHALL respond `401` with `WWW-Authenticate` carrying `resource_metadata`
- **AND** the challenge SHALL NOT carry an `error` parameter

<!-- evidence-kind: showboat-cli -->
*Proof:* the deployment answers a request without `Authorization` with `401`, an empty body and a challenge that carries no `error`.

```bash
curl -sS -o /dev/null -D - -X POST "$FORGEJO_MCP_LIVE/mcp" | tr -d '\r' | grep -iE '^(HTTP/|www-authenticate:|content-length:)'
```

```output
HTTP/2 401 
www-authenticate: Bearer resource_metadata="https://forgejo-mcp.byteflavour.dev/.well-known/oauth-protected-resource/mcp", scope="openid profile email"
content-length: 0
```

<!-- evidence-kind: test-invocation -->
*Proof:* the test compares the challenge byte for byte and asserts the empty body.

```bash
proof ./operation/ -run '^TestTheMCPEndpointChallengesUniformly$'
```

```output
--- PASS: TestTheMCPEndpointChallengesUniformly
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#expired-and-wrongly-signed-tokens-are-indistinguishable -->
**Proves:** [spec.md → Scenario: Expired and wrongly signed tokens are indistinguishable](./spec.md#scenario-expired-and-wrongly-signed-tokens-are-indistinguishable)

#### Scenario: Expired and wrongly signed tokens are indistinguishable
- **WHEN** one request carries an expired token and another carries a token with an invalid signature
- **THEN** both responses SHALL be byte-identical
- **AND** both SHALL carry `error="invalid_token"`

<!-- evidence-kind: test-invocation -->
*Proof:* an expired provider token and a token signed by another key under the provider key ID get the same status, challenge and body, and the challenge carries `error="invalid_token"`. Neither request reaches Forgejo.

```bash
proof ./operation/ -run '^TestTheMCPEndpointChallengesUniformly$'
```

```output
--- PASS: TestTheMCPEndpointChallengesUniformly
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#burst-of-unknown-key-ids -->
**Proves:** [spec.md → Scenario: Burst of unknown key IDs](./spec.md#scenario-burst-of-unknown-key-ids)

#### Scenario: Burst of unknown key IDs
- **WHEN** many requests within one refresh interval carry tokens with different unknown key IDs
- **THEN** the server SHALL fetch the provider's key set at most once during that interval
- **AND** every such request SHALL receive `401`

<!-- evidence-kind: test-invocation -->
*Proof:* 50 tokens with different unknown key IDs inside one refresh interval cause exactly one refetch of the key set, and every one of them is refused as invalid, which the endpoint answers with `401`.

```bash
proof ./pkg/oauthrs/ -run '^TestUnknownKeyIDsTriggerAtMostOneFetchPerInterval$'
```

```output
--- PASS: TestUnknownKeyIDsTriggerAtMostOneFetchPerInterval
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#caller-disconnects-during-a-refetch -->
**Proves:** [spec.md → Scenario: Caller disconnects during a refetch](./spec.md#scenario-caller-disconnects-during-a-refetch)

#### Scenario: Caller disconnects during a refetch
- **WHEN** a request past the refresh interval names a key ID the provider has just added, and its client disconnects before the key set is fetched
- **THEN** the server SHALL complete the fetch
- **AND** a later request within the same interval SHALL find the new key without another fetch

<!-- evidence-kind: test-invocation -->
*Proof:* past the refresh interval, a lookup of a key the provider has just added, made with an already-cancelled context, refetches the set and finds the key; a second lookup finds it without another fetch. With the fetch bound to the caller's context, the test fails with `context canceled`.

```bash
proof ./pkg/oauthrs/ -run '^TestARefetchCompletesWhenTheRequestThatTriggeredItIsCancelled$'
```

```output
--- PASS: TestARefetchCompletesWhenTheRequestThatTriggeredItIsCancelled
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```

<!-- spec-scenario: oauth-resource-server#metadata-at-the-path-specific-location -->
**Proves:** [spec.md → Scenario: Metadata at the path-specific location](./spec.md#scenario-metadata-at-the-path-specific-location)

#### Scenario: Metadata at the path-specific location
- **WHEN** `-resource` is `https://mcp.example.org/mcp` and a request without `Authorization` asks for `/.well-known/oauth-protected-resource/mcp`
- **THEN** the server SHALL respond `200` with a JSON document whose `resource` is `https://mcp.example.org/mcp`

<!-- evidence-kind: showboat-cli -->
*Proof:* the deployment serves the metadata without authentication at the path-specific location.

```bash
curl -sS -w '\nHTTP %{http_code}\n' "$FORGEJO_MCP_LIVE/.well-known/oauth-protected-resource/mcp"
```

```output
{"resource":"https://forgejo-mcp.byteflavour.dev/mcp","authorization_servers":["https://sso.byteflavour.dev"],"bearer_methods_supported":["header"],"scopes_supported":["openid","profile","email"]}
HTTP 200
```

<!-- evidence-kind: test-invocation -->
*Proof:* the test checks every member of the document at both locations.

```bash
proof ./operation/ -run '^TestProtectedResourceMetadataIsPublishedWithoutAuthentication$'
```

```output
--- PASS: TestProtectedResourceMetadataIsPublishedWithoutAuthentication
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#metadata-with-a-forged-host -->
**Proves:** [spec.md → Scenario: Metadata with a forged Host](./spec.md#scenario-metadata-with-a-forged-host)

#### Scenario: Metadata with a forged Host
- **WHEN** a request for the metadata carries a Host the server does not answer to
- **THEN** the server SHALL respond `403`

<!-- evidence-kind: unit-test-output -->
*Proof:* the metadata test ends with a request for the metadata under `Host: attacker.example.com`, which gets `403`. The `http+resource-server` rows of the listener conformance table show the same for the MCP endpoint.

```bash
proof ./operation/ -run '^(TestProtectedResourceMetadataIsPublishedWithoutAuthentication|TestForgedHostHeaderIsRejected)$'
```

```output
--- PASS: TestForgedHostHeaderIsRejected
    --- PASS: TestForgedHostHeaderIsRejected/sse
    --- PASS: TestForgedHostHeaderIsRejected/http
    --- PASS: TestForgedHostHeaderIsRejected/http+resource-server
--- PASS: TestProtectedResourceMetadataIsPublishedWithoutAuthentication
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#root-openid-discovery -->
**Proves:** [spec.md → Scenario: Root OpenID discovery](./spec.md#scenario-root-openid-discovery)

#### Scenario: Root OpenID discovery
- **WHEN** a request asks for `/.well-known/openid-configuration` and the Forgejo JWT issuer has a path component
- **THEN** the server SHALL respond `404`

<!-- evidence-kind: showboat-cli -->
*Proof:* the deployment, whose Forgejo JWT issuer has the path `/issuer`, answers the root discovery path with `404` and no redirect.

```bash
curl -sS -o /dev/null -w 'HTTP %{http_code} redirect_url=%{redirect_url}\n' "$FORGEJO_MCP_LIVE/.well-known/openid-configuration"
```

```output
HTTP 404 redirect_url=
```

<!-- evidence-kind: test-invocation -->
*Proof:* the test covers the root discovery path among other undefined paths, and an unclean path that `http.ServeMux` would have redirected.

```bash
proof ./operation/ -run '^TestOnlyDefinedRoutesAnswerAndNothingRedirects$'
```

```output
--- PASS: TestOnlyDefinedRoutesAnswerAndNothingRedirects
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#trailing-slash -->
**Proves:** [spec.md → Scenario: Trailing slash](./spec.md#scenario-trailing-slash)

#### Scenario: Trailing slash
- **WHEN** a request asks for the metadata path with an added trailing slash
- **THEN** the server SHALL respond `404`
- **AND** it SHALL NOT respond with a redirect

<!-- evidence-kind: showboat-cli -->
*Proof:* the deployment answers the metadata path with an added trailing slash with `404` and no redirect.

```bash
curl -sS -o /dev/null -w 'HTTP %{http_code} redirect_url=%{redirect_url}\n' "$FORGEJO_MCP_LIVE/.well-known/oauth-protected-resource/mcp/"
```

```output
HTTP 404 redirect_url=
```

<!-- evidence-kind: test-invocation -->
*Proof:* the test asserts `404` for the metadata, issuer, key set and MCP paths with a trailing slash, and never a redirect.

```bash
proof ./operation/ -run '^TestOnlyDefinedRoutesAnswerAndNothingRedirects$'
```

```output
--- PASS: TestOnlyDefinedRoutesAnswerAndNothingRedirects
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#forgejo-never-sees-the-inbound-token -->
**Proves:** [spec.md → Scenario: Forgejo never sees the inbound token](./spec.md#scenario-forgejo-never-sees-the-inbound-token)

#### Scenario: Forgejo never sees the inbound token
- **WHEN** a tool call authenticated with an inbound access token makes requests to Forgejo
- **THEN** none of those requests SHALL carry the inbound access token

<!-- evidence-kind: test-invocation -->
*Proof:* an in-process Forgejo records the `Authorization` header of every request a handler makes through the typed SDK client and the raw-HTTP helper. None contains the inbound token; both carry the same minted JWT.

```bash
proof ./operation/ -run '^TestAToolCallReachesForgejoWithTheMintedTokenOnly$'
```

```output
--- PASS: TestAToolCallReachesForgejoWithTheMintedTokenOnly
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: oauth-resource-server#debug-logging-of-a-refusal -->
**Proves:** [spec.md → Scenario: Debug logging of a refusal](./spec.md#scenario-debug-logging-of-a-refusal)

#### Scenario: Debug logging of a refusal
- **WHEN** the server refuses a token with debug logging enabled
- **THEN** the log SHALL NOT contain the token or any part of its signature

<!-- evidence-kind: test-invocation -->
*Proof:* with logging captured at debug level, the test sends a refused and an accepted token and scans every log entry for both inbound tokens, their signatures, and the minted JWT and its signature.

```bash
proof ./operation/ -run '^TestNeitherTokenIsLoggedAtAnyLevel$'
```

```output
--- PASS: TestNeitherTokenIsLoggedAtAnyLevel
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

## The `stateless-http-auth` delta

`openspec/specs/stateless-http-auth/spec.md` is not an anchored spec, so this change's
delta to it is proven here rather than in a sibling demo of its own. Making that spec
anchored would oblige every one of its existing scenarios to carry a proof, which is the
maintainer's call — see the note in the pull request.

### Scenario: Metadata and issuer documents are served without a credential

The three public documents answer with no `Authorization` header at all; the MCP
endpoint of the same deployment does not, and its refusal carries the challenge that
tells a client where to authenticate.

<!-- evidence-kind: showboat-cli -->

```bash
for path in /.well-known/oauth-protected-resource/mcp /issuer/.well-known/openid-configuration /issuer/jwks.json /mcp; do
  curl -sS -o /dev/null -w "%{http_code}  $path\n" "$FORGEJO_MCP_LIVE$path"
done
curl -sS -o /dev/null -D - -X POST "$FORGEJO_MCP_LIVE/mcp" | tr -d "\r" | grep -iE "^(HTTP/|www-authenticate:)"
```

```output
200  /.well-known/oauth-protected-resource/mcp
200  /issuer/.well-known/openid-configuration
200  /issuer/jwks.json
401  /mcp
HTTP/2 401 
www-authenticate: Bearer resource_metadata="https://forgejo-mcp.byteflavour.dev/.well-known/oauth-protected-resource/mcp", scope="openid profile email"
```

### Scenario: Scheme matching is case-insensitive

This proves the `resource-server` clause of the scenario: `Bearer` is accepted as a bearer token and `token` is not. `token <jwt>` counts as no bearer token at all, so its challenge carries no `error`
parameter. A malformed `Bearer` value is a token that was presented and failed
validation, so its challenge adds `error="invalid_token"`. Neither reaches Forgejo.

<!-- evidence-kind: showboat-cli -->

```bash
for scheme in token Bearer; do
  curl -sS -o /dev/null -D - -X POST "$FORGEJO_MCP_LIVE/mcp" \
    -H "Authorization: $scheme eyJhbGciOiJFUzI1NiJ9.e30.x" | tr -d "\r" | grep -iE "^www-authenticate:"
done
```

```output
www-authenticate: Bearer resource_metadata="https://forgejo-mcp.byteflavour.dev/.well-known/oauth-protected-resource/mcp", scope="openid profile email"
www-authenticate: Bearer resource_metadata="https://forgejo-mcp.byteflavour.dev/.well-known/oauth-protected-resource/mcp", scope="openid profile email", error="invalid_token"
```

### Scenario: The inbound access token is not a Forgejo credential

An in-process Forgejo records the `Authorization` header of every request a handler
makes. Both client paths carry the minted JWT, the inbound token appears nowhere, and
neither token reaches the log even at debug level.

<!-- evidence-kind: test-invocation -->

```bash
proof ./operation/ -run '^(TestAToolCallReachesForgejoWithTheMintedTokenOnly|TestNeitherTokenIsLoggedAtAnyLevel)$'
```

```output
--- PASS: TestAToolCallReachesForgejoWithTheMintedTokenOnly
--- PASS: TestNeitherTokenIsLoggedAtAnyLevel
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

### Scenario: Every Forgejo call in resource-server mode uses a minted token

The singleton is constructed from an operator token. This mode refuses to start when one
is configured, when the fallback flag is set, and on the transports where the singleton
is the credential — so in a running `resource-server` process there is nothing for the
factory to fall back to. `TestAuthorizationSchemes` covers the other half: `token <jwt>`
is not a bearer token, while `bearer` and `BEARER` are.

<!-- evidence-kind: test-invocation -->

```bash
proof ./operation/ ./pkg/oauthrs/ -run '^(TestValidateAuthConfig|TestAuthorizationSchemes)$/^(operator_token_present|operator_token_fallback|cli_mode|sse_transport)$'
```

```output
--- PASS: TestValidateAuthConfig
    --- PASS: TestValidateAuthConfig/sse_transport
    --- PASS: TestValidateAuthConfig/cli_mode
    --- PASS: TestValidateAuthConfig/operator_token_present
    --- PASS: TestValidateAuthConfig/operator_token_fallback
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
--- PASS: TestAuthorizationSchemes
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/oauthrs
```
