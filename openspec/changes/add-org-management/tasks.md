## 1. Scaffold and Registration

- [ ] 1.1 Create `operation/org/` package with `org.go`, `membership.go`, `teams.go` files and a `RegisterTool(s *server.MCPServer)` function
- [ ] 1.2 Add `RegisterOrgTool` wrapper in `operation/operation.go` and call it from `RegisterTool()`
- [ ] 1.3 Add `{"org", operation.RegisterOrgTool}` entry in `cmd/cli.go` `registerToolsWithDomains()` domain list

## 2. Organization CRUD (`operation/org/org.go`)

- [ ] 2.1 Implement `create_org` tool — accepts name (required), full_name, description, website, location, visibility; calls `forgejo.Client().CreateOrg()`
- [ ] 2.2 Implement `get_org` tool — accepts org (required); calls `forgejo.Client().GetOrg()`
- [ ] 2.3 Implement `list_my_orgs` tool — accepts page, limit; calls `forgejo.Client().ListMyOrgs()`
- [ ] 2.4 Implement `list_user_orgs` tool — accepts user (required), page, limit; calls `forgejo.Client().ListUserOrgs()`
- [ ] 2.5 Implement `edit_org` tool — accepts org (required), full_name, description, website, location, visibility; calls `forgejo.Client().EditOrg()`
- [ ] 2.6 Implement `delete_org` tool — accepts org (required); calls `forgejo.Client().DeleteOrg()`; description includes destructive warning

## 3. Membership (`operation/org/membership.go`)

- [ ] 3.1 Implement `list_org_members` tool — accepts org (required), page, limit; calls `forgejo.Client().ListOrgMembership()`
- [ ] 3.2 Implement `check_org_membership` tool — accepts org (required), user (required); calls `forgejo.Client().CheckOrgMembership()`
- [ ] 3.3 Implement `remove_org_member` tool — accepts org (required), user (required); calls `forgejo.Client().DeleteOrgMembership()`

## 4. Teams (`operation/org/teams.go`)

- [ ] 4.1 Implement `list_org_teams` tool — accepts org (required), page, limit; calls `forgejo.Client().ListOrgTeams()`
- [ ] 4.2 Implement `create_org_team` tool — accepts org (required), name (required), description, permission, can_create_org_repo, includes_all_repositories; calls `forgejo.Client().CreateTeam()`
- [ ] 4.3 Implement `add_team_member` tool — accepts team_id (required), user (required); calls `forgejo.Client().AddTeamMember()`
- [ ] 4.4 Implement `remove_team_member` tool — accepts team_id (required), user (required); calls `forgejo.Client().RemoveTeamMember()`
- [ ] 4.5 Implement `add_team_repo` tool — accepts team_id (required), org (required), repo (required); calls `forgejo.Client().AddTeamRepository()`
- [ ] 4.6 Implement `remove_team_repo` tool — accepts team_id (required), org (required), repo (required); calls `forgejo.Client().RemoveTeamRepository()`

## 5. Testing

- [ ] 5.1 Add unit tests for org CRUD tools in `operation/org/org_test.go`
- [ ] 5.2 Add unit tests for membership tools in `operation/org/membership_test.go`
- [ ] 5.3 Add unit tests for team tools in `operation/org/teams_test.go`

## 6. Verify

- [ ] 6.1 Run `make build` — confirm binary compiles
- [ ] 6.2 Run `forgejo-mcp --cli list` — confirm all 15 org tools appear under the `org` domain
- [ ] 6.3 Run `forgejo-mcp --cli create_org --help` — confirm parameter schema is correct
