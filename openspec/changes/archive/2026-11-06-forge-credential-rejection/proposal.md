<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Proposal: report a forge-side credential rejection as an authentication failure

## Why

In `passthrough` mode, forgejo-mcp does not validate the caller's Forgejo token at the door (`stateless-http-auth`; a door check was deliberately not reintroduced in #588). Forgejo refuses an unknown, revoked or expired token on the first call that reaches it. forgejo-mcp then answers **HTTP 200** with the generic JSON-RPC error `-32603` and Forgejo's raw message (#701).

Two things go wrong:

- **OAuth MCP clients never re-authorize.** Claude, ChatGPT and others start a sign-in only on HTTP `401` with `WWW-Authenticate`, as the MCP authorization specification requires. A user whose token was revoked sees tool errors forever. A proxy in front of the server has to pattern-match the error text to recover.
- **The token is echoed.** Forgejo puts the presented credential verbatim into its rejection message (`access token does not exist [sha: <token>]`, `task with token "<token>": …`). Verified on Forgejo 16.0.5 and Codeberg; the v17.0.1 source is unchanged. forgejo-mcp relays that text into the JSON-RPC error and into its own error log. #683 redacts it on two paths (ephemeral client construction, raw-helper `HTTPError`), but SDK errors after a successful `NewClient` are not covered. That is the path where a rejection surfaces once the server version is known before the first call: already the case in `resource-server` mode, and proposed for every mode in #710. `stateless-http-auth` already requires that no logged error carries the token; `main` does not meet that today.

## What Changes

- **Detect the rejection where it happens, not by error text.** A Forgejo `401` to a request that carried the caller's per-request credential is recognised at the HTTP transport level. That covers every path to Forgejo: SDK calls, the SDK's version probe in `NewClient`, and the raw helpers.
- **Replace Forgejo's rejection body with a fixed message** before anything reads it. No error string, log line or JSON-RPC response can then carry the echoed credential, whichever code path produced it.
- **`passthrough` on `http`:** a tool call or resource read whose credential Forgejo rejected is answered with **HTTP `401`** and `WWW-Authenticate: Bearer error="invalid_token"`, still carrying the JSON-RPC error body.
- **Dedicated JSON-RPC error code `-32004`** ("Forgejo rejected the access token") for such calls, in every transport. It is the only signal where the HTTP status can no longer change: the `sse` transport, and a response the server has already started streaming. It is chosen from the implementation-defined range so that it collides with no code an MCP SDK already uses (`-32001` is the TypeScript and Python SDKs' request timeout).
- **`resource-server` mode:** when Forgejo rejects the JWT this server minted, the answer is **not** a `401`. The caller's sign-in is valid; the cause is server-side (for example a removed Authorized Integration or a key mismatch), and a challenge would send the client into a re-authorization loop that cannot succeed. The call fails with a fixed message naming that cause, and the operator gets a log line.
- **Unchanged:** `stdio`, any call made with the operator's own token, the door check, and the `401` that `resource-server` mode already returns for an invalid inbound token.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `stateless-http-auth`: adds how a forge-side rejection of the per-request credential is reported (`401` + challenge in `passthrough` on `http`, `-32004` everywhere, fixed message), and extends the existing "does not log the token" guarantee from ephemeral construction to every Forgejo response that rejects a per-request credential.
- `forgejo-jwt-issuer`: adds how a forge-side rejection of the minted JWT is reported in `resource-server` mode (no challenge, fixed server-side message, operator log line).

## Impact

- **Code:** `pkg/forgejo` (a rejection-observing `http.RoundTripper` for the ephemeral SDK client and the raw-HTTP client, and a per-request rejection marker carried in the context); `operation/operation.go` (the marker enters the context in `requestTokenContextFunc`, a tool and resource handler middleware maps a marked call to the dedicated error, and a `ResponseWriter` wrapper around the streamable-HTTP handler turns it into a `401`).
- **Builds on #683.** Its redaction stays as a second line of defence. If #683 has not merged when this is implemented, its commit is carried here with authorship kept.
- **Client-visible:** in `passthrough` mode on `http`, a rejected credential now yields `401` instead of `200`. Clients that already handle a `401` (all OAuth-capable MCP clients) re-authorize; others see the same failed call with a clearer message. Proxies can match `-32004` instead of error text.
- **No new flags, no door check, no extra Forgejo request.** Related: #701, #683, #588, #710, #582 / #584.
