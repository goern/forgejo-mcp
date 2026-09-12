# network-transport-binding Specification

## Purpose

Decide what the `sse` and `http` transports expose on the network: the address they
bind, the `Host` and `Origin` values they answer to, and how a loopback family that
cannot be bound is treated. Both bind loopback by default and refuse to start on a
network-reachable address unless the operator declares the host names clients will use,
so a misconfigured start never opens a public socket. A loopback name binds both
families, because a client that resolves `localhost` to either one must reach the same
server; only a family this machine cannot use at all is skipped, and any other bind
failure refuses the start.

## Requirements

### Requirement: Network transports bind loopback by default

The `sse` and `http` transports SHALL bind the address given by `--host` /
`FORGEJO_MCP_HOST`, defaulting to `localhost`. They SHALL NOT bind the unspecified
address unless the operator asks for it.

When the configured value is a loopback NAME, the server SHALL listen on both loopback
families, so that a client resolving `localhost` to `::1` can connect. When it is a
loopback ADDRESS, the server SHALL listen on that address alone: naming one family is
how an operator asks for one family.

A family this machine cannot use — IPv6 disabled, or absent from the kernel — SHALL NOT
prevent startup when the other family bound, whichever family is the missing one. Any
other failure to bind either family, including an address already in use, SHALL prevent
startup, and every listener already bound SHALL be closed. A refusal naming an address
the operator did not configure SHALL state that a loopback name binds both families, and
SHALL name the setting that binds one.

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

#### Scenario: An unavailable loopback family does not block startup

- **WHEN** the server starts on a loopback address
- **AND** this machine cannot use one loopback family, whichever one it is
- **THEN** the server SHALL start on the other family
- **AND** it SHALL log the skipped family at a level the default configuration shows

#### Scenario: A loopback address binds only the family it names

- **WHEN** the server starts with `--host ::1`
- **THEN** it SHALL bind `::1` alone
- **AND** it SHALL start even when `127.0.0.1` could not be bound

#### Scenario: A loopback port taken on either family refuses to start

- **WHEN** the server starts on a loopback address
- **AND** binding one loopback family fails because the address is already in use
- **THEN** the server SHALL refuse to start rather than serve on the other family alone
- **AND** it SHALL close any listener it had already bound

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
