# Demo: Organization Management Tools

*2026-03-20T08:20:00Z by Showboat 0.6.1*
<!-- showboat-id: f3a8d91c-org-management-demo-2026 -->

## What these tools do

15 new MCP tools in the `org` domain give AI agents (and CLI users) full control over Forgejo organizations without leaving the MCP/CLI workflow:

- **CRUD**: create, get, list, edit, delete organizations
- **Membership**: list members, check membership, remove members
- **Teams**: list teams, create teams, add/remove team members, add/remove team repos

This enables autonomous workflows like: create an org, set up teams with appropriate permissions, assign repos to teams, and manage membership — all without a web browser.

## Setup

Set `FORGEJO_URL` and `FORGEJO_ACCESS_TOKEN` environment variables (or use direnv), then build:

```bash
make build
```

## CLI mode: tool discovery

```bash
./forgejo-mcp --cli list 2>/dev/null | sed -n '/^ORG:/,/^$/p'
```

```output
ORG:
  add_team_member                          Add a user to a team
  add_team_repo                            Add a repository to a team
  check_org_membership                     Check if a user is a member of an organization
  create_org                               Create an organization
  create_org_team                          Create a team in an organization
  delete_org                               Delete an organization. WARNING: This is destructive and irreversible — all repos, teams, and data will be permanently removed
  edit_org                                 Edit organization settings
  get_org                                  Get organization details
  list_my_orgs                             List my organizations
  list_org_members                         List members of an organization
  list_org_teams                           List teams in an organization
  list_user_orgs                           List a user's organizations
  remove_org_member                        Remove a member from an organization
  remove_team_member                       Remove a user from a team
  remove_team_repo                         Remove a repository from a team
```

## CLI mode: parameter schemas

```bash
./forgejo-mcp --cli create_org --help 2>/dev/null
```

```output
Tool: create_org
Description: Create an organization

Parameters:
  description          string     optional   Description
  full_name            string     optional   Display name
  location             string     optional   Location
  name                 string     required   Organization username
  visibility           string     optional   Visibility: public, limited, or private
  website              string     optional   Website URL
```

```bash
./forgejo-mcp --cli create_org_team --help 2>/dev/null
```

```output
Tool: create_org_team
Description: Create a team in an organization

Parameters:
  can_create_org_repo  boolean    optional   Whether members can create repos in the org
  description          string     optional   Description
  includes_all_repositories boolean    optional   Whether team has access to all org repos
  name                 string     required   Team name
  org                  string     required   Organization name
  permission           string     optional   Access level: read, write, or admin (default: read)
```

## CLI mode: live demos

### List my organizations

```bash
./forgejo-mcp --cli list_my_orgs --args '{"page":1,"limit":10}' 2>/dev/null
```

```output
  b4mad                           vis=public
  feeldata                        vis=public
  kunsttherapie-bonn              vis=private
  machdenstaat                    vis=public
  open-by-default                 vis=public
  operate-first                   vis=public    Open-sourcing operations on community-managed clusters
  sportverein-vilich-mueldorf     vis=public
  sustainablesupplychain          vis=public
  tinytalesshop                   vis=public
  toolbxs                         vis=public    This is the Home of a set of toolbox container images
```

### Get organization details

```bash
./forgejo-mcp --cli get_org --args '{"org":"operate-first"}' 2>/dev/null
```

```output
  id               105277
  username         operate-first
  description      Open-sourcing operations on community-managed clusters
  website          https://www.operate-first.cloud/
  visibility       public
```

### List organization members

```bash
./forgejo-mcp --cli list_org_members --args '{"org":"operate-first","page":1,"limit":10}' 2>/dev/null
```

```output
  b4mad-renovate             the #B4mad Renovate bot
  durandom
  goern                      Christoph Görn
  op1st-gitops
  schwesig                   Thorsten Schwesig
```

### Check membership

```bash
./forgejo-mcp --cli check_org_membership --args '{"org":"operate-first","user":"goern"}' 2>/dev/null
```

```output
  user=goern  org=operate-first  is_member=True
```

```bash
./forgejo-mcp --cli check_org_membership --args '{"org":"operate-first","user":"torvalds"}' 2>/dev/null
```

```output
  user=torvalds  org=operate-first  is_member=False
```

### List organization teams

```bash
./forgejo-mcp --cli list_org_teams --args '{"org":"operate-first","page":1,"limit":10}' 2>/dev/null
```

```output
  id=21889    devops                permission=admin
  id=21890    Members               permission=read
  id=9018     Owners                permission=owner
```

## MCP stdio mode: tool discovery

An MCP client sends a `tools/list` request via JSON-RPC over stdio and receives all 15 org tools:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./forgejo-mcp 2>/dev/null
```

```output
  add_team_member                 Add a user to a team
  add_team_repo                   Add a repository to a team
  check_org_membership            Check if a user is a member of an organization
  create_org                      Create an organization
  create_org_team                 Create a team in an organization
  delete_org                      Delete an organization. WARNING: This is destructive and irreversible
  edit_org                        Edit organization settings
  get_org                         Get organization details
  list_my_orgs                    List my organizations
  list_org_members                List members of an organization
  list_org_teams                  List teams in an organization
  list_user_orgs                  List a user's organizations
  remove_org_member               Remove a member from an organization
  remove_team_member              Remove a user from a team
  remove_team_repo                Remove a repository from a team

  Total org tools: 15
```

## MCP stdio mode: calling a tool

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"demo","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_org","arguments":{"org":"b4mad"}}}' | ./forgejo-mcp 2>/dev/null
```

```output
{
  "id": 56209,
  "username": "b4mad",
  "full_name": "",
  "avatar_url": "https://codeberg.org/avatars/e37da9ccc4009c35f9ae121fb05a508b",
  "description": "",
  "website": "https://web.b4mad.net/",
  "location": "",
  "visibility": "public"
}
```

## End-to-end autonomous workflow

With these tools, an AI agent can execute a full org setup without human intervention:

1. `list_my_orgs` — discover existing organizations
2. `create_org` — create a new org with name, description, visibility
3. `create_org_team` — set up teams (e.g., "developers" with write, "reviewers" with read)
4. `add_team_member` — add users to teams by team ID
5. `add_team_repo` — assign repos to teams
6. `check_org_membership` — verify membership at any point
7. `list_org_teams` — discover team IDs for mutation operations

The team ID discovery flow (`list_org_teams` → use returned `id` field) is required before team mutations, since team names are not unique across organizations.
