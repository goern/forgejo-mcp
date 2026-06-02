## 1. Resource URI parsing

- [ ] 1.1 Add `ParseBranchProtections(uri)` (collection: `forgejo://repo/{owner}/{repo}/branch_protections`) and `ParseBranchProtection(uri)` (single: `.../branch_protection/{rule}`) to `operation/resource/parse.go`, returning owner/repo (+rule), validating the rule segment is non-empty.
- [ ] 1.2 Add parser unit tests in `operation/resource/parse_test.go` (happy path both shapes; malformed → ErrInvalidParams).

## 2. CRUD tools

- [ ] 2.1 Create `operation/branchprotection/branchprotection.go`: tool name consts + `mcp.NewTool` declarations for `list_branch_protections`, `get_branch_protection`, `create_branch_protection`, `edit_branch_protection`, `delete_branch_protection`, using `params.Owner/Repo/Page/Limit` and the focused field subset from design.md D3; declare `status_check_contexts` as an array param.
- [ ] 2.2 Implement `ListBranchProtectionsFn` with `page`/`limit` bounds (ListOptions) and a resumable response echoing the page.
- [ ] 2.3 Implement `GetBranchProtectionFn` (owner, repo, rule).
- [ ] 2.4 Implement `CreateBranchProtectionFn`: require `branch_name`; optional `rule_name`; coerce `status_check_contexts` `[]any`→`[]string`; build `CreateBranchProtectionOption`.
- [ ] 2.5 Implement `EditBranchProtectionFn` with pointer PATCH semantics (`pkg/ptr.To`): set a field only when the caller passed it; send `status_check_contexts` only when provided.
- [ ] 2.6 Implement `DeleteBranchProtectionFn`: call SDK delete, return a success confirmation payload.
- [ ] 2.7 Add `RegisterTool(s *server.MCPServer)` registering all five tools.

## 3. Resource-templates

- [ ] 3.1 Create `operation/branchprotection/resources_branchprotection.go`: register the collection template (`…/branch_protections`) and single template (`…/branch_protection/{rule}`) via `resource.RegisterTemplate`.
- [ ] 3.2 Collection handler: request `EmbeddedListCap + 1`, cap with `resource.Bounded(..., "list_branch_protections")`, emit truncation sentinel + `list_tool` when over cap; map errors with `resource.MapForgejoError`.
- [ ] 3.3 Single handler: `GetBranchProtection`, marshal protection state; map errors with `resource.MapForgejoError`.
- [ ] 3.4 Add `RegisterResource(s *server.MCPServer)` and call it alongside tool registration.

## 4. Wiring

- [ ] 4.1 In `operation/operation.go` add `RegisterBranchProtectionTool(s)` (+ resource registration) and call it from `RegisterTool`.
- [ ] 4.2 Add any new shared param descriptions to `operation/params/`.

## 5. Tests

- [ ] 5.1 `operation/branchprotection/branchprotection_test.go`: httptest + `forgejo.SetClientForTesting`; cover list (bounded), get (ok + 404), create (status_check_contexts round-trip; missing branch_name → error), edit (only-passed-fields in body; contexts round-trip), delete (ok).
- [ ] 5.2 `operation/branchprotection/resources_branchprotection_test.go`: collection (happy path + truncation sentinel at >EmbeddedListCap + 404), single (happy path + malformed URI → invalid-params).

## 6. Docs & validation

- [ ] 6.1 Add the five tools to the README tool table with their bound params (output-bounding documentation contract); fill the output-bounding checklist in the PR description.
- [ ] 6.2 `gofmt`/`go vet`, `make build`, `go test ./...` green.
- [ ] 6.3 `openspec validate branch-protection-management --strict` passes.
