## 1. Resource Framework (slice 1 — `mcp-resources-core`)

- [x] 1.1 Add `server.WithResourceCapabilities(false, false)` to `newMCPServer` in `operation/operation.go`
- [x] 1.2 Create `operation/resource/` package skeleton with `doc.go`
- [x] 1.3 Implement URI parser helpers in `operation/resource/parse.go`: one typed-struct parser per entity (`ParseOwner`, `ParseRepo`, `ParseCommit`, `ParseIssue`, `ParsePR`, `ParseComment`, `ParseStatus`)
- [x] 1.4 Implement template-registration helpers in `operation/resource/register.go` (thin wrapper around `mcp.NewResourceTemplate` + `s.AddResourceTemplate`)
- [x] 1.5 Implement embedded-list bounding helper in `operation/resource/bound.go`: produces sentinel block referencing a named list tool, default cap = 30 exposed as `const EmbeddedListCap = 30`
- [x] 1.6 Implement error-mapping helper in `operation/resource/errors.go`: `MapForgejoError(uri, err) *mcp.JSONRPCError` mapping HTTP `403` → `-32002`, `404` → `-32003`
- [x] 1.7 Add unit tests for `operation/resource/parse.go` covering: happy path each entity, short sha, non-numeric index, unknown kind
- [x] 1.8 Add unit tests for `operation/resource/bound.go` covering: under-cap, at-cap, over-cap with sentinel contents
- [x] 1.9 Add unit tests for `operation/resource/errors.go` covering both code paths
- [x] 1.10 Add `RegisterCoreResources(s *server.MCPServer)` stub function in `operation/operation.go` (no-op for slice 1, gives stable wiring point)
- [ ] 1.11 Manual verification against Claude Code, Claude Desktop, Codex, Cursor: confirm each sends `resources/templates/list` and `resources/read`; record results in design.md "Open Questions" answer

## 2. Commit Resource (slice 2 — `mcp-resource-commit`)

- [x] 2.1 Create `operation/repository/resources_commit.go` (commit lives under repository domain) with `RegisterCommitResource(s *server.MCPServer)`
- [x] 2.2 Register template `forgejo://repo/{owner}/{repo}/commit/{sha}` with description marking immutability
- [x] 2.3 Implement handler returning JSON metadata block plus `text/markdown` body sidecar
- [x] 2.4 Reject shas not exactly 40 hex chars via `operation/resource.ParseCommit`
- [x] 2.5 Unit tests: existing sha (mock client), short sha, missing sha, sidecar present
- [x] 2.6 Wire `RegisterCommitResource` from `operation/operation.go`
- [x] 2.7 README: append commit row to a new "Resources" table

## 3. Commit Status Resource (slice 3 — `mcp-resource-status`)

- [x] 3.1 Create `operation/repository/resources_status.go` with `RegisterStatusResource(s *server.MCPServer)`
- [x] 3.2 Register template `forgejo://repo/{owner}/{repo}/commit/{sha}/status` with cacheability note in description
- [x] 3.3 Implement handler returning aggregate state + bounded `statuses` array via `operation/resource.Bounded(..., "get_commit_statuses")`
- [x] 3.4 Map empty-statuses case to aggregate `state="unknown"`
- [x] 3.5 Unit tests: < cap, > cap with sentinel naming `get_commit_statuses`, empty case, short sha, missing sha
- [x] 3.6 Wire from `operation/operation.go`
- [x] 3.7 README: append status row to Resources table

## 4. Repo + Owner Resources (slice 4)

- [ ] 4.1 Create `operation/repository/resources_repo.go` with `RegisterRepoResource(s *server.MCPServer)`; template `forgejo://repo/{owner}/{repo}`; return counts only, no embedded lists
- [ ] 4.2 Create `operation/user/resources_owner.go` with `RegisterOwnerResource(s *server.MCPServer)`; template `forgejo://owner/{owner}`; resolve user or org
- [ ] 4.3 Unit tests for each: happy path, 403, 404
- [ ] 4.4 Wire both from `operation/operation.go`
- [ ] 4.5 README: append rows for `repo` and `owner` to Resources table

## 5. Issue + Comment Resources (slice 5)

- [ ] 5.1 Create `operation/issue/resources.go` with `RegisterIssueResources(s *server.MCPServer)`
- [ ] 5.2 Register issue template `forgejo://repo/{owner}/{repo}/issue/{index}` with handler returning metadata + markdown sidecar + bounded `recent_comments` (sentinel names `list_issue_comments`)
- [ ] 5.3 Register comment template `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` constraining `kind ∈ {issue, pr}` in parser (reject `wiki` etc. with `-32602`)
- [ ] 5.4 Unit tests: issue happy path, issue with > cap comments, non-numeric index, unknown kind for comment, missing comment
- [ ] 5.5 Wire from `operation/operation.go`
- [ ] 5.6 README: append rows for `issue` and `comment` to Resources table

## 6. PR Resource (slice 6 — `mcp-resource-pr`)

- [ ] 6.1 Create `operation/pull/resources.go` with `RegisterPullResources(s *server.MCPServer)`
- [ ] 6.2 Register template `forgejo://repo/{owner}/{repo}/pr/{index}` with handler returning metadata + head/base refs + mergeability + markdown sidecar + bounded `recent_comments` (sentinel names `list_issue_comments`) + bounded `recent_reviews` (sentinel names `list_pull_reviews`)
- [ ] 6.3 Unit tests: open PR, merged PR, PR with > cap comments, PR with > cap reviews, non-numeric index, missing PR
- [ ] 6.4 Wire from `operation/operation.go`
- [ ] 6.5 README: append row for `pr` to Resources table

## 7. Documentation & Wrap-up

- [ ] 7.1 Add "Resources" section to README explaining the URI scheme, listing all templates in one table, and stating the coexist-with-tools rule
- [ ] 7.2 Add "Resources" subsection to AGENTS.md mirroring the README rule for AI assistants modifying the codebase
- [ ] 7.3 Update CHANGELOG (or release notes file) noting the additive surface
- [ ] 7.4 Verify `make build` and full test suite pass with all seven `RegisterXResources` calls wired
- [ ] 7.5 File a follow-up bead to revisit `WithResourceCapabilities(subscribe=true, …)` once a real subscription use case appears
- [ ] 7.6 File a follow-up bead to revisit the default embedded-list cap (currently `30`) once usage telemetry is available
