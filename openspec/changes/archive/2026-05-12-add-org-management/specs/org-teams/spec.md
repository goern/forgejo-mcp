## ADDED Requirements

### Requirement: List organization teams
The system SHALL provide a `list_org_teams` MCP tool that lists teams in an organization. The tool SHALL accept:
- `org` (required): organization name
- `page` (required, default 1): page number
- `limit` (required, default 100): page size

The tool SHALL return a list of team objects including id, name, description, and permission level.

#### Scenario: List teams in an organization
- **WHEN** user calls `list_org_teams` with `org` set to an existing organization
- **THEN** the system returns a list of team objects for that organization

#### Scenario: List teams with pagination
- **WHEN** user calls `list_org_teams` with `org`, `page` set to 1, `limit` set to 10
- **THEN** the system returns at most 10 teams from the first page

### Requirement: Create organization team
The system SHALL provide a `create_org_team` MCP tool that creates a team within an organization. The tool SHALL accept:
- `org` (required): organization name
- `name` (required): team name
- `description` (optional): team description
- `permission` (optional): access level — one of `read`, `write`, `admin` (default: `read`)
- `can_create_org_repo` (optional, default false): whether team members can create repos in the org
- `includes_all_repositories` (optional, default false): whether the team has access to all org repos

The tool SHALL return the created team object including its `id`.

#### Scenario: Create a team with defaults
- **WHEN** user calls `create_org_team` with `org` set to an existing organization and `name` set to `"developers"`
- **THEN** the system creates a team with `read` permission and returns the team object with its assigned `id`

#### Scenario: Create a team with custom permissions
- **WHEN** user calls `create_org_team` with `org`, `name`, `permission` set to `"write"`, and `includes_all_repositories` set to `true`
- **THEN** the system creates a team with write access to all repositories and returns the team object

#### Scenario: Create team in non-existent organization
- **WHEN** user calls `create_org_team` with `org` set to a name that does not exist
- **THEN** the system returns an error indicating the organization was not found

### Requirement: Add team member
The system SHALL provide an `add_team_member` MCP tool that adds a user to a team. The tool SHALL accept:
- `team_id` (required): numeric team ID
- `user` (required): username to add

The tool SHALL return a success confirmation.

#### Scenario: Add user to team
- **WHEN** user calls `add_team_member` with a valid `team_id` and `user`
- **THEN** the system adds the user to the team and returns a success message

#### Scenario: Add user to non-existent team
- **WHEN** user calls `add_team_member` with a `team_id` that does not exist
- **THEN** the system returns an error indicating the team was not found

### Requirement: Remove team member
The system SHALL provide a `remove_team_member` MCP tool that removes a user from a team. The tool SHALL accept:
- `team_id` (required): numeric team ID
- `user` (required): username to remove

The tool SHALL return a success confirmation.

#### Scenario: Remove user from team
- **WHEN** user calls `remove_team_member` with a valid `team_id` and `user` who is a current member
- **THEN** the system removes the user from the team and returns a success message

### Requirement: Add repository to team
The system SHALL provide an `add_team_repo` MCP tool that grants a team access to a repository. The tool SHALL accept:
- `team_id` (required): numeric team ID
- `org` (required): organization name (owner of the repo)
- `repo` (required): repository name

The tool SHALL return a success confirmation.

#### Scenario: Add repo to team
- **WHEN** user calls `add_team_repo` with a valid `team_id`, `org`, and `repo`
- **THEN** the system adds the repository to the team's access list and returns a success message

#### Scenario: Add non-existent repo to team
- **WHEN** user calls `add_team_repo` with a valid `team_id` but a `repo` that does not exist
- **THEN** the system returns an error indicating the repository was not found

### Requirement: Remove repository from team
The system SHALL provide a `remove_team_repo` MCP tool that revokes a team's access to a repository. The tool SHALL accept:
- `team_id` (required): numeric team ID
- `org` (required): organization name (owner of the repo)
- `repo` (required): repository name

The tool SHALL return a success confirmation.

#### Scenario: Remove repo from team
- **WHEN** user calls `remove_team_repo` with a valid `team_id`, `org`, and `repo` that the team currently has access to
- **THEN** the system removes the repository from the team's access list and returns a success message
