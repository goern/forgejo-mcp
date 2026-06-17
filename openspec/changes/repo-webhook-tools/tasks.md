## 1. Package scaffold

- [ ] 1.1 Create `operation/hook/` directory with SPDX-licensed `hook.go` stub (package declaration, `RegisterTools` function skeleton)
- [ ] 1.2 Create `operation/hook/resources_hook.go` stub (package declaration, `RegisterResources` function skeleton)
- [ ] 1.3 Import and call `hook.RegisterTools` and `hook.RegisterResources` in `operation/operation.go`

## 2. URI parser helpers

- [ ] 2.1 Add `ParseHook(uri string)` helper to `operation/resource/` returning `{Owner, Repo, ID int64}` for `forgejo://repo/{owner}/{repo}/hook/{id}` URIs
- [ ] 2.2 Add `ParseHooks(uri string)` helper to `operation/resource/` returning `{Owner, Repo}` for `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` URIs

## 3. MCP tools

- [ ] 3.1 Implement `list_repo_hooks` tool — `page`/`limit` params, bounded at 50, returns hook array without `secret`
- [ ] 3.2 Implement `get_repo_hook` tool — `owner`, `repo`, `id` params, returns single hook without `secret`
- [ ] 3.3 Implement `create_repo_hook` tool — all params from spec, `secret` accepted but not echoed in response
- [ ] 3.4 Implement `edit_repo_hook` tool — optional patch fields, returns updated hook without `secret`
- [ ] 3.5 Implement `delete_repo_hook` tool — returns success or MCP error
- [ ] 3.6 Implement `test_repo_hook` tool — returns `{"triggered": true}` on 204

## 4. Resource templates

- [ ] 4.1 Implement `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` resource handler — uses `resource.Bounded`, cap `EmbeddedListCap`, sentinel `list_repo_hooks`, no `secret`
- [ ] 4.2 Implement `forgejo://repo/{owner}/{repo}/hook/{id}` resource handler — single hook, `MapForgejoError`, no `secret`
- [ ] 4.3 Register both templates in `RegisterResources` with description, MIME type `application/json`

## 5. Documentation

- [ ] 5.1 Add the two new hook URI rows to the AGENTS.md resource table
- [ ] 5.2 Verify `ToolSearch` for `webhook`/`hook` now surfaces all six tools (build and spot-check)

## 6. Quality gates

- [ ] 6.1 Run `make build` — no compile errors
- [ ] 6.2 Run `make vendor` — go.mod/go.sum clean
- [ ] 6.3 Run existing tests — no regressions (`go test ./...`)
- [ ] 6.4 Run `pre-commit run --all-files` — passes
