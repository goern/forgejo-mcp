## 1. Scaffold and Registration

- [x] 1.1 Create `operation/org/` package with `org.go`, `membership.go`, `teams.go` files and a `RegisterTool(s *server.MCPServer)` function
- [x] 1.2 Add `RegisterOrgTool` wrapper in `operation/operation.go` and call it from `RegisterTool()`
- [x] 1.3 Add `{"org", operation.RegisterOrgTool}` entry in `cmd/cli.go` `registerToolsWithDomains()` domain list

## 2. Organization CRUD (`operation/org/org.go`)

- [x] 2.1 Implement `create_org` tool — accepts name (required), full_name, description, website, location, visibility; calls `forgejo.Client().CreateOrg()`
- [x] 2.2 Implement `get_org` tool — accepts org (required); calls `forgejo.Client().GetOrg()`
- [x] 2.3 Implement `list_my_orgs` tool — accepts page, limit; calls `forgejo.Client().ListMyOrgs()`
- [x] 2.4 Implement `list_user_orgs` tool — accepts user (required), page, limit; calls `forgejo.Client().ListUserOrgs()`
- [x] 2.5 Implement `edit_org` tool — accepts org (required), full_name, description, website, location, visibility; calls `forgejo.Client().EditOrg()`
- [x] 2.6 Implement `delete_org` tool — accepts org (required); calls `forgejo.Client().DeleteOrg()`; description includes destructive warning

## 3. Membership (`operation/org/membership.go`)

- [x] 3.1 Implement `list_org_members` tool — accepts org (required), page, limit; calls `forgejo.Client().ListOrgMembership()`
- [x] 3.2 Implement `check_org_membership` tool — accepts org (required), user (required); calls `forgejo.Client().CheckOrgMembership()`
- [x] 3.3 Implement `remove_org_member` tool — accepts org (required), user (required); calls `forgejo.Client().DeleteOrgMembership()`

## 4. Teams (`operation/org/teams.go`)

- [x] 4.1 Implement `list_org_teams` tool — accepts org (required), page, limit; calls `forgejo.Client().ListOrgTeams()`
- [x] 4.2 Implement `create_org_team` tool — accepts org (required), name (required), description, permission, can_create_org_repo, includes_all_repositories; calls `forgejo.Client().CreateTeam()`
- [x] 4.3 Implement `add_team_member` tool — accepts team_id (required), user (required); calls `forgejo.Client().AddTeamMember()`
- [x] 4.4 Implement `remove_team_member` tool — accepts team_id (required), user (required); calls `forgejo.Client().RemoveTeamMember()`
- [x] 4.5 Implement `add_team_repo` tool — accepts team_id (required), org (required), repo (required); calls `forgejo.Client().AddTeamRepository()`
- [x] 4.6 Implement `remove_team_repo` tool — accepts team_id (required), org (required), repo (required); calls `forgejo.Client().RemoveTeamRepository()`

## 5. Testing

- [x] 5.1 Add unit tests for org CRUD tools in `operation/org/org_test.go`
- [x] 5.2 Add unit tests for membership tools in `operation/org/membership_test.go`
- [x] 5.3 Add unit tests for team tools in `operation/org/teams_test.go`

## 6. Verify

- [x] 6.1 Run `make build` — confirm binary compiles
- [x] 6.2 Run `forgejo-mcp --cli list` — confirm all 15 org tools appear under the `org` domain
- [x] 6.3 Run `forgejo-mcp --cli create_org --help` — confirm parameter schema is correct
