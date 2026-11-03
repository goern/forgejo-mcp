<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

The planning artifacts and the implementation land together in one pull request, unarchived. Specs and design are the reference: `specs/stateless-http-auth/spec.md`, `specs/forgejo-jwt-issuer/spec.md` and `design.md` (D1–D5). CI parity means `golangci-lint run ./...` and `go test -race -shuffle=on -count=1 ./...`, not plain `go test`.

## 1. Base

- [ ] 1.1 Bring in #683. If it has merged, rebase on `main`. Otherwise cherry-pick its commit (`a84093c`, author kept) with the two CI fixes from its review thread (`errors.Is` in `redact_test.go`, a non-hex test token). Verify that `golangci-lint` and gitleaks are clean on the branch and that `pkg/forgejo` tests pass with `-race -shuffle=on`.

## 2. Rejection at the transport (D1)

- [ ] 2.1 Add the rejection-observing `http.RoundTripper` to `pkg/forgejo`. On `401` it replaces the body with `{"message":"Forgejo rejected the access token"}`, closes the original after a bounded read, and sets the request context's marker if one is present. Verify with unit tests against an `httptest` Forgejo that echoes the token: the body seen by the caller is the fixed one, a `403` and a `404` pass through untouched, and the marker is set only on `401`.
- [ ] 2.2 Install it on the ephemeral SDK client (`SetHTTPClient`, plus `SetContext(ctx)` so SDK requests carry the request context), on the singleton SDK client, and on `rawHTTPClient`. Verify with tests for the scenarios "Rejection on an SDK call after the client was built" (`SetServerVersion` set, rejection at the first API call), "Rejection while the per-request client is built" (version unknown, rejection at the probe) and "Rejection on a raw-HTTP helper call" (`errors.Is(err, ErrUnauthorized)` still holds). Each test asserts that neither the returned error nor the captured log output contains the token.
- [ ] 2.3 Cover "The operator token is rejected at startup": run the startup connection test against an `httptest` Forgejo that answers `401` with the token echoed, and assert that the returned error and the captured log contain no token. Verify by that test.

## 3. Marker and handler mapping (D2, D3, D5)

- [ ] 3.1 Add the per-request marker type to `pkg/forgejo` (atomic flag plus auth mode), with accessors to put it into and read it from a context. `requestTokenContextFunc` copies it from the incoming request's context, and only when it injects a per-request credential (`passthrough` token or minted JWT). Verify with tests in `operation/` for both modes, and for the fallback path where no credential is injected and no marker is attached.
- [ ] 3.2 Add a tool handler middleware and a resource handler middleware to `newMCPServer`. For a marked call they replace the handler's outcome: `server.NewHandlerError` with `-32001` and the fixed message in `passthrough`; the fixed server-side message with `-32603` plus a warning-level log line naming the tool in `resource-server`. Verify with tests that drive `tools/call` and `resources/read` through the real server for the scenarios "A revoked token fails with -32001", "A resource read with a revoked token fails with -32001", "Insufficient scope is not a rejection" (a `403` keeps its code), and a handler that swallows the error still fails.
- [ ] 3.3 Cover the `forgejo-jwt-issuer` scenarios "The integration was removed", "The operator learns of the rejection" and "The echoed JWT never reaches the caller" with an end-to-end test in `operation/resource_server_http_test.go`: a valid inbound token, and an `httptest` Forgejo that answers the minted JWT with `401` and echoes it. Assert no `401` and no `WWW-Authenticate` on the response, code `-32603` with the server-side message, a warning log line without the JWT, and no JWT anywhere in the response.

## 4. HTTP status on `http` (D4)

- [ ] 4.1 Add the HTTP middleware that attaches a fresh marker per request and wraps the `ResponseWriter`. On `WriteHeader(200)` with the marker set in `passthrough`, it writes `401` with `WWW-Authenticate: Bearer error="invalid_token"`; the wrapper keeps `http.Flusher` and exposes `Unwrap`. Mount it in front of the streamable-HTTP handler, and attach the marker for the `sse` transport's posted messages. Verify with tests for "A revoked token yields 401 with a challenge" (status, header and `-32001` body), "A request that never reaches Forgejo is not challenged" (`tools/list` stays `200`), "A response already being streamed keeps its status" (a tool that emits a progress notification before the rejection: status `200`, `-32001` in the stream), and "The SSE transport reports the code".
- [ ] 4.2 Verify that streaming still works through the wrapper: a `subscriptions/listen` request and the heartbeat test pass unchanged, with `-race`.
- [ ] 4.3 Cover "stdio is unchanged": a stdio tool call with a rejected operator token fails with the fixed message and no `-32001`. Verify by that test.

## 5. Documentation and end-to-end check

- [ ] 5.1 Document the behaviour for operators and client authors in the README: a rejected token in `passthrough` on `http` yields `401` plus `-32001`; `sse` yields `-32001`; `resource-server` mode reports a server-side failure instead. Verify that the README states the code, the header, and the mode difference, and that `scripts/ci/check-readme-tool-coverage.sh` still passes.
- [ ] 5.2 Live check against `git.b4mad.industries` with a made-up token, no secret needed: run the built binary with `--transport http` and no operator token, send `tools/call get_my_user_info` with `Authorization: token not-a-real-token`, and record status, headers and body. Verify that the response is `401` with the challenge and `-32001`, and that neither the response nor the server log contains the made-up token. Paste the transcript into the PR description.
- [ ] 5.3 Run the full CI set on the branch: `go build ./...`, `go vet ./...`, `gofmt -l`, `golangci-lint run ./...`, `go test -race -shuffle=on -count=1 ./...`, `go mod tidy` without drift, `scripts/ci/check-api-path-escaping.sh`, `scripts/ci/check-readme-tool-coverage.sh`, `scripts/ci/check-spec-demo-anchors.sh`, gitleaks, and `openspec validate forge-credential-rejection --strict`. Verify that every one passes.

## Workflow follow-up

- Open the PR from `byteflavour/feat/forge-credential-rejection`, referencing #701 and #683, with the live transcript from 5.2.
- After merge, archive in a separate PR: `openspec archive forge-credential-rejection`, remove the `demos-anchored` marker from the archived delta specs if present, and add anchored proofs for the new `forgejo-jwt-issuer` scenarios to `openspec/specs/forgejo-jwt-issuer/forgejo-jwt-issuer.demo.md`, so `check-spec-demo-anchors.sh` stays green.
- Close #701 with a pointer to the merged PR.
