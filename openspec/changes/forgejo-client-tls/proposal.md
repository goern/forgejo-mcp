# Proposal

## Why

Some Forgejo instances sit behind mutual TLS: the TLS terminator in front of them refuses any client that does not present a certificate, often one from a private CA. forgejo-mcp cannot talk to such an instance at all today, because it has no way to present a client certificate or to trust a private CA. #232 added this in June, was reviewed twice, and is now abandoned (its author's account is gone). Its branch is 1020 commits behind `main`, so this change re-does it on today's client code, keeping the review's to-do list as the scope.

## What Changes

- New settings `-tls-cert` / `FORGEJO_TLS_CERT` and `-tls-key` / `FORGEJO_TLS_KEY`: a PEM client certificate and key that forgejo-mcp presents to Forgejo.
- New setting `-tls-ca` / `FORGEJO_TLS_CA`: a PEM CA bundle that forgejo-mcp trusts for Forgejo's server certificate instead of the system roots. Usable on its own (private CA, no client certificate) or together with the client certificate.
- Every connection to Forgejo uses this configuration: SDK calls with a caller's token or the operator token, the raw-HTTP helpers (wiki, attachments, `DoJSON` tools), and the startup version probe. #232 covered only the SDK path.
- The settings are validated once at startup. A half-configured or unusable setting stops the server with a clear error and never falls back to plain TLS.
- Unset, behaviour is unchanged.
- Out of scope (agreed in #232): reloading a rotated certificate without a restart, and PEM content passed inline through an environment variable.

## Capabilities

### New Capabilities

- `forgejo-client-tls`: how forgejo-mcp authenticates itself at the TLS layer to the Forgejo instance it talks to (client certificate and trusted CA). This covers settings, startup validation, and which connections use them.

### Modified Capabilities

None. The rejection handling from `stateless-http-auth` stays as it is; the TLS configuration sits underneath it.

## Impact

- `cmd/cmd.go`: three flags, env resolution, and startup validation.
- `pkg/flag`: three settings.
- `pkg/forgejo`: one TLS transport built at startup and set as the base of the existing shared clients' transport. The client declarations themselves stay as they are (open PR #720 edits those lines).
- README "Configuration Options": three rows and a short mTLS section.
- No new dependencies (standard library `crypto/tls`, `crypto/x509`).
- Credit: Paul Berz (#232) as `Co-authored-by` on the implementation commit.
