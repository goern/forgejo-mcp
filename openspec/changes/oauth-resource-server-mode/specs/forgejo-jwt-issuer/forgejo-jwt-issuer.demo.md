<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Forgejo JWT issuer (forgejo-jwt-issuer)

*Captured: 2026-09-10 via Showboat 0.6.1*
<!-- captured-for: PR #584 -->
<!-- captured-at: 2026-09-10 -->
<!-- captured-against: 8d525d7 (byteflavour/feat/oauth-rs-issuer-core); live evidence against https://forgejo-mcp.byteflavour.dev running 7d48e99 (tag deploy/chiba-20260910) -->

Proves the `forgejo-jwt-issuer` capability of the change `oauth-resource-server-mode` against its [spec](./spec.md).

The evidence comes from three places, and each proof names its kind:

- **The deployment.** `https://forgejo-mcp.byteflavour.dev` runs this implementation in `resource-server` mode against Forgejo 16.0.3. Its discovery document and key set are public, so the `curl` commands replay without credentials. One proof is an MCP session from Claude Code against the same deployment.
- **The deployment's journal.** The startup warning and a scan for tokens were captured from the systemd journal on the deployment host. Those commands need that host and do not replay elsewhere.
- **The tests.** A minted JWT only travels from forgejo-mcp to Forgejo, so its decoded header and claims come from a test, which prints them and never the signature. The tests run against an in-process Forgejo and identity provider.

No access token, JWT signature, signing key or other credential appears in this file.

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

