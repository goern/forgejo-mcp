<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## ADDED Requirements

### Requirement: Network transports bind loopback by default

The `sse` and `http` transports SHALL bind the address given by `--host` /
`FORGEJO_MCP_HOST`, defaulting to `127.0.0.1`. They SHALL NOT bind the unspecified
address unless the operator asks for it.

When the configured address is a loopback name, the server SHALL listen on both
loopback families, so that a client resolving `localhost` to `::1` can connect. Failure
to bind one family SHALL NOT prevent startup when the other succeeded.

The startup log SHALL state the address actually bound and who can reach it. It SHALL
NOT print a fixed `localhost` URL.

#### Scenario: Default bind is loopback only

- **WHEN** the server starts on `http` with no `--host`
- **THEN** it SHALL listen only on loopback addresses
- **AND** the log SHALL say the service is reachable from this machine only

#### Scenario: A network-reachable bind requires declared hosts

- **WHEN** the server is configured to bind an address the network can reach
- **AND** no allowed hosts are declared
- **THEN** the server SHALL refuse to start, naming the option that fixes it
- **AND** it SHALL refuse before binding, so a misconfigured start never opens a
  public socket

### Requirement: Host and Origin are validated on every request

Both network transports SHALL reject a request whose `Host` header is not permitted,
with `403`. Permitted values are the declared allowed hosts, plus any loopback name
when the listener is loopback-only.

Both SHALL reject a request that carries an `Origin` header naming an origin that was
not declared, with `403`. A present-but-empty `Origin` SHALL be treated as present, not
absent. A request carrying no `Origin` header SHALL be accepted, since a non-browser
client sends none and the `Host` check still applies to it.

Origins SHALL be compared as full origins — scheme, host and port, with the default
port for the scheme normalised away. They SHALL NOT be compared as bare hostnames: an
`Origin`'s port belongs to the requesting page, not to this listener.

The allowed-hosts and allowed-origins lists SHALL be separate.

#### Scenario: Forged Host is refused

- **WHEN** a request arrives over loopback carrying `Host: attacker.example.com`
- **THEN** the server SHALL respond `403`

#### Scenario: Cross-site Origin is refused despite a correct Host

- **WHEN** a request arrives with a permitted `Host` and an undeclared `Origin`
- **THEN** the server SHALL respond `403`

#### Scenario: OPTIONS * does not bypass the checks

- **WHEN** a request arrives as `OPTIONS *` with a forged `Host`
- **THEN** the server SHALL respond `403` rather than answering from the runtime's
  built-in handler
