## Why

Forgejo MCP currently has no organization management capabilities. Users working with Forgejo instances need to manage organizations — creating them, listing memberships, managing teams — but must leave the MCP/CLI workflow to do so. The Forgejo Go SDK already provides 38+ organization-related methods, making this a well-supported addition. Requested in issue #92.

## What Changes

- Add a new `operation/org/` domain with MCP tool handlers for organization CRUD, membership, and team operations
- Register the new domain in `operation/operation.go` and `cmd/cli.go` (domain mapping)
- All new tools automatically available via both MCP (stdio/SSE) and CLI (`--cli`) — no separate CLI wiring needed

### New MCP Tools

**Organization CRUD:**
- `create_org` — Create a new organization
- `get_org` — Get organization details by name
- `list_my_orgs` — List organizations the authenticated user belongs to
- `list_user_orgs` — List organizations for a given user
- `edit_org` — Update organization settings (description, visibility, etc.)
- `delete_org` — Delete an organization

**Membership:**
- `list_org_members` — List members of an organization
- `check_org_membership` — Check if a user is a member
- `remove_org_member` — Remove a member from an organization

**Teams:**
- `list_org_teams` — List teams in an organization
- `create_org_team` — Create a team in an organization
- `add_team_member` — Add a user to a team
- `remove_team_member` — Remove a user from a team
- `add_team_repo` — Add a repository to a team
- `remove_team_repo` — Remove a repository from a team

## Capabilities

### New Capabilities
- `org-crud`: Organization create, read, update, delete, and listing operations
- `org-membership`: Organization membership queries and management
- `org-teams`: Team creation, membership, and repository assignment within organizations

### Modified Capabilities
_(none — this is purely additive)_

## Impact

- **Code**: New `operation/org/` package (3-4 files). One-line additions to `operation/operation.go` and `cmd/cli.go` domain registry.
- **API surface**: 15 new MCP tools in the `org` domain. No existing tools change.
- **Dependencies**: No new dependencies — uses existing `forgejo-sdk/forgejo/v2` already vendored.
- **Breaking changes**: None. Purely additive.
- **Testing**: New unit tests in `operation/org/` following existing patterns.