<!-- spec-scenario: forgejo-jwt-issuer#a-tool-call-reaches-forgejo -->
**Proves:** [spec.md → Scenario: A tool call reaches Forgejo](./spec.md#scenario-a-tool-call-reaches-forgejo)

#### Scenario: A tool call reaches Forgejo
- **WHEN** an authenticated MCP request invokes a tool that calls the Forgejo API twice
- **THEN** both Forgejo requests SHALL carry `Authorization: token <jwt>`, with the same JWT signed by forgejo-mcp
- **AND** neither SHALL carry the inbound access token

<!-- evidence-kind: external-artifact -->
*Proof:* a tool call from Claude Code, authenticated only by the Zitadel login, returns the Forgejo user. Profile fields other than those shown are elided as `…`. The deployment holds no operator token, so the only credential Forgejo can have accepted is the JWT that forgejo-mcp signed.

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
*Proof:* a handler calls Forgejo through the typed SDK client and the raw-HTTP helper. Both requests carry `token <jwt>` with the same JWT, whose `iss`, `sub` and `aud` the test checks, and neither carries the inbound token.

```bash
proof ./operation/ -run '^TestAToolCallReachesForgejoWithTheMintedTokenOnly$'
```

```output
--- PASS: TestAToolCallReachesForgejoWithTheMintedTokenOnly
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: forgejo-jwt-issuer#two-mcp-requests -->
**Proves:** [spec.md → Scenario: Two MCP requests](./spec.md#scenario-two-mcp-requests)

#### Scenario: Two MCP requests
- **WHEN** the same caller sends two MCP requests in succession
- **THEN** the JWTs presented to Forgejo for them SHALL differ in `jti`

<!-- evidence-kind: test-invocation -->
*Proof:* two JWTs minted for the same caller at the same instant differ in `jti`.

```bash
proof ./pkg/jwtissuer/ -run '^TestMintedTokensDifferInJTI$'
```

```output
--- PASS: TestMintedTokensDifferInJTI
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
```

<!-- spec-scenario: forgejo-jwt-issuer#claims-of-a-minted-jwt -->
**Proves:** [spec.md → Scenario: Claims of a minted JWT](./spec.md#scenario-claims-of-a-minted-jwt)

#### Scenario: Claims of a minted JWT
- **WHEN** a caller whose inbound token has `sub` `388443616722288641` and Forgejo audience `u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42` makes an authenticated request, with `-forgejo-jwt-issuer` set to `https://mcp.example.org/issuer`
- **THEN** the JWT presented to Forgejo SHALL have `iss` `https://mcp.example.org/issuer`, `sub` `388443616722288641`, and `aud` the string `u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42`
- **AND** its `iat` SHALL be 60 seconds before signing and its `exp` 300 seconds after
- **AND** it SHALL carry no `nbf`

<!-- evidence-kind: test-invocation -->
*Proof:* the test mints at Unix time 1789063466 and prints the decoded header and claims: exactly `aud`, `exp`, `iat`, `iss`, `jti` and `sub`, `aud` a single string, `iat` 60 s before and `exp` 300 s after signing, and no `nbf`.

```bash
proof ./pkg/jwtissuer/ -run '^TestMintedTokenShape$'
```

```output
    issuer_test.go:252: header {"alg":"ES256","kid":"uCcu3ARkHb5Vvk82Blha-UHVfLceUXlAjpT4cgTVr6g","typ":"JWT"}
    issuer_test.go:253: claims {"aud":"u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42","exp":1789063766,"iat":1789063406,"iss":"https://mcp.example.org/issuer","jti":"NN4dRdqVL7LNwiyBJHV7CA","sub":"388443616722288641"}
--- PASS: TestMintedTokenShape
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
```

<!-- spec-scenario: forgejo-jwt-issuer#forgejo-s-clock-is-slightly-behind -->
**Proves:** [spec.md → Scenario: Forgejo's clock is slightly behind](./spec.md#scenario-forgejo-s-clock-is-slightly-behind)

#### Scenario: Forgejo's clock is slightly behind
- **WHEN** Forgejo's clock is up to 60 seconds behind the server's clock
- **THEN** Forgejo SHALL accept the minted JWT, because its `iat` does not lie in Forgejo's future

<!-- evidence-kind: unit-test-output -->
*Proof:* the minted `iat` lies 60 s before signing, as the claims above show, so a Forgejo clock up to 60 s behind still sees it in the past. Forgejo applies no clock skew of its own.

```bash
proof ./pkg/jwtissuer/ -run '^TestMintedTokenShape$'
```

```output
    issuer_test.go:252: header {"alg":"ES256","kid":"c8E4lh1XcXsWRf6ZkXl2OZ96ADDO09lxVbik_E2ZDf0","typ":"JWT"}
    issuer_test.go:253: claims {"aud":"u:1:08759546-30f8-48f4-bd7e-b57dd7ddcd42","exp":1789063766,"iat":1789063406,"iss":"https://mcp.example.org/issuer","jti":"WY1tN0S2eJjDvZoWlAArpQ","sub":"388443616722288641"}
--- PASS: TestMintedTokenShape
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
```

<!-- spec-scenario: forgejo-jwt-issuer#claim-missing -->
**Proves:** [spec.md → Scenario: Claim missing](./spec.md#scenario-claim-missing)

#### Scenario: Claim missing
- **WHEN** a request carries a valid inbound token without a `forgejo_aud` claim
- **THEN** the server SHALL respond `403` with a body naming `forgejo_aud`
- **AND** Forgejo SHALL receive no request

<!-- evidence-kind: test-invocation -->
*Proof:* a valid token without `forgejo_aud` gets `403` with a body naming the claim and no challenge. Forgejo receives no request apart from the startup version probe.

```bash
proof ./operation/ -run '^TestATokenWithoutAUsableForgejoAudienceIsForbidden$/^claim_missing$'
```

```output
--- PASS: TestATokenWithoutAUsableForgejoAudienceIsForbidden
    --- PASS: TestATokenWithoutAUsableForgejoAudienceIsForbidden/claim_missing
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- evidence-kind: external-artifact -->
*Proof:* the same refusal against the deployment. A throwaway Zitadel user holding the project grant but no `forgejo_aud` metadata logged in through Claude Code. Zitadel issued an access token without the claim, and forgejo-mcp answered the first MCP request with `403` and the body naming it. Captured from Claude Code's log for that server entry, which records no token values.

```text
# 2026-09-13, times UTC. Claude Code 2.1.266, MCP server entry "chiba-forge-test" -> https://forgejo-mcp.byteflavour.dev/mcp
# Logged in through Zitadel as forgejo-mcp-test: role grant on project forgejo-mcp, no forgejo_aud metadata.
```

```output
13:37:29 Saving tokens
13:37:29 Token expires in: 43199
13:37:30 HTTP Connection failed after 163ms: Error POSTing to endpoint: Forbidden: the access token carries no usable "forgejo_aud" claim. It must hold the audience of your Forgejo Authorized Integration for this server.
```

<!-- evidence-kind: external-artifact -->
*Proof:* the server side of the same attempt. The deployment's access log shows the requests that carried the token: `403` with 149 bytes of `text/plain` and no `WWW-Authenticate` header. The proxy does not log bodies, but 149 bytes is exactly the length of forgejo-mcp's refusal text for the claim `forgejo_aud`. Captured on the deployment host; the `Authorization` header value is not reproduced here.

```text
# Caddy access log for forgejo-mcp.byteflavour.dev, 2026-09-13 since 13:00 UTC,
# requests carrying an Authorization header: time, method, path, status, size, WWW-Authenticate present, user agent
```

```output
13:37:30 POST /mcp 403 size=149 WWW-Authenticate=no claude-code/2.1.266 (cli)
13:37:30 POST /mcp 403 size=149 WWW-Authenticate=no claude-code/2.1.266 (cli)
13:37:30 POST /mcp 403 size=149 WWW-Authenticate=no claude-code/2.1.266 (cli)
13:37:30 POST /mcp 403 size=149 WWW-Authenticate=no claude-code/2.1.266 (cli)
```

<!-- spec-scenario: forgejo-jwt-issuer#claim-is-an-array -->
**Proves:** [spec.md → Scenario: Claim is an array](./spec.md#scenario-claim-is-an-array)

#### Scenario: Claim is an array
- **WHEN** a request carries a valid inbound token whose `forgejo_aud` is `["u:1:a", "u:2:b"]`
- **THEN** the server SHALL respond `403`

<!-- evidence-kind: test-invocation -->
*Proof:* a valid token whose `forgejo_aud` is `["u:1:a", "u:2:b"]` gets `403`.

```bash
proof ./operation/ -run '^TestATokenWithoutAUsableForgejoAudienceIsForbidden$/^claim_is_an_array$'
```

```output
--- PASS: TestATokenWithoutAUsableForgejoAudienceIsForbidden
    --- PASS: TestATokenWithoutAUsableForgejoAudienceIsForbidden/claim_is_an_array
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: forgejo-jwt-issuer#custom-claim-name -->
**Proves:** [spec.md → Scenario: Custom claim name](./spec.md#scenario-custom-claim-name)

#### Scenario: Custom claim name
- **WHEN** `-forgejo-audience-claim` is `urn:example:forgejo` and a valid inbound token carries that claim with a usable value
- **THEN** the minted JWT's `aud` SHALL be that value

<!-- evidence-kind: test-invocation -->
*Proof:* with the claim name `urn:example:forgejo`, the minted `aud` is the value of that claim.

```bash
proof ./operation/ -run '^TestCustomAudienceClaimName$'
```

```output
--- PASS: TestCustomAudienceClaimName
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: forgejo-jwt-issuer#p-256-key -->
**Proves:** [spec.md → Scenario: P-256 key](./spec.md#scenario-p-256-key)

#### Scenario: P-256 key
- **WHEN** the signing key file holds an EC P-256 private key
- **THEN** minted JWTs SHALL declare `alg` `ES256`

<!-- evidence-kind: showboat-cli -->
*Proof:* the deployment signs with an EC P-256 key; its key set and discovery document declare `ES256`.

```bash
curl -sS "$FORGEJO_MCP_LIVE/issuer/jwks.json" | jq -c '.keys[] | {kty, crv, alg}'
curl -sS "$FORGEJO_MCP_LIVE/issuer/.well-known/openid-configuration" | jq -c '.id_token_signing_alg_values_supported'
```

```output
{"kty":"EC","crv":"P-256","alg":"ES256"}
["ES256"]
```

<!-- evidence-kind: test-invocation -->
*Proof:* an EC P-256 key maps to `ES256`, and every other supported type to its one algorithm.

```bash
proof ./pkg/jwtissuer/ -run '^TestKeyTypeDeterminesAlgorithm$'
```

```output
--- PASS: TestKeyTypeDeterminesAlgorithm
    --- PASS: TestKeyTypeDeterminesAlgorithm/EC_P-256
    --- PASS: TestKeyTypeDeterminesAlgorithm/EC_P-384
    --- PASS: TestKeyTypeDeterminesAlgorithm/Ed25519
    --- PASS: TestKeyTypeDeterminesAlgorithm/RSA_2048
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
```

<!-- spec-scenario: forgejo-jwt-issuer#unsupported-key -->
**Proves:** [spec.md → Scenario: Unsupported key](./spec.md#scenario-unsupported-key)

#### Scenario: Unsupported key
- **WHEN** the signing key file holds an RSA key with a 1024-bit modulus
- **THEN** the server SHALL refuse to start
- **AND** the message SHALL name `-forgejo-jwt-signing-key-file`

<!-- evidence-kind: test-invocation -->
*Proof:* RSA-1024, EC P-521 and X25519 keys are refused with the file named; at startup, the refusal names `-forgejo-jwt-signing-key-file`.

```bash
proof ./pkg/jwtissuer/ ./operation/ -run '^(TestUnsupportedKeysAreRefusedNamingTheFile|TestPrepareResourceServerRefusesAnUnsupportedSigningKey)$'
```

```output
--- PASS: TestUnsupportedKeysAreRefusedNamingTheFile
    --- PASS: TestUnsupportedKeysAreRefusedNamingTheFile/RSA_1024
    --- PASS: TestUnsupportedKeysAreRefusedNamingTheFile/EC_P-521
    --- PASS: TestUnsupportedKeysAreRefusedNamingTheFile/X25519
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
--- PASS: TestPrepareResourceServerRefusesAnUnsupportedSigningKey
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: forgejo-jwt-issuer#forgejo-fetches-discovery -->
**Proves:** [spec.md → Scenario: Forgejo fetches discovery](./spec.md#scenario-forgejo-fetches-discovery)

#### Scenario: Forgejo fetches discovery
- **WHEN** `-forgejo-jwt-issuer` is `https://mcp.example.org/issuer` and a request without `Authorization` asks for `/issuer/.well-known/openid-configuration`
- **THEN** the server SHALL respond `200` with `issuer` `https://mcp.example.org/issuer` and `jwks_uri` `https://mcp.example.org/issuer/jwks.json`
- **AND** the document SHALL contain no other members

<!-- evidence-kind: showboat-cli -->
*Proof:* the deployment serves the discovery document under its issuer path, without authentication, with exactly three members.

```bash
curl -sS -D - -w '\n' "$FORGEJO_MCP_LIVE/issuer/.well-known/openid-configuration" | tr -d '\r' | grep -iE '^(HTTP/|cache-control:|\{)'
curl -sS "$FORGEJO_MCP_LIVE/issuer/.well-known/openid-configuration" | jq -c 'keys'
```

```output
HTTP/2 200 
cache-control: public, max-age=300
{"issuer":"https://forgejo-mcp.byteflavour.dev/issuer","jwks_uri":"https://forgejo-mcp.byteflavour.dev/issuer/jwks.json","id_token_signing_alg_values_supported":["ES256"]}
["id_token_signing_alg_values_supported","issuer","jwks_uri"]
```

<!-- evidence-kind: test-invocation -->
*Proof:* the tests assert exactly three members, the exact `issuer` and `jwks_uri`, and the headers under which they are served.

```bash
proof ./pkg/jwtissuer/ ./operation/ -run '^(TestDiscoveryDocumentHasExactlyThreeMembers|TestIssuerDocumentsAreServedUnderTheIssuerPath)$'
```

```output
--- PASS: TestDiscoveryDocumentHasExactlyThreeMembers
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
--- PASS: TestIssuerDocumentsAreServedUnderTheIssuerPath
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: forgejo-jwt-issuer#private-key-material-never-published -->
**Proves:** [spec.md → Scenario: Private key material never published](./spec.md#scenario-private-key-material-never-published)

#### Scenario: Private key material never published
- **WHEN** the key set is requested and a published key file contains a private key
- **THEN** the key set SHALL contain only that key's public parameters

<!-- evidence-kind: showboat-cli -->
*Proof:* the members of every key in the deployment key set: public parameters, `kid`, `use` and `alg`, and no private member such as `d`.

```bash
curl -sS "$FORGEJO_MCP_LIVE/issuer/jwks.json" | jq -c '.keys[] | keys'
```

```output
["alg","crv","kid","kty","use","x","y"]
```

<!-- evidence-kind: test-invocation -->
*Proof:* a published key given as a private key file is published with its public parameters only; the test checks for `d`, `p`, `q`, `dp`, `dq` and `qi`.

```bash
proof ./pkg/jwtissuer/ -run '^TestKeySetPublishesPublicParametersOfEveryKey$'
```

```output
--- PASS: TestKeySetPublishesPublicParametersOfEveryKey
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
```

<!-- spec-scenario: forgejo-jwt-issuer#key-published-ahead-of-use -->
**Proves:** [spec.md → Scenario: Key published ahead of use](./spec.md#scenario-key-published-ahead-of-use)

#### Scenario: Key published ahead of use
- **WHEN** a second key is listed in `-forgejo-jwt-published-key-files`
- **THEN** the key set SHALL contain both keys, each with a distinct `kid`
- **AND** minted JWTs SHALL still be signed with the signing key only

<!-- evidence-kind: test-invocation -->
*Proof:* with a second key published, the key set holds both key IDs, and a JWT minted afterwards still carries the `kid` and `alg` of the signing key.

```bash
proof ./pkg/jwtissuer/ -run '^TestKeySetPublishesPublicParametersOfEveryKey$'
```

```output
--- PASS: TestKeySetPublishesPublicParametersOfEveryKey
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/jwtissuer
```

<!-- spec-scenario: forgejo-jwt-issuer#startup-warning -->
**Proves:** [spec.md → Scenario: Startup warning](./spec.md#scenario-startup-warning)

#### Scenario: Startup warning
- **WHEN** the server starts in `resource-server` mode
- **THEN** the log SHALL contain one warning naming the `kid` and the issuer URL, and stating that the key acts for every user who trusts that issuer

<!-- evidence-kind: external-artifact -->
*Proof:* on the deployment host, the unit logged the warning at both of its starts in this boot. The `kid` is the one the public key set serves. zap colour codes are removed.

```bash
journalctl -u forgejo-mcp -b -o cat | grep 'signing key is a credential'
```

```output
2026-09-10 20:59:39	WARN	operation/authmode.go:202	The Forgejo signing key is a credential for every Forgejo user whose Authorized Integration trusts this issuer	{"kid": "8drpWDTAolL_YTHNlUPY3oQqh4YUcjZLOVu16cIHbM8", "issuer": "https://forgejo-mcp.byteflavour.dev/issuer", "forgejo_version": "16.0.3"}
2026-09-10 21:01:36	WARN	operation/authmode.go:202	The Forgejo signing key is a credential for every Forgejo user whose Authorized Integration trusts this issuer	{"kid": "8drpWDTAolL_YTHNlUPY3oQqh4YUcjZLOVu16cIHbM8", "issuer": "https://forgejo-mcp.byteflavour.dev/issuer", "forgejo_version": "16.0.3"}
```

<!-- evidence-kind: test-invocation -->
*Proof:* the test captures warnings during startup and requires the warning with the `kid` of the signing key and the configured issuer.

```bash
proof ./operation/ -run '^TestPrepareResourceServerSucceedsAndWarnsAboutTheSigningKey$'
```

```output
--- PASS: TestPrepareResourceServerSucceedsAndWarnsAboutTheSigningKey
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- spec-scenario: forgejo-jwt-issuer#minted-jwts-are-not-logged -->
**Proves:** [spec.md → Scenario: Minted JWTs are not logged](./spec.md#scenario-minted-jwts-are-not-logged)

#### Scenario: Minted JWTs are not logged
- **WHEN** debug logging is enabled and a tool call reaches Forgejo
- **THEN** the log SHALL NOT contain the minted JWT

<!-- evidence-kind: test-invocation -->
*Proof:* with logging captured at debug level, a tool call reaches Forgejo and the test scans every log entry for the minted JWT, its signature and both inbound tokens.

```bash
proof ./operation/ -run '^TestNeitherTokenIsLoggedAtAnyLevel$'
```

```output
--- PASS: TestNeitherTokenIsLoggedAtAnyLevel
ok  	git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation
```

<!-- evidence-kind: external-artifact -->
*Proof:* the deployment runs at info level, so this complements the test and does not replace it. Its whole journal, which includes two authenticated Claude Code sessions, holds no `eyJ`, the prefix of every JWT, and no debug line.

```bash
journalctl -u forgejo-mcp -o cat | grep -c 'eyJ'
journalctl -u forgejo-mcp -o cat | grep -o -E '(DEBUG|INFO|WARN|ERROR)' | sort | uniq -c
```

```output
0
     16 INFO
      3 WARN
```
