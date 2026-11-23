<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

## 1. Version probe in pkg/forgejo

- [x] 1.1 Add `ErrSignInRequired` and the swagger fallback to `ProbeServerVersion`. The fallback runs only on `401`/`403` from `/version`, requests `<instance root>/swagger.v1.json` with no credential, and reads it as a stream bounded at 4 MiB that stops after `info`. Verify with `httptest` unit tests for every scenario of the requirement "The version is probed anonymously, with a swagger fallback":
  - `/version` `200` (swagger never requested);
  - `403` then swagger `200`;
  - `401`/`403` then swagger `303` to HTML, `404`, non-JSON, or empty `info.version` (`ErrSignInRequired`);
  - `/version` `404` or `500` and a connection refused (no swagger request, not `ErrSignInRequired`);
  - no `Authorization` header on either request even with `flag.Token` set;
  - an oversized body without `info` (`ErrSignInRequired`, bounded read);
  - a `ROOT_URL` sub-path (`https://host/forgejo` → `/forgejo/swagger.v1.json`).
- [x] 1.2 Verify with `go test -race ./pkg/forgejo/` that the existing version and rejection tests still pass, and that the swagger request goes through `rawHTTPClient`: one test with the mTLS test server from #725, if that has merged, otherwise with a redirect-policy test. (#725 merged: `TestProbeFallsBackToSwaggerOverMutualTLS`.)

## 2. The --forgejo-version setting

- [x] 2.1 Add `ForgejoVersion` to `pkg/flag`, the `--forgejo-version` flag, and `FORGEJO_MCP_FORGEJO_VERSION` resolution via `resolveString` in `cmd/cmd.go`. Validate it in `initConfig` with `forgejo.MajorVersion` (fatal, naming the setting). Do not add it to `ResourceServerSettings`. Verify with `cmd` tests:
  - the flag wins over env;
  - env alone applies;
  - `--cli` mode reads env;
  - an unparsable value is refused;
  - passthrough mode accepts the setting.

## 3. Startup paths in operation

- [x] 3.1 In `Run` (passthrough), branch as in design D3: setting given → `SetServerVersion`; token configured or `stdio` → `testConnection` unchanged; `http`/`sse` without a token → `ProbeServerVersion`, where `ErrSignInRequired` logs the D5 warning and continues and any other error is fatal. Verify with tests that drive the startup decision against `httptest` Forgejo stubs (403 + swagger, 403 without swagger, 404, unreachable, token configured, stdio).
- [x] 3.2 In `prepareResourceServer`, use `--forgejo-version` instead of the probe when it is set. On `ErrSignInRequired`, refuse with the D5 message naming `--forgejo-version`. Keep the floor check for both sources. Verify with `operation/authmode_test.go` cases for the five resource-server version scenarios in the delta spec.
- [x] 3.3 Live check against throwaway Forgejo 15 and 16 containers with `REQUIRE_SIGNIN_VIEW=true`, once with swagger on and once with `ENABLE_SWAGGER=false`. Verify:
  - tokenless `http` starts in both cases, and a request without `Authorization` gets `401`;
  - `resource-server` mode on 16 gets past the version check with swagger on;
  - it refuses with the `--forgejo-version` message with swagger off;
  - it passes with `--forgejo-version 16.0.7`.

## 4. Docs and demo

- [x] 4.1 README: add a "Configuration Options" row for `--forgejo-version` / `FORGEJO_VERSION`, and a sentence in the multi-tenant HTTP section saying that a sign-in-only instance is fine. Operator guide: update "Requirements" (how the version is read and the fallback), and the refusal list. Verify that the flag and env names match `cmd/cmd.go` (`grep`).
- [x] 4.2 `openspec/specs/oauth-resource-server/oauth-resource-server.demo.md`: add anchored proofs for the four new scenarios, using the binary or the tests, following the file's existing `proof` helper. Verify with `scripts/ci/check-spec-demo-anchors.sh` after a local spec sync in a scratch copy.

## 5. Integration checks

- [x] 5.1 Run the CI command set locally and verify that all pass:
  - `go build ./...`, `go vet ./...`, `gofmt -l`;
  - `golangci-lint run ./...`;
  - `go test -race -shuffle=on -count=1 ./...`;
  - `go mod tidy` with no drift;
  - `scripts/ci/check-api-path-escaping.sh`, `scripts/ci/check-readme-tool-coverage.sh`.
- [x] 5.2 Run `openspec validate signin-required-forgejo --strict` and verify that it is valid.

## 6. Review follow-ups (PR #730)

- [x] 6.1 Validate `--forgejo-version` with the SDK's parser (`forgejo.ValidateVersion`), so `16.banana`, `+16` or `16.0.7 rc` refuse startup instead of failing every client build. Verify with `cmd` and `pkg/forgejo` tests.
- [x] 6.2 Record the setting in `initConfig`, so `--cli` gets it too. Verify with a `cmd` test in `--cli` mode.
- [x] 6.3 Rename the environment variable to `FORGEJO_MCP_FORGEJO_VERSION`. Verify that the bare `FORGEJO_VERSION` is ignored.
- [x] 6.4 With the setting given, keep one anonymous reachability request (`CheckReachable`) in tokenless passthrough and in `resource-server` mode, and warn when Forgejo reports a different version, also after `testConnection`. Verify with startup tests for unreachable, `404`, mismatch and build-metadata-only difference.
- [x] 6.5 Count a `401`/`403` as sign-in only when it carries a Forgejo API error. Verify with probe tests for HTML, plain-text, empty and array bodies.
- [x] 6.6 Log the stated version and its source in `resource-server` mode. Verify with `TestPrepareResourceServerUsesTheVersionSetting`.
- [x] 6.7 Drain the `/version` body before the fallback, bound the probe with a 15 s deadline, keep user information out of probe errors, and distrust a swagger document reached off the instance. Verify with probe tests.
- [x] 6.8 Fix the refusal message, the `anonymousGet` comment, the README sentence on startup refusals, the dead `written` counter, and the missing SPDX headers.

## Workflow follow-up

- Open the PR with "Fixes #729", and comment on #729 that the reporter's "rejected the access token" message implies a token was configured (see proposal Non-goals).
- Archive the change after merge, in a follow-up PR. The archive must carry the demo anchors for the modified `oauth-resource-server` scenarios.
