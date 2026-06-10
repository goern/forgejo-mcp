## 1. Shared helpers (`label-crud`)

- [x] 1.1 Add a `color` normalisation helper: accept `rrggbb` / `#rrggbb` (6-digit only), lowercase, prepend `#`, reject invalid hex with `-32602` (shared by repo + org tools). No 3-digit shorthand (upstream unverified — see Open Questions)
- [x] 1.2 Add an in-use-count helper: repo via SDK `ListRepoIssues` filtered by label name (`state=all`, `type=all`), read total from the returned `*Response` pagination (NOT `X-Total-Count` — `DoJSON` discards headers); org by iterating the token-visible org repos and summing — best-effort, MAY under-count, refusal message discloses the visibility limit

## 2. Repo-label CRUD tools (`label-crud`)

- [x] 2.1 Implement `create_repo_label(owner, repo, name, color, description?)` via SDK `CreateLabel`; return created label incl. numeric `id`
- [x] 2.2 Implement `edit_repo_label(owner, repo, id, name?, color?, description?)` via SDK `EditLabel`; send only supplied fields; reject empty edit with `-32602`
- [x] 2.3 Implement `delete_repo_label(owner, repo, id, delete_mode?)` via SDK `DeleteLabel`; refuse if in-use count `> 0` and `delete_mode != "force"` (report count); surface upstream `404`
- [x] 2.4 Implement `get_repo_label(owner, repo, id)` via SDK `GetRepoLabel`; return single label
- [x] 2.5 Register the four repo tools from `operation/operation.go` (issue domain by default per design D5)
- [x] 2.6 Unit tests: create happy path + missing-hash normalisation, invalid color reject, edit partial-field PATCH, edit empty reject, delete unused/success, delete in-use refused-with-count, delete in-use `delete_mode="force"` success, delete + 404, get-one + 404

## 3. Org-label CRUD tools (`label-crud`, raw-HTTP `DoJSON`)

- [x] 3.1 Add org-label `DoJSON` calls in `pkg/forgejo` against `/orgs/{org}/labels[/{id}]`, mirroring `fetchOrgLabels`
- [x] 3.2 Implement `create_org_label(org, name, color, description?)` and `get_org_label(org, id)`
- [x] 3.3 Implement `edit_org_label(org, id, name?, color?, description?)` (PATCH, only supplied fields) and `delete_org_label(org, id, delete_mode?)` (in-use guard via §1.2 org path)
- [x] 3.4 Register the four org tools from `operation/operation.go`
- [x] 3.5 Unit tests: create + color normalisation, get-one, edit partial PATCH, delete in-use refused-with-count, delete `delete_mode="force"` success, 404

## 4. Label resource-templates (`mcp-resource-label`)

- [x] 4.1 Add `ParseLabel` (repo single) and the org-labels-list parser to `operation/resource/parse.go`: reject non-numeric id with `-32602`
- [x] 4.2 Create `resources*.go` with `RegisterLabelResources(s *server.MCPServer)`
- [x] 4.3 Register `forgejo://repo/{owner}/{repo}/label/{id}` → single `application/json` label block
- [x] 4.4 Register `forgejo://repo/{owner}/{repo}/labels{?page,limit}` → client-controlled `page`/`limit` bound, `operation/resource.Bounded` + `EmbeddedListCap` as ceiling; sentinel names `list_repo_labels` + next `page`
- [x] 4.5 Register `forgejo://org/{org}/labels{?page,limit}` → same page/limit bound; sentinel names `list_org_labels` + next `page`
- [x] 4.6 Reject scope-less `forgejo://label/{id}` URIs (repo/org ids share an int space) in the parser
- [x] 4.7 Map `403`→`-32002`, `404`→`-32003` via `MapForgejoError` for all reads
- [x] 4.8 Wire `RegisterLabelResources` from `operation/operation.go`
- [x] 4.9 Unit tests: single label happy path, non-numeric id, scope-less URI rejected, 404, 403; repo list under/over-cap + `limit` honoured (sentinel `list_repo_labels` + next page), repo list 404; org list over-cap (sentinel `list_org_labels`), org list 404
- [x] 4.10 Confirm all three list/single resources satisfy the `mcp-resources-core` "Collection resource" requirement (cap = `EmbeddedListCap`, shared sentinel, `list_repo_labels` / `list_org_labels` tools untouched)
- [x] 4.11 README "Resources" section + `AGENTS.md` resource table: add all three label templates

## 5. Wrap-up

- [x] 5.1 `make build` + `make vendor` clean; all tests pass
- [x] 5.2 README tool table + `AGENTS.md`: add the eight label tools so `ToolSearch` for `label` surfaces them
- [x] 5.3 Tick Codeberg #190 acceptance-criteria checkboxes (tools + resources); note `exclusive`/`is_archived` still out of scope on both transports
