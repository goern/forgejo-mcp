# Design

## Context

There are two startup paths that read the version today:

- **passthrough** (`operation/operation.go`, `Run`): every transport calls `testConnection` → `forgejo.VerifyConnection`. That builds the SDK singleton with `flag.Token` and calls `ServerVersion()`. Without a token, the request goes out anonymously, and the SDK's own version request inside `NewClient` already fails with `403` (observed: `create forgejo client: Only signed in user is allowed to call APIs.`).
- **resource-server** (`operation/authmode.go`, `prepareResourceServer`): `forgejo.ProbeServerVersion` makes a raw `GET /api/v1/version` with no credential, checks the major version against 16, and calls `forgejo.SetServerVersion`.

These facts were checked against Forgejo 15.0.11 and 16.0.7 with `REQUIRE_SIGNIN_VIEW=true`:

- `/api/v1/version` answers `403`;
- `/swagger.v1.json` answers `200` with `info.version` set;
- `/api/swagger` answers `200`;
- with `[api] ENABLE_SWAGGER=false`, `/swagger.v1.json` answers `303`, and `rawHTTPClient` follows that to an HTML page.

The swagger document is about 850 KB, and `info` is near its start.

## Goals / Non-Goals

**Goals:**

- One probe function that both paths use, and that #710 can extend.
- Tokenless passthrough keeps failing fast on a wrong URL.

**Non-Goals:**

- Storing the probed version in passthrough mode. Only `--forgejo-version` is recorded via `SetServerVersion` there. A probed version stays a log line, as today, because recording it would freeze the SDK's version across a Forgejo upgrade, and #710 owns that decision.
- Changing `HealthCheck`.

## Decisions

### D1: `ProbeServerVersion` returns a typed outcome

`ProbeServerVersion(ctx)` keeps its signature `(string, error)`. When Forgejo answered `401`/`403` on `/version` and the fallback read no version, the returned error wraps a sentinel, `ErrSignInRequired`, so callers can tell "Forgejo is there but closed" apart from "Forgejo is not usable" with `errors.Is`. The fallback runs only on `401`/`403`. Any other status, and any transport error, returns at once without touching swagger, so a wrong URL never looks like a closed instance.

*Alternative:* a result struct with an enum. That is more ceremony for two callers; the sentinel is the idiom already used in the package (`ErrNoRequestToken`, `ErrCrossOrigin`).

### D2: Swagger is decoded as a stream, bounded

The fallback requests `<instance root>/swagger.v1.json`. That is the root of `flag.URL`, not the `/api/v1` base, and it keeps a `ROOT_URL` sub-path. The request goes through `rawHTTPClient` with no `Authorization` header, so the rejection transport, the redirect policy and the TLS settings from #725 all apply unchanged. The body is read through `io.LimitReader` (4 MiB) with a `json.Decoder` that walks top-level tokens until it reaches `info`, decodes only that object, and stops. A non-JSON body (the `303` → HTML case), a missing or empty `info.version`, or hitting the limit all mean "no version".

*Alternative:* `/api/swagger` (the UI page). It is HTML, and parsing the version out of it is brittle.

### D3: Passthrough startup branches on configuration, not on the error text

In `Run`, before `testConnection`:

```text
--forgejo-version set           → SetServerVersion(v); skip the anonymous probe
token configured, or stdio/cli  → testConnection() as today (fatal)
http/sse and no token           → ProbeServerVersion:
                                    version     → log it
                                    ErrSignInRequired → warn, continue
                                    other error → fatal
```

With an operator token and `--forgejo-version` both set, `testConnection` still runs, because it checks the credential and not only the version. With the version recorded first, the SDK skips its own version request.

### D4: `--forgejo-version` is a general setting

It lives in `pkg/flag` next to `URL` and is resolved in `initConfig` with `resolveString(…, "FORGEJO_VERSION", "forgejo-version")`. It is not added to `ResourceServerSettings`, because passthrough mode would then refuse it. It is validated in `initConfig` with `forgejo.MajorVersion`, and an unparsable value is fatal there. That covers `--cli` and server mode, like the TLS settings. `prepareResourceServer` uses it instead of `ProbeServerVersion` when it is set.

### D5: Messages

- Passthrough warning: `Forgejo requires sign-in to read its version; starting without it. Every request still needs its own token.`, with field `url`.
- Resource-server refusal on `ErrSignInRequired`: `refusing to start: Forgejo at <url> requires sign-in to read its version and publishes no /swagger.v1.json; set --forgejo-version`.

## Risks / Trade-offs

- [A tokenless passthrough server starts against an instance whose credentials then all fail] → This is the same as today once it has started. Per-request errors already report Forgejo's refusal with `401`.
- [An operator sets a wrong `--forgejo-version`] → It only affects the floor check and the SDK's feature gates. In resource-server mode, Forgejo below 16 rejects the minted JWT anyway, because Authorized Integrations do not exist there. The startup log names the value and its source.
- [Swagger layout changes in a future Forgejo] → Only `info.version` is used, which is standard OpenAPI. If it disappears, the outcome is "no version", which is handled.

## Migration Plan

None. Deployments that start today behave the same. Deployments that fail today on sign-in-only instances now start, or get a message naming the setting. Rollback is reverting the release.
