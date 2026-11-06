# Spec Delta

## Purpose

Lets forgejo-mcp reach a Forgejo instance that requires mutual TLS or uses a private CA, by presenting a client certificate and trusting an operator-supplied CA.

## ADDED Requirements

### Requirement: TLS settings are configurable by flag and environment

The server SHALL accept a client certificate path (`-tls-cert` / `FORGEJO_TLS_CERT`), a client key path (`-tls-key` / `FORGEJO_TLS_KEY`) and a CA bundle path (`-tls-ca` / `FORGEJO_TLS_CA`), all PEM files. A flag that is passed SHALL take precedence over its environment variable. With none of them set, connections to Forgejo SHALL behave as before.

#### Scenario: Flag wins over environment

- **WHEN** `FORGEJO_TLS_CA=/a.pem` is set and the server is started with `-tls-ca /b.pem`
- **THEN** the server trusts the CA in `/b.pem`

#### Scenario: Environment alone configures the client certificate

- **WHEN** `FORGEJO_TLS_CERT` and `FORGEJO_TLS_KEY` name a valid pair and no TLS flag is passed
- **THEN** the server presents that certificate to Forgejo

#### Scenario: CLI mode honours the environment

- **WHEN** forgejo-mcp runs with `--cli` and `FORGEJO_TLS_CERT` / `FORGEJO_TLS_KEY` name a valid pair
- **THEN** the tool call it executes presents that certificate to Forgejo

#### Scenario: Nothing configured

- **WHEN** no TLS setting is given
- **THEN** connections to Forgejo use the system trust roots and present no client certificate

### Requirement: Every connection to Forgejo uses the TLS configuration

When TLS settings are given, every connection the server opens to the configured Forgejo instance SHALL use them. That covers calls made with a caller's token, with the operator token, through the raw-HTTP helpers, and the startup version probe.

#### Scenario: SDK call to an mTLS-only instance

- **WHEN** Forgejo requires a client certificate signed by a CA, the server is configured with such a certificate, and a caller invokes `get_my_user_info` with their own token
- **THEN** the call succeeds

#### Scenario: Raw-HTTP call to an mTLS-only instance

- **WHEN** Forgejo requires a client certificate, the server is configured with one, and a tool that uses the raw-HTTP helpers is called
- **THEN** the request carries the client certificate and succeeds

#### Scenario: Version probe to an mTLS-only instance

- **WHEN** Forgejo requires a client certificate and the server is configured with one
- **THEN** the startup version probe succeeds

#### Scenario: No client certificate configured against an mTLS-only instance

- **WHEN** Forgejo requires a client certificate and the server is configured with none
- **THEN** the call fails with a TLS error and no request reaches Forgejo's API

### Requirement: A configured CA replaces the system trust roots

When a CA bundle is configured, the server SHALL verify Forgejo's server certificate against the certificates in that bundle only. It SHALL NOT disable server certificate verification in any configuration.

#### Scenario: Private CA trusted

- **WHEN** Forgejo's certificate is signed by a private CA and `-tls-ca` names that CA
- **THEN** connections to Forgejo succeed

#### Scenario: Private CA without the setting

- **WHEN** Forgejo's certificate is signed by a private CA and no CA bundle is configured
- **THEN** connections to Forgejo fail certificate verification

#### Scenario: Server certificate from another CA

- **WHEN** `-tls-ca` names CA A and Forgejo presents a certificate signed by CA B
- **THEN** connections to Forgejo fail certificate verification, even if CA B is in the system trust roots

### Requirement: TLS settings are validated at startup and fail closed

The server SHALL validate the TLS settings before it serves anything or runs a CLI command. It SHALL refuse to start, naming the setting at fault, when a client certificate is given without a key or a key without a certificate, when a file cannot be read, when the certificate and key do not form a valid pair, when the CA file contains no PEM certificate, or when TLS settings are given for a URL that is not `https`. It SHALL never continue without the configured TLS settings.

#### Scenario: Certificate without key

- **WHEN** `-tls-cert` is given and `-tls-key` is not
- **THEN** the server exits at startup with an error naming both settings

#### Scenario: Unreadable or invalid file

- **WHEN** `-tls-key` names a file that does not exist, or a file that is not a PEM key matching the certificate
- **THEN** the server exits at startup with an error naming the setting

#### Scenario: CA file without certificates

- **WHEN** `-tls-ca` names a file with no PEM certificate in it
- **THEN** the server exits at startup with an error naming `-tls-ca`

#### Scenario: TLS settings with an http URL

- **WHEN** a TLS setting is given and the Forgejo URL starts with `http://`
- **THEN** the server exits at startup with an error saying that TLS settings need an `https` URL

### Requirement: An expired client certificate is refused at startup

The server SHALL refuse to start when the configured client certificate has already expired, and SHALL log the certificate's expiry time when it starts with one. The certificate is read once at startup; replacing the file takes effect on the next restart.

#### Scenario: Expired certificate

- **WHEN** `-tls-cert` names a certificate whose validity ended before startup
- **THEN** the server exits at startup with an error naming `-tls-cert` and the expiry time

#### Scenario: Valid certificate

- **WHEN** `-tls-cert` names a certificate valid at startup
- **THEN** the startup log records that a client certificate is configured and when it expires, and never logs the key
