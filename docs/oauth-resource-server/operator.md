<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Operating forgejo-mcp as an OAuth resource server

This guide is for whoever deploys forgejo-mcp with `--auth-mode resource-server`.
The people who use the deployment follow the [user guide](user.md).

## How the pieces fit

```text
MCP client  --(1) login ------------------------>  OpenID Connect provider
MCP client  --(2) Bearer <access token> -------->  forgejo-mcp  /mcp
forgejo-mcp --(3) token <JWT signed by it> ----->  Forgejo API
Forgejo     --(4) discovery document, key set -->  forgejo-mcp  /issuer/...
```

1. The MCP client logs the user in at your provider and receives a JWT access token.
2. forgejo-mcp validates that token:
   - the signature, with a key from the provider's key set;
   - the exact issuer and the expiry;
   - the audience;
   - that it is not an ID token.
3. It reads the user's Forgejo audience from a claim in the token. It signs a JWT that lives five minutes, with `iss` set to forgejo-mcp's issuer URL, `sub` to the provider's subject and `aud` to that audience, and calls Forgejo with it.
4. Forgejo finds the user's Authorized Integration by issuer and audience and checks its claim rules. It verifies the signature against forgejo-mcp's key set and applies the integration's permissions.

forgejo-mcp keeps no per-user state and holds no forge token. Its one secret is the signing key.

## Requirements

**Forgejo 16.0 or newer.** forgejo-mcp reads `/api/v1/version` at startup and refuses older versions.

**An OpenID Connect provider** that can:

- issue **JWT access tokens**, signed with an asymmetric algorithm. Some providers issue opaque access tokens by default, or make JWT a per-client setting.
- put a value into `aud` that identifies your MCP client, see `--resource-audience` below;
- add a **custom claim** to the access token from a per-user attribute. The attribute holds the audience of that user's Authorized Integration. Where the provider allows it, only administrators should be able to write it; see [The signing key and the sub rule](#the-signing-key-and-the-sub-rule).
- register a public client that uses PKCE with a fixed redirect URI, for MCP clients that cannot register themselves dynamically.

**A public HTTPS origin** for forgejo-mcp, such as `https://mcp.example.org`. Forgejo fetches the issuer documents from it, over https and without following redirects. MCP clients reach `/mcp` on the same origin.

**A signing key** as a PEM file: EC P-256 or P-384, Ed25519, or RSA of at least 2048 bits.

## Configuration

```bash
forgejo-mcp --transport http --url https://forgejo.example.org \
  --host 127.0.0.1 --http-port 8089 --allowed-hosts mcp.example.org \
  --auth-mode resource-server \
  --authorization-server https://id.example.org \
  --resource https://mcp.example.org/mcp \
  --resource-audience <client ID of the MCP client at the provider> \
  --scopes-supported "openid profile email" \
  --forgejo-jwt-issuer https://mcp.example.org/issuer \
  --forgejo-jwt-signing-key-file /run/credentials/forgejo-mcp.service/signing-key
```

