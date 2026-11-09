<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Proposal

## Why

forgejo-mcp refuses to start against a Forgejo instance that requires sign-in (`[service] REQUIRE_SIGNIN_VIEW = true`) in exactly the two modes meant for many users: tokenless passthrough on `http`/`sse`, and `resource-server`. Both read the version anonymously from `/api/v1/version` at startup, and such an instance answers `403` there (#729, reproduced on Forgejo 15.0.11 and 16.0.7). On Kubernetes the result is a `CrashLoopBackOff`. In `resource-server` mode there is no workaround, because the mode refuses an operator token by design.

The same instances still serve `/swagger.v1.json` anonymously, and its `info.version` carries the exact version (`16.0.7+gitea-1.22.0`), so the version is readable without a credential unless swagger is disabled as well.

## What Changes

- **One anonymous version probe** for every caller: `GET /api/v1/version`. If Forgejo answers `401` or `403` with a Forgejo API error, the probe reads `info.version` from `/swagger.v1.json` instead, with a bounded read, and trusts it only from the instance itself. No credential is sent on either request.
- **New setting `--forgejo-version` / `FORGEJO_MCP_FORGEJO_VERSION`**, optional, valid in every mode. When set, forgejo-mcp does not read the version: one anonymous request to `/api/v1/version` only checks that `--url` reaches Forgejo, and a different reported version is logged as a warning. It uses the value for the `resource-server` floor check and passes it to the SDK in every mode, `--cli` included, so clients skip the SDK's own version request. A value the SDK cannot parse refuses startup.
- **Tokenless passthrough on `http`/`sse`** no longer builds an SDK client at startup. It runs the probe instead:
  - if a version is read, it is logged;
  - if Forgejo answers `401`/`403` with a Forgejo API error and the swagger fallback fails too, a warning is logged and the server starts. Every request still needs its own token, as before;
  - if Forgejo cannot be reached (DNS, connection, TLS) or answers anything else, including a `401`/`403` that is not a Forgejo API error (a proxy, a WAF), startup still fails, so a wrong URL is caught at once. This holds with `--forgejo-version` set too.
- **`resource-server` mode** uses the same probe. If it reads no version and `--forgejo-version` is unset, startup is refused with a message that names `--forgejo-version`.
- Unchanged: `stdio`, `--cli`, and any configuration with an operator token keep today's authenticated connection check, and its failure stays fatal.

### Non-goals

- **The `401` message rewrite.** The reporter saw "Forgejo rejected the access token". We checked against a server that answers every request with `401`: without a configured token the message is Forgejo's own text; the rewrite happens only when a token was sent, as the `stateless-http-auth` spec already requires. The reporter's process therefore had some token configured. No change is needed here.
- Re-probing after startup, and probing more than the version (that is #710).

## Capabilities

### New Capabilities

- `forgejo-version-detection`: how forgejo-mcp learns the version of the Forgejo instance it serves, namely the anonymous probe, the swagger fallback, and the `--forgejo-version` setting that replaces both. #710 extends this capability with API detection later.

### Modified Capabilities

- `stateless-http-auth`: adds what startup does on `http`/`sse` in passthrough mode without an operator token. When Forgejo requires sign-in, the server starts with a warning instead of exiting.
- `oauth-resource-server`: the "Forgejo version" refusal now applies only when neither the probe nor `--forgejo-version` yields a version, and its message names the setting.

## Impact

- `pkg/forgejo/version.go`: swagger fallback in `ProbeServerVersion`, a typed "sign-in required" error.
- `operation/operation.go`: startup branch for tokenless `http`/`sse`.
- `operation/authmode.go`: `prepareResourceServer` honours `--forgejo-version`.
- `cmd/cmd.go`, `pkg/flag/flag.go`: the new setting and its validation.
- `README.md` (configuration table), `docs/oauth-resource-server/operator.md` (requirements, refusal list).
- No new module: `github.com/hashicorp/go-version`, already in the graph through the SDK, becomes a direct dependency so the setting is validated with the parser the SDK uses. No change for stdio users or for deployments with an operator token.
