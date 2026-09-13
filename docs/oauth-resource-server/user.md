<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Connecting to forgejo-mcp with an OAuth login

This guide is for people who connect an MCP client to a forgejo-mcp deployment that runs in `resource-server` mode. You log in with your identity provider, and you never create a Forgejo token for the MCP client. The operator's side is in the [operator guide](operator.md).

You set this up once, in three steps:

1. Create an Authorized Integration in Forgejo that trusts forgejo-mcp and only you.
2. Store the audience that Forgejo generates in your account at the identity provider.
3. Configure your MCP client.

## What you need from your operator

- the **issuer URL** of the deployment, for example `https://mcp.example.org/issuer`;
- the **MCP endpoint**, for example `https://mcp.example.org/mcp`;
- the **client ID** to use at the identity provider, and the callback port if the provider requires a fixed one;
- your **subject**, the `sub` value in your tokens (in Zitadel, your user ID);
- how the audience gets into your account in step 2, and the claim name if it is not `forgejo_aud`.

## 1. Create the Authorized Integration

In Forgejo, open your user settings, go to **Authorized Integrations** (`/user/settings/authorized-integrations`), and add an integration of type **Generic JWT**:

- **Name**: anything you recognise, such as `forgejo-mcp`.
- **Issuer (`iss` Claim)**: the issuer URL from your operator, character for character.
- **Claim Rules JSON**: a rule on your subject. It is required, see below.

  ```json
  {"rules": [{"claim": "sub", "compare": "eq", "value": "<your subject>"}]}
  ```

- **Capabilities Permitted**: only what the MCP client needs, see below.

Forgejo checks the issuer when you save, by fetching forgejo-mcp's discovery document and key set. If saving fails with "Issuer validation failed", tell your operator.

After saving, Forgejo shows the integration's **audience** (`aud` claim), which looks like `u:<number>:<uuid>`. Copy it for step 2. Deleting the integration and creating a new one generates a different audience.

### Why the sub rule is mandatory

Forgejo finds your integration by issuer and audience alone. forgejo-mcp signs for whoever logged in, with the audience taken from that person's account at the identity provider. Without a `sub` rule, anyone whose account carries your audience acts as you in Forgejo. That can happen through an administrator's typo, or through a provider that lets users edit the attribute.

With the rule, Forgejo refuses every subject but yours. forgejo-mcp cannot see your integration, so it cannot warn you when the rule is missing.

### Choosing permissions

Once you are logged in, forgejo-mcp lets you call every tool. The integration's permissions are the only limit on what the MCP client can do as you. Start narrow, for example with `read:repository` and `read:issue` to read code, pull requests and issues, plus `read:user` so that tools such as `get_my_user_info` can see who you are. A tool that needs more fails with an error from Forgejo naming the scope it requires, and you can widen the integration then. You can also restrict the integration to selected repositories.

## 2. Store the audience at the identity provider

Your identity provider has to put the audience into your access token, as the claim `forgejo_aud` unless your operator named another. How you store it depends on the provider, and on many deployments an administrator does it for you. On Zitadel it is user metadata with the key `forgejo_aud`.

The claim is added when a token is issued. If you stored the audience after your last login, log in again.

## 3. Configure your MCP client

forgejo-mcp publishes where to log in, as RFC 9728 protected resource metadata, so a client that implements MCP authorization finds the identity provider by itself. Most identity providers do not let clients register themselves. The client then needs the pre-registered client ID, and a callback address that matches the registered redirect URI exactly.

For Claude Code, add the server to `.mcp.json`:

```json
{
  "mcpServers": {
    "forgejo-remote": {
      "type": "http",
      "url": "https://mcp.example.org/mcp",
      "oauth": {
        "clientId": "<client ID>",
        "callbackPort": 41998,
        "scopes": "openid profile email"
      }
    }
  }
}
```

Then run `/mcp` in Claude Code and authenticate the server. Your browser opens the provider's login. The configuration holds no credential; Claude Code keeps the token in its own store.

## Troubleshooting

| What you see | What to do |
| --- | --- |
| The identity provider refuses your login, possibly as "unknown error" | You have no access to the application. Ask your operator for a grant. |
| The login succeeds, but the client reports `401` again | forgejo-mcp refuses your token. Ask your operator to check the debug log; a common cause is a client ID other than the one the deployment expects. |
| `403` with a message naming `forgejo_aud` | Your token carries no audience. Store it as in step 2, then log in again. |
| A tool fails with a Forgejo error about a missing scope | Widen the integration's permissions. |
| Every tool fails with a Forgejo authentication error | The integration does not match. Check the issuer spelling and the value in the `sub` rule, and whether the stored audience belongs to a deleted integration. |