Every setting also has an environment variable; the [README](../../README.md#configuration-options) lists them.

- **`--authorization-server`** is the provider's issuer URL, spelled exactly as its discovery document states it. At startup forgejo-mcp fetches `/.well-known/openid-configuration` under it, then the RFC 8414 location. It refuses to start if the `issuer` there differs by a single character, a trailing slash included. It then fetches the key set from the `jwks_uri` that document names, at startup and again whenever it refreshes the keys. That URL may be on another host than the issuer, as Google's is, so forgejo-mcp needs outbound https access to both, and trusting `--authorization-server` means trusting the `jwks_uri` it names.
- **`--resource`** is the canonical URL of the MCP endpoint. forgejo-mcp publishes it in the protected resource metadata at `/.well-known/oauth-protected-resource/mcp` and at `/.well-known/oauth-protected-resource`. Its path must be `/mcp`, where the endpoint is served; forgejo-mcp refuses to start otherwise, so serving the endpoint under another path behind a proxy is not supported.
- **`--resource-audience`** is the value an access token's `aud` must contain. It defaults to `--resource`, as the MCP specification asks, but many providers cannot put a URL there. Choose the narrowest value the provider offers, usually the client ID of the MCP client. A value shared by several clients, such as a project ID, admits tokens issued to all of them.
- **`--scopes-supported`** is published in the metadata and in the `401` challenge, and clients request these scopes at login. forgejo-mcp does not check scopes; the permissions of each Authorized Integration are the permission boundary.
- **`--forgejo-audience-claim`** names the access-token claim that holds the user's Forgejo audience. The default is `forgejo_aud`.
- **`--forgejo-jwt-issuer`** is the issuer URL users enter in their Authorized Integrations. It must use https, its host must be in `--allowed-hosts`, and it must not end with a slash or carry a query or fragment. Changing it later breaks every integration.
- **`--forgejo-jwt-signing-key-file`** and **`--forgejo-jwt-published-key-files`**: see [Keys and rotation](#keys-and-rotation).

forgejo-mcp refuses to start, before it binds a socket, when:

- the transport is not `http`, or `--cli` is used;
- an operator token (`--token`, `FORGEJO_ACCESS_TOKEN`) or `--allow-operator-token-fallback` is set;
- a required setting is missing, or a URL does not use https;
- the host of `--resource` or `--forgejo-jwt-issuer` is not in `--allowed-hosts`;
- the path of `--resource` is not `/mcp`;
- the key cannot be used;
- Forgejo is older than 16.0;
- the provider's metadata is unreachable, names another issuer or names no key set.

The message names the setting to fix.

## Deployment order

Forgejo validates the issuer when a user saves an integration. At that moment forgejo-mcp's discovery document must answer, and its key set must hold a key. So:

1. Start forgejo-mcp and publish it through the proxy.
2. Check the public documents and the challenge from outside:

   ```bash
   curl -sS https://mcp.example.org/.well-known/oauth-protected-resource/mcp
   curl -sS https://mcp.example.org/issuer/.well-known/openid-configuration
   curl -sS https://mcp.example.org/issuer/jwks.json
   curl -sS -o /dev/null -w '%{http_code}\n' -X POST https://mcp.example.org/mcp   # 401
   ```

3. Only then let users create their integrations.

## Reverse proxy

forgejo-mcp serves exactly these paths and answers `404` to every other:

- `/mcp`;
- `/.well-known/oauth-protected-resource` and `/.well-known/oauth-protected-resource/mcp`, where the second path follows `--resource`;
- `/issuer/.well-known/openid-configuration` and `/issuer/jwks.json`, where the prefix follows `--forgejo-jwt-issuer`.

The proxy must:

- **not redirect** any of these paths. That rules out trailing-slash normalisation and any redirect on the paths Forgejo fetches; serve https on them directly. Forgejo refuses redirects.
- **pass the `Host` header through unchanged**, because forgejo-mcp answers only hosts in `--allowed-hosts`.

Nothing needs to be served at the root `/.well-known/openid-configuration` of this host. Forgejo reads the discovery document under the issuer path, and MCP clients find the provider through the protected resource metadata. Leave the root path at `404`.

A Caddy site that does all of this:

```caddyfile
mcp.example.org {
	@forgejo_mcp path /mcp /.well-known/oauth-protected-resource /.well-known/oauth-protected-resource/mcp /issuer/.well-known/openid-configuration /issuer/jwks.json
	handle @forgejo_mcp {
		reverse_proxy 127.0.0.1:8089
	}
	handle {
		error 404
	}
}
```

**Ban tools.** Every new client connection begins with one `401` on `/mcp`, and every expired token adds another. If fail2ban, CrowdSec or a similar tool reacts to `401` responses, exempt `401` on `/mcp` of this host. If Forgejo reaches forgejo-mcp through its public name from the same machine, those requests come from that machine's own public address, so make sure the tool never bans it.

## Forgejo's network restrictions

Forgejo fetches the issuer documents through an HTTP client restricted by the `[authorized_integration]` section of `app.ini`. These are its settings and defaults in Forgejo 16.0.3:

```ini
[authorized_integration]
ALLOWED_DOMAINS =
BLOCKED_DOMAINS =
ALLOW_LOCALNETWORKS = false
REQUEST_TIMEOUT = 10s
CACHE_TTL = 10m
```

Forgejo's source applies them to every connection like this:

- With `ALLOWED_DOMAINS` empty, only public addresses are allowed. A host that resolves to a loopback address, a private address (RFC 1918, RFC 4193, RFC 6598) or another non-global address is refused.
- A non-empty `ALLOWED_DOMAINS` replaces that default. Only the listed host names, addresses and networks are allowed, and a listed host name is allowed whatever address it resolves to. The list applies to every integration on the instance.
- `ALLOW_LOCALNETWORKS = true` additionally allows private and loopback addresses, for every integration.
- `BLOCKED_DOMAINS` is checked as well and always wins.

So Forgejo refuses forgejo-mcp's documents when forgejo-mcp's name resolves to a local address on the Forgejo host. That happens when both share a machine and the name is pinned to `127.0.0.1`, or when split-horizon DNS or NAT returns a private address. The ways out, best first:

1. Let the name resolve to a public address on the Forgejo host. This needs no setting, and it is the only way exercised in a deployment so far.
2. List forgejo-mcp's host in `ALLOWED_DOMAINS`. Integrations on the instance can then only use the listed issuer hosts, so list every issuer your users need.
3. Set `ALLOW_LOCALNETWORKS = true`. It works, but it lets any user's integration make Forgejo fetch from internal addresses.

`CACHE_TTL` also governs key rotation, below.

## Keys and rotation

Generate an EC P-256 key and make it readable by its owner only:

```bash
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out signing.pem
chmod 0400 signing.pem
```

forgejo-mcp reads PKCS#8, SEC1 (`EC PRIVATE KEY`) and PKCS#1 encodings; it refuses encrypted keys. Supply the file through systemd `LoadCredential=`, which places it at `/run/credentials/<unit name>.service/<credential name>`, through a Kubernetes secret mount, or as a file readable only by the server's user. The key's `kid` is its RFC 7638 thumbprint and needs no configuration; the startup warning prints it.

**Rotation without downtime.** Forgejo caches forgejo-mcp's key set for `CACHE_TTL`. It fetches the key set again only on the first request after the cache has expired, and a JWT with an unknown `kid` does not make it fetch earlier.

1. Generate the new key, add it to `--forgejo-jwt-published-key-files` and restart. The key set now holds both keys, and JWTs are still signed with the old one.
2. Wait at least `CACHE_TTL` plus a margin: 15 minutes with the default of 10.
3. Make the new key `--forgejo-jwt-signing-key-file`, move the old key to `--forgejo-jwt-published-key-files`, and restart.
4. After five minutes, the lifetime of the last JWT signed with the old key, remove the old key and restart.

**After a suspected leak**, replace the signing key with a fresh one and remove the leaked key in one restart. Forgejo refuses JWTs signed with the new key until its cached key set expires, which is up to `CACHE_TTL` after Forgejo last fetched it. During that window forgejo-mcp cannot reach Forgejo for any user, and the leaked key is still in Forgejo's cache. A user who cannot wait can delete their integration: Forgejo then finds no integration for a JWT with that audience.

## The signing key and the sub rule

Whoever holds the signing key can sign a JWT that Forgejo accepts for every user whose Authorized Integration trusts `--forgejo-jwt-issuer`, with that integration's permissions. forgejo-mcp states this in a warning at every start. Guard the key like a forge administrator's token.

Two properties of each integration limit the damage. Both are set by the users, and the user guide makes them mandatory:

- **A `sub` claim rule.** Forgejo finds an integration by issuer and audience alone. Without a claim rule, anyone whose provider account carries a user's audience acts as that user. That can happen through a mistyped attribute, or through a provider that lets users edit it. The rule `sub eq <the user's subject>` closes that gap. forgejo-mcp cannot see integrations, so it cannot detect a missing rule.
- **Narrow permissions.**

On the provider side, make the audience attribute writable by administrators only, where the provider allows it.

## Token lifetimes

forgejo-mcp accepts an access token until its `exp`, so the provider's access-token lifetime is how long a stolen token grants access. Set it as short as your users tolerate. The JWTs that forgejo-mcp signs for Forgejo live five minutes and are never cached or reused.

## Logs

- Every start logs one warning with the signing key's `kid` and the issuer URL.
- Neither the inbound access token nor the JWT signed for Forgejo is logged, at any level.
- Requests refused for their `Host` or `Origin` header are logged as warnings, as in `passthrough` mode.
- Refused tokens are logged at debug level (`--debug`) with the reason. The client only ever sees the same `401`.
- Refusal lines are rate-limited.
- For accepted requests, the subject and the Forgejo audience are logged at debug level.

## Worked example: Zitadel

This example uses Zitadel 4. The per-user claim relies on Actions v1, which Zitadel plans to remove in version 5, so plan for that before upgrading.

### Project and application

1. Create a project for forgejo-mcp alone. Zitadel puts the client ID of every application in a project into `aud`, so sharing the project widens the audience.
2. Make the project require a role grant at login. In the Management API this is `projectRoleCheck`; the console labels it "Check authorization on authentication", or "Check Role Assignment on Authentication" in newer versions. Create a role such as `user` and grant it to the people who may use forgejo-mcp. The provider refuses the login of users without a grant. Zitadel 4.17.3 shows that refusal in the browser only as "unknown error", logs `Errors.User.GrantRequired`, and does not redirect back to the MCP client.
3. Add an application:
   - type **Native**;
   - authentication method **None**, which means PKCE and no client secret;
   - grant type authorization code;
   - access token type **JWT**;
   - the exact redirect URIs of the MCP client. For Claude Code with a pinned callback port those are `http://localhost:41998/callback` and `http://127.0.0.1:41998/callback`.

Access tokens then carry `aud` with the client ID and the project ID. Start forgejo-mcp with `--resource-audience <client ID>` and with `--authorization-server` set to the Zitadel instance URL.

Zitadel's ID tokens carry the same `aud` and the same `typ` header as its access tokens. forgejo-mcp tells them apart because ID tokens carry `nonce` or `at_hash`, and it refuses them.

Access and ID token lifetimes are instance-wide OIDC settings, with a default of 12 hours.

### The forgejo_aud claim

Store each user's Forgejo audience as user metadata with the key `forgejo_aud`. Through the Management API, the value is sent base64-encoded:

```bash
curl -sS -X POST "https://id.example.org/management/v1/users/<user ID>/metadata/forgejo_aud" \
  -H "Authorization: Bearer <token of an account allowed to manage users>" -H 'Content-Type: application/json' \
  -d "{\"value\": \"$(printf '%s' 'u:1:<uuid>' | base64 | tr -d '\n')\"}"
```

Then create an action that copies the metadata into the access token:

- its name must equal the function name;
- give it a short timeout, such as 2 seconds;
- allow it to fail, because it runs for every access token in the organisation.

```js
function forgejoAud(ctx, api) {
  // The flow runs for every access token in the organisation.
  // Act only for the forgejo-mcp application.
  if (ctx.v1.application.getClientId() !== '<client ID>') {
    return;
  }
  var md = ctx.v1.user.getMetadata();
  var list = (md && md.metadata) ? md.metadata : [];
  for (var i = 0; i < list.length; i++) {
    if (list[i].key === 'forgejo_aud') {
      api.v1.claims.setClaim('forgejo_aud', list[i].value);
      return;
    }
  }
}
```

Attach it to the flow **Complement Token**, trigger **Pre access token creation**. Through the API, the call that sets a trigger's actions takes the complete list, so read the list first and include the actions already on it.

The action runs when an access token is issued. A user whose metadata was set after their last login gets the claim with their next token; logging in again is the sure way to get one.

### forgejo-mcp and Forgejo

Start forgejo-mcp as in [Configuration](#configuration), with `--resource-audience` set to the application's client ID and `--scopes-supported "openid profile email"`. Zitadel requires `openid`.

Users create their integrations as the [user guide](user.md) describes. A Forgejo administrator can also create one on the server:

```bash
forgejo admin user create-authorized-integration \
  --username <Forgejo user> --name forgejo-mcp \
  --issuer https://mcp.example.org/issuer \
  --claim-eq sub=<Zitadel user ID> \
  --scope read:repository --scope read:issue
```

Always pass `--scope`. Without it, the integration gets the scope `all`, and without `--repo` it covers all repositories.

The command outputs the audience Forgejo generated. Store that as the user's `forgejo_aud` metadata.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| The start is refused with a message naming a setting | That setting. The checks run before anything is bound. |
| The client logs in, and `/mcp` still answers `401` | forgejo-mcp refuses the token. Run with `--debug` and read the refusal reason. Common causes: `--resource-audience` does not match `aud`, the issuer is spelled differently, or the access token is opaque. |
| `403` with a message naming the audience claim | The token carries no usable audience claim. The metadata is missing, the action is not attached, or the token was issued before the metadata was set. |
| Tool calls fail with an authentication error from Forgejo | Forgejo refuses the JWT. There is no integration with this issuer and audience, its `sub` rule names another subject, or Forgejo has not yet fetched a new key (see [Keys and rotation](#keys-and-rotation)). |
| Tool calls fail with a missing-scope error from Forgejo | The integration's permissions do not cover the tool. |
| Saving an integration fails with "Issuer validation failed" | Forgejo cannot fetch the discovery document or the key set. forgejo-mcp is not reachable yet, the proxy redirects, or the [network restrictions](#forgejos-network-restrictions) apply. |
