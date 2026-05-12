## 1. Wire raw-HTTP fetch for org labels

- [ ] 1.1 Add a small `OrgLabel` (or shared `Label`) struct in `operation/issue/` with fields `id`, `name`, `color`, `description`, `scope`
- [ ] 1.2 Add a helper `fetchOrgLabels(ctx, org, page, limit) ([]Label, error)` that calls `pkg/forgejo.DoJSONList` against `/orgs/{org}/labels?page=&limit=` and stamps `scope: "org"` on each entry
- [ ] 1.3 Confirm 404 → empty behavior is preserved (helper already maps it); add a unit test that asserts no error on 404

## 2. New `list_org_labels` MCP tool

- [ ] 2.1 Add `ListOrgLabelsToolName` constant and `ListOrgLabelsTool` `mcp.NewTool(...)` definition with `org`, `page`, `limit` parameters in `operation/issue/issue.go`
- [ ] 2.2 Implement `ListOrgLabelsFn` handler using the helper from 1.2; return `to.TextResult(labels)`
- [ ] 2.3 Register the tool in `RegisterTool(s *server.MCPServer)`
- [ ] 2.4 Unit test against `httptest.Server` (pattern in `pkg/forgejo/raw_http_test.go`): success path, empty path, 401 → `ErrUnauthorized`

## 3. Augment `list_repo_labels` with org merge

- [ ] 3.1 Add `include_org_labels` boolean parameter (default `true`) to `ListRepoLabelsTool` definition
- [ ] 3.2 Update `ListRepoLabelsFn` to:
  - call existing SDK `ListRepoLabels` and stamp `scope: "repo"` on each result
  - when `include_org_labels` is true, call `fetchOrgLabels(ctx, owner, page, limit)` and append results
  - propagate errors from the org call **except** 404 (helper handles), and **except** when `include_org_labels` is false
- [ ] 3.3 Unit tests for: org-owned repo merged result, user-owned repo (org call 404 → empty), `include_org_labels=false` returns repo-only, 401 from org endpoint surfaces as `ErrUnauthorized`

## 4. Documentation

- [ ] 4.1 Update `docs/PROMPTING.md` if it references label discovery flows
- [ ] 4.2 Update `README.md` tool list with the new `list_org_labels` entry
- [ ] 4.3 Add a one-line CHANGELOG/release-note entry referencing Codeberg #125

## 5. Verification

- [ ] 5.1 `make build` passes
- [ ] 5.2 `go test ./...` passes
- [ ] 5.3 `openspec validate add-org-label-support --strict` passes
- [ ] 5.4 Manual smoke against a Codeberg org-owned repo: `list_org_labels` returns expected labels; `list_repo_labels` includes them with `scope: "org"`
- [ ] 5.5 Comment back on Codeberg issue #125 with the PR link once merged
