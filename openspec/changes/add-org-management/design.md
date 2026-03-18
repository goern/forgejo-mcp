## Context

Forgejo MCP currently supports six operational domains: user, repo, issue, pull, search, actions, and version. Each domain follows the same pattern:

1. A package under `operation/<domain>/` defines tools via `mcp.NewTool()` and handler functions
2. A `RegisterTool(s *server.MCPServer)` function wires them to the MCP server
3. `operation/operation.go` calls each domain's registration
4. `cmd/cli.go` mirrors the same registration with domain tagging for `--cli list` grouping

The Forgejo Go SDK (`codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v2`) provides full organization API coverage. The singleton client in `pkg/forgejo/` is already used by all domains.

Shared parameter descriptions live in `operation/params/params.go` — the `Org` and `User` constants already exist.

## Goals / Non-Goals

**Goals:**
- Add 15 MCP tools for organization management (CRUD, membership, teams)
- Follow existing domain patterns exactly — no new abstractions
- Available via both MCP and CLI with zero extra CLI wiring (CLI dispatches to MCP handlers)
- Consistent parameter naming and response formatting with existing tools

**Non-Goals:**
- Admin-only endpoints (`AdminCreateOrg`, `AdminListOrgs`) — these require admin tokens and are a different use case
- Organization webhooks — complex configuration, separate feature
- Organization Actions secrets/variables — already partially covered by the actions domain
- Organization avatar management — low priority, not in SDK as structured API
- Organization labels — blocked on SDK support

## Decisions

### 1. Single `operation/org/` package with file-per-concern

Three files: `org.go` (CRUD), `membership.go` (membership), `teams.go` (teams). One `RegisterTool()` entry point.

**Why over a flat single file**: 15 tools × ~50 lines each = ~750 lines. Splitting by concern matches `operation/repo/` which splits into `repo.go`, `file.go`, `branch.go`, `commit.go`.

### 2. Tool naming: `{verb}_{noun}` with `org` prefix

Names: `create_org`, `get_org`, `list_my_orgs`, `list_user_orgs`, `edit_org`, `delete_org`, `list_org_members`, `check_org_membership`, `remove_org_member`, `list_org_teams`, `create_org_team`, `add_team_member`, `remove_team_member`, `add_team_repo`, `remove_team_repo`.

**Why**: Matches existing conventions (`create_repo`, `fork_repo`, `list_my_repos`). The `org` prefix disambiguates from user-level operations.

### 3. Team operations use team ID (int64) for mutations, org name for listing

`list_org_teams` takes `org` (string). `add_team_member`, `remove_team_member`, `add_team_repo`, `remove_team_repo` take `team_id` (number) — this matches the SDK which uses `int64` team IDs for these operations.

**Why over name-based lookup**: The SDK methods (`AddTeamMember(id, user)`, `AddTeamRepository(id, org, repo)`) use numeric IDs directly. Adding a name→ID lookup would require an extra API call and introduce ambiguity (team names aren't unique across orgs).

### 4. Reuse `pkg/to` for response formatting

All handlers return `to.TextResult(obj)` for success and `to.ErrorResult(err)` for errors, matching every existing tool.

### 5. Pagination on list endpoints

`list_my_orgs`, `list_user_orgs`, `list_org_members`, `list_org_teams` accept `page` and `limit` parameters using the existing `params.Page` / `params.Limit` descriptions and the same defaults (page=1, limit=100).

### 6. Visibility as string enum

`create_org` and `edit_org` accept `visibility` as a string: `"public"`, `"limited"`, or `"private"`. The SDK's `VisibleType` is a string alias, so no conversion is needed.

## Risks / Trade-offs

**[Risk] Delete org is destructive** → Tool description will include a clear warning. No confirmation mechanism exists in MCP protocol, so the tool description must convey severity. This matches the existing `delete_branch` and `delete_file` patterns.

**[Risk] Team ID discovery requires two steps** → Users must call `list_org_teams` to find a team ID before using team mutation tools. This is inherent to the SDK design and documented in tool descriptions. Alternative (name-based lookup) was rejected per Decision 3.

**[Trade-off] No admin endpoints** → Users with admin tokens cannot create orgs on behalf of other users. This keeps the scope focused and avoids requiring admin-level permissions for the basic use case. Can be added later as a separate feature.

**[Trade-off] No webhook/actions/label management** → Keeps the initial scope manageable (15 tools). These can be added incrementally in future changes.
