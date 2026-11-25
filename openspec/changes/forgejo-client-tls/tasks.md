# Tasks

## 1. TLS transport in pkg/forgejo

- [x] 1.1 Add a test PKI helper (CA, server cert for 127.0.0.1, client cert, expired client cert, all generated in `t.TempDir()`) and verify that no PEM file is added to the repository (`git status` shows only `.go` files)
- [x] 1.2 Implement `ConfigureTLS(cert, key, ca)` in `pkg/forgejo/tls.go`: pairing check, file loading, expiry check, CA pool replacing the system roots, cloned default transport set as the `rejectionTransport` base of `sdkHTTPClient` and `rawHTTPClient`. Verify with unit tests for each startup-validation scenario in the spec (cert without key, key without cert, missing file, mismatched pair, CA without certificates, expired certificate) and for "nothing configured leaves the transports unchanged"
- [x] 1.3 Add `httptest` mTLS server tests covering an SDK call with a caller token (`Client(ctx)` then `GetMyUserInfo`), a raw-HTTP call (`DoJSON`), `ProbeServerVersion`, the TLS failure without a client certificate (handler never reached), the private-CA failure without `-tls-ca`, and the "server cert from another CA" failure. Verify with `go test -race ./pkg/forgejo/`
- [x] 1.4 Verify that the rejection handling still works through the TLS transport: a 401 from the mTLS server keeps its body replaced (existing rejection test pattern, run over TLS)

## 2. Settings and startup in cmd

- [x] 2.1 Add `TLSCert`, `TLSKey`, `TLSCA` to `pkg/flag`, and the three flags plus `FORGEJO_TLS_*` resolution via `resolveString` in `cmd/cmd.go`. Verify with `cmd` tests: flag wins over env, env alone applies, CLI mode (no parsed flags) reads env
- [x] 2.2 Call `forgejo.ConfigureTLS` from `initConfig`, refuse `http://` URLs with TLS settings, `log.Fatal` naming the setting on error, and log `client_cert_configured` / `not_after` / `ca_configured` at info with no key material. Verify with a test of the URL check and a manual run of `forgejo-mcp -url http://x -tls-ca /dev/null` that exits with the error
- [x] 2.3 Document `-tls-cert` / `-tls-key` / `-tls-ca` and `FORGEJO_TLS_*` in README "Configuration Options", with a short mTLS subsection (CA replaces system roots, a restart picks up a renewed certificate, no skip-verify). Verify that the README examples match the flag names in `cmd/cmd.go`

## 3. Integration checks

- [x] 3.1 Run the CI command set locally (`go build ./...`, `go vet ./...`, `gofmt -l`, `golangci-lint run ./...`, `go test -race -shuffle=on -count=1 ./...`, `go mod tidy` with no drift, `scripts/ci/check-api-path-escaping.sh`, `scripts/ci/check-readme-tool-coverage.sh`) and verify all pass
- [x] 3.2 Run `openspec validate forgejo-client-tls --strict` and verify it is valid

## Workflow follow-up

- Open the PR, referencing #232 (to be closed once this lands), with Paul Berz as `Co-authored-by` on the implementation commit.
- Archive the change after merge, in a follow-up PR.
- File a follow-up issue for certificate rotation and inline PEM if someone asks for them.

## 4. Review follow-ups (#755, #756)

- [x] 4.1 Refuse a client certificate whose `NotBefore` is after startup, with an error naming `-tls-cert` and the time it becomes valid (#756). Verify with a `TestTLSStartupValidation` case.
- [x] 4.2 README, "Connecting to Forgejo behind mutual TLS": state that the TLS settings do not cover the identity-provider requests in `resource-server` mode (#755).
