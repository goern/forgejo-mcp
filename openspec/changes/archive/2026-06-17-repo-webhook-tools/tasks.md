## 1. Package scaffold

- [x] 1.1 Create `operation/hook/` directory with SPDX-licensed `hook.go` stub (package declaration, `RegisterTools` function skeleton)
- [x] 1.2 Create `operation/hook/resources_hook.go` stub (package declaration, `RegisterResources` function skeleton)
- [x] 1.3 Import and call `hook.RegisterTools` and `hook.RegisterResources` in `operation/operation.go`

## 2. URI parser helpers

- [x] 2.1 Add `ParseHook(uri string)` helper to `operation/resource/` returning `{Owner, Repo, ID int64}` for `forgejo://repo/{owner}/{repo}/hook/{id}` URIs
- [x] 2.2 Add `ParseHooks(uri string)` helper to `operation/resource/` returning `{Owner, Repo}` for `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` URIs

## 3. MCP tools

- [x] 3.1 Implement `list_repo_hooks` tool — `page`/`limit` params, bounded at 50, returns hook array without `secret`
- [x] 3.2 Implement `get_repo_hook` tool — `owner`, `repo`, `id` params, returns single hook without `secret`
- [x] 3.3 Implement `create_repo_hook` tool — all params from spec, `secret` accepted but not echoed in response
- [x] 3.4 Implement `edit_repo_hook` tool — optional patch fields, returns updated hook without `secret`
- [x] 3.5 Implement `delete_repo_hook` tool — returns success or MCP error
- [x] 3.6 Implement `test_repo_hook` tool — returns `{"triggered": true}` on 204

## 4. Resource templates

- [x] 4.1 Implement `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` resource handler — uses `resource.Bounded`, cap `EmbeddedListCap`, sentinel `list_repo_hooks`, no `secret`
- [x] 4.2 Implement `forgejo://repo/{owner}/{repo}/hook/{id}` resource handler — single hook, `MapForgejoError`, no `secret`
- [x] 4.3 Register both templates in `RegisterResources` with description, MIME type `application/json`

## 5. Documentation

- [x] 5.1 Add the two new hook URI rows to the AGENTS.md resource table
- [x] 5.2 Verify `ToolSearch` for `webhook`/`hook` now surfaces all six tools (build and spot-check)

## 6. Quality gates

- [x] 6.1 Run `make build` — no compile errors
- [x] 6.2 Run `make vendor` — go.mod/go.sum clean
- [x] 6.3 Run existing tests — no regressions (`go test ./...`)
- [x] 6.4 Run `pre-commit run --all-files` — passes
