# Design

## Context

See proposal.md for the motivation. Since #712, two package-level clients in `pkg/forgejo` carry all traffic to Forgejo:

- `sdkHTTPClient` (`rejection.go`): every SDK client is built with it, both the per-request ephemeral clients and the operator-token singleton.
- `rawHTTPClient` (`rawhttp.go`): the raw-HTTP helpers (`DoJSON`, `DoJSONList`, `DoAPIRaw`, `DoMultipart`) and `ProbeServerVersion`.

Both use a `rejectionTransport{base}` whose `base` is nil, which means `http.DefaultTransport`. Nothing else talks to Forgejo. `pkg/oauthrs` fetches the identity provider's documents with its own client; that is not a Forgejo connection.

#232 built a new `http.Client` inside `Client()` on every ephemeral call, so each call re-read the certificate. It covered only the SDK path, and it `log.Fatalf`ed from inside `Client()`.

Open PR #720 rewrites the two client declarations (it adds `CheckRedirect`).

## Goals / Non-Goals

**Goals:**

- One TLS transport, built once at startup, under both shared clients.
- All validation at startup, in the same place the other settings are validated.
- Tests against a real TLS server that demands a client certificate, with no key material committed to the repository.

**Non-Goals:**

- Certificate rotation without a restart (`GetClientCertificate` with reload). Static certificates were the use case in #232. A follow-up issue can add it.
- Inline PEM through environment variables.
- TLS settings for the identity-provider fetches in resource-server mode, or for the listener side (inbound TLS stays with the reverse proxy).
- An "insecure" or skip-verify option. The spec rules it out.

## Decisions

### D1: Install the transport as the base of the existing clients, at runtime

`pkg/forgejo` gets a `ConfigureTLS(cert, key, ca string) (*TLSSummary, error)`. With no settings it returns nil and changes nothing. Otherwise it builds the `tls.Config`, clones `http.DefaultTransport`, sets `TLSClientConfig`, and assigns `&rejectionTransport{base: t}` to both `sdkHTTPClient.Transport` and `rawHTTPClient.Transport`.

The clone keeps proxy settings from the environment, HTTP/2, the pool, and the dial timeouts. This was #232's second review point.

The assignment happens at startup, before any request. So the shared clients stay shared: ephemeral SDK clients keep reusing one connection pool, and the certificate is read once.

- *Alternative:* change the two `var` declarations to build the transport. That conflicts with #720, and it would need the flags at package init, which is before they are parsed.
- *Alternative:* a `SetHTTPClient` per SDK client, as #232 did. It misses the raw path and re-reads the certificate per call.

### D2: Validate in `cmd.initConfig`, fail with `log.Fatal`

`initConfig` resolves the three settings with the existing `resolveString` (flag wins when passed, else env). It then calls `forgejo.ConfigureTLS` and turns an error into `log.Fatal` with the setting named. `initConfig` runs in server mode and in `--cli` mode, so both are covered. In CLI mode no flags are parsed, so only the environment variables apply there. That matches every other setting.

The pairing checks (cert without key, http URL) are plain functions that are tested directly. `ConfigureTLS` returns errors and never exits, so tests can call it.

### D3: `-tls-ca` replaces the system roots

`RootCAs` is set to a pool that holds only the bundle's certificates, as `curl --cacert` does. An operator who names a CA is pinning the instance's issuer. Adding it to the system pool would also accept any public CA for that hostname, which is weaker and not what "trust this CA" means. An operator who needs both can concatenate the bundle.

### D4: The expiry is checked and logged at startup

Without rotation, an expired certificate turns into handshake failures on every call. These are hard to tell apart from other TLS errors in a tool result. Refusing to start with "expired at …" is clearer.

The startup log line records `client_cert_configured`, the leaf's `not_after`, and `ca_configured`. It never logs the key or the PEM content. Paths are logged at debug level only.

### D5: Tests generate their PKI at run time

A test helper creates a CA, a server certificate for `127.0.0.1`, and a client certificate (ECDSA P-256) in `t.TempDir()`, and writes PEM files there. `httptest.NewUnstartedServer` with `ClientAuth: tls.RequireAndVerifyClientCert` stands in for an mTLS Forgejo, and its handler answers `/api/v1/version` and `/api/v1/user`.

No key is committed. gitleaks scans the whole history (#712), so a committed test key would have to be allow-listed for good.

Tests that call `ConfigureTLS` restore both transports in `t.Cleanup` and do not run in parallel, because they swap package state.

## Risks / Trade-offs

- [Package-level transport swap] → It is done once, before serving. Tests restore it in `t.Cleanup`. A comment marks `ConfigureTLS` as startup-only.
- [#720 merges first] → No textual conflict, because the declarations are not touched. #720's `CheckRedirect` is on the client, not the transport, so the two compose.
- [`-tls-ca` users relying on system roots too] → Documented in the README row. Concatenating bundles is the escape hatch.
- [Certificate expires while running] → Calls fail with a TLS error until the operator restarts. Documented, and rotation is a named follow-up.

## Migration Plan

Additive. With no setting given, behaviour is unchanged. No rollback beyond unsetting the variables.
