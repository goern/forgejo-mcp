## 1. Context-aware client factory

- [x] 1.1 Add `forgejo.WithToken(ctx, token) context.Context` and an unexported `tokenKey` type to `pkg/forgejo/forgejo.go`. Read back via `tokenFromContext(ctx) string`.
- [x] 1.2 Refactor `Client()` to `Client(ctx context.Context) *forgejo.Client`. When `tokenFromContext(ctx)` returns a non-empty string, build a fresh client via `forgejo.NewClient(flag.URL, forgejo.SetToken(token))`. Otherwise return the singleton initialized via `sync.Once` from `flag.Token` (existing behavior).
- [x] 1.3 **Blocker fix** — when ctx-token is supplied and `forgejo.NewClient(...)` returns an error, MUST NOT fall through to the singleton. Either change `Client(ctx)` signature to `(*forgejo.Client, error)` and propagate, or return `nil` and log loudly. Silent demotion to the global token is a privilege-escalation vector. See design D3.

## 2. Transport-layer extraction

- [x] 2.1 In `operation/operation.go`, register `WithHTTPContextFunc` on `server.NewStreamableHTTPServer` and `WithSSEContextFunc` on `server.NewSSEServer`. Each func reads `r.Header.Get("Authorization")` and calls `forgejo.WithToken(ctx, token)` when a recognized scheme is present.
- [x] 2.2 Accept `token <X>` prefix (Forgejo native scheme).
- [x] 2.3 Accept `Bearer <X>` prefix (OAuth2 transport).
- [x] 2.4 **Blocker fix** — match scheme case-insensitively (RFC 7235). `bearer xyz` MUST resolve to the same token as `Bearer xyz`. Today the case-sensitive `strings.HasPrefix("Bearer ")` rejects lowercase, which then falls through to the bare-token branch (1.5), which silently downgrades to the global identity.
- [x] 2.5 **Blocker fix** — drop the no-scheme bare-token branch (`operation/operation.go:152-154` and `:181-183`). Not in the spec. Accepting a bare token without a scheme expands the contract silently. If kept, document in proposal and README.
- [x] 2.6 Stdio path is left without a context func; stdio handlers see no ctx-token, `Client(ctx)` returns the singleton.

## 3. Handler refactor

- [x] 3.1 Change every handler under `operation/**/*.go` to pass its `ctx` to `forgejo.Client(ctx)`. Mechanical; 84 callsites.
- [x] 3.2 Update `pkg/forgejo/rawhttp.go` (`setCommonHeaders`) to read the token from `ctx` first, falling back to `flag.Token`. Attachment upload/download paths must honor per-request identity.
- [x] 3.3 Adjust all attachment handlers in `operation/attachment/` to pass `ctx` into the raw-http path.

## 4. Tests

- [x] 4.1 `pkg/forgejo/forgejo_test.go` exists, exercises `WithToken` → `Client(ctx)` → distinct client returned.
- [x] 4.2 **Blocker fix** — replace the pointer-inequality assertion (`c1 != c2`) with an `httptest.Server` based test. Two parallel goroutines, each calling `Client(ctx)` with a distinct token, each making an outbound SDK call. The test server records inbound `Authorization` headers and asserts each goroutine's request arrives carrying its own token (not the other's, not the global). This is the only test that proves the no-leakage requirement.
- [x] 4.3 Add a test for the D3 failure path: stub `forgejo.NewClient` to return an error, supply a ctx-token, assert `Client(ctx)` returns error / nil (after 1.3 lands) rather than silently returning the singleton.
- [x] 4.4 Add a test for case-insensitive scheme matching (after 2.4 lands): `bearer xyz`, `BEARER xyz`, `TOKEN xyz` all resolve to the same context token.
- [x] 4.5 Existing tests under `test/race/` continue to pass with the singleton path exercised in stdio mode.

## 5. Documentation

- [x] 5.1 Add a "Multi-tenant HTTP mode" section to README with a runnable showboat demo: start the server with no `--token`, two `curl` calls with distinct `Authorization` headers hitting the same `Mcp-Session-Id` show distinct identities via `get_my_user_info`, a third call with a bogus token returns the 401. Showboat ask already posted on PR #138.
- [x] 5.2 Cross-link this openspec change from the README section so future readers can find the rationale.

## 6. Verification

- [x] 6.1 `make build` passes against PR #138's branch.
- [x] 6.2 `go test ./...` passes against PR #138's branch.
- [x] 6.3 `openspec validate stateless-http-auth --strict` passes.
- [ ] 6.4 Manual smoke: start server with `--transport http --url https://codeberg.org` (no `--token`); send two requests with distinct tokens; each `get_my_user_info` returns the correct distinct identity.
- [ ] 6.5 Manual smoke: send a request with `Authorization: bearer xyz` (lowercase scheme) and confirm it resolves to the per-request identity, not the global fallback. (Validates 2.4.)
- [ ] 6.6 Manual smoke: send a request with a deliberately broken Forgejo URL so the SDK's version probe fails; confirm the request returns an error rather than silently authenticating as the global token. (Validates 1.3.)
- [ ] 6.7 After PR #138 merges with blocker fixes, archive this change under `openspec/changes/archive/<date>-stateless-http-auth/`.
