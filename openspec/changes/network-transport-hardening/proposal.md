<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

The `sse` and `http` transports call `Start(fmt.Sprintf(":%d", port))`. A listen
address with no host binds every network interface, so both transports are reachable
from the local network by default, while the startup log line reads
`http://localhost:<port>` — telling the operator the service is local when it is not.

Neither transport validates `Host` or `Origin`. The Model Context Protocol makes
`Origin` validation a MUST for the Streamable HTTP transport and a loopback bind a
SHOULD, for exactly this reason. `mcp-go` v0.58.0 does ship DNS-rebinding protection,
on by default, but it only fires when the connection's local address is loopback; with
the listener on every interface it does not apply to a request arriving on a network
interface. The library's default is correct for a loopback-bound server, and this
server is not one.

Compounding both: `Client(ctx)` and `setCommonHeaders` fall back to the server's own
configured credential when a request carries no `Authorization` header. That fallback
came in with per-request authentication, and its own description says it exists
"preserving backward compatibility for `--stdio` mode" — but it lives in a factory
shared by every transport, so on `sse` and `http` an unauthenticated request is served
using the operator's credential.

Together these mean the two network transports would hand any host that can reach the
port unauthenticated use of the operator's forge credential: issues, pull requests,
file writes, branch protection, releases. This is latent rather than live — it is
reachable only by selecting a transport that is not the default.

## What Changes

- **New `--host` / `FORGEJO_MCP_HOST`**, default `127.0.0.1`. A loopback value binds
  both loopback families, so a client that resolves `localhost` to `::1` still
  connects.
- **New `--allowed-hosts` / `FORGEJO_MCP_ALLOWED_HOSTS`.** Required when the bind
  address is not loopback: the server refuses to start without it rather than starting
  and rejecting every request. Loopback names remain acceptable on a loopback listener
  whatever else is declared, so declaring a proxy's name does not lock out the
  operator's own machine.
- **New `--allowed-origins` / `FORGEJO_MCP_ALLOWED_ORIGINS`**, deliberately separate
  from the host list. A `Host` names this server; an `Origin` names the page making
  the request. One list for both would accept every other service sharing a declared
  hostname on any port, and every plaintext origin on it. Empty — the default — means
  no browser origin is accepted. A request carrying no `Origin` at all is unaffected.
- **Both transports go through one listener.** Neither calls the library's `Start`, so
  the bind address, the header checks and the credential policy are decided in exactly
  one place. The streamable HTTP transport is mounted on a mux at its endpoint path,
  as the library's own `Start` does, so it does not begin answering on every path.
- **The credential fallback is refused on `sse` and `http`**, and a request with no
  usable `Authorization` header is refused at the door with `401` rather than at the
  forge client — so an anonymous caller cannot open a session, enumerate the tool
  catalogue, or hold an event stream open either. `stdio` is untouched.
- **New `--allow-operator-token-fallback`**, off by default, for an operator who
  genuinely wants the old behaviour on a network transport.

## Why the credential policy does not follow the bind address

Keying the credential policy to the **bound address** — keeping the fallback available
on a loopback-only listener, on the reasoning that the reachable set is then the same as
`stdio`'s — is an attractive idea that does not hold. It is recorded here because it
will be proposed again otherwise:

- A loopback TCP port is reachable by every local user account and every sandboxed
  process on the machine. `stdio` is reachable only by the process that spawned the
  server. They are not the same boundary.
- A reverse proxy in front of a loopback listener makes the listener's own address say
  nothing about who can reach it. nginx and Apache both rewrite `Host` to the proxied
  target by default, so a request from the internet arrives on a loopback socket
  carrying a loopback `Host`. Every observable the server can check looks local, and a
  remote anonymous request would be served with the operator's credential — verified
  end to end against a proxy in that configuration, which is now a standing test.

The second point has the worse shape. A proxy configured to preserve the original
`Host` is refused, and the operator fixes it by declaring the name; the **default**
configuration silently works and is the insecure one. A control whose failure mode
selects for the vulnerable deployment is not a control. So the transport decides the
credential policy, and the bind address decides only whether an allow-list is
required.

## Impact

This is a breaking change for deployments that use `sse` or `http`. `stdio` — the
default, and what the packaged editor extension uses — is unaffected in every respect.

- A container publishing a port must now set `--host 0.0.0.0` and declare
  `--allowed-hosts`.
- Every client of a network transport must send its own `Authorization` header, or the
  operator must set `--allow-operator-token-fallback` and accept what it means.

The version implication needs a maintainer decision: the module path is `/v2`, and a
`BREAKING CHANGE` footer would have the release tooling cut `v3.0.0`, which `go get`
cannot consume on a `/v2` path. The footer is deliberately omitted from the commit for
that reason, and the breakage is described in prose instead.
