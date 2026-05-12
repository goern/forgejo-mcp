## ADDED Requirements

### Requirement: List organization members
The system SHALL provide a `list_org_members` MCP tool that lists members of an organization. The tool SHALL accept:
- `org` (required): organization name
- `page` (required, default 1): page number
- `limit` (required, default 100): page size

The tool SHALL return a list of user objects representing organization members.

#### Scenario: List members of an organization
- **WHEN** user calls `list_org_members` with `org` set to an existing organization
- **THEN** the system returns a list of user objects who are members of that organization

#### Scenario: List members with pagination
- **WHEN** user calls `list_org_members` with `org`, `page` set to 2, `limit` set to 25
- **THEN** the system returns the second page of members with at most 25 entries

#### Scenario: List members of non-existent organization
- **WHEN** user calls `list_org_members` with `org` set to a name that does not exist
- **THEN** the system returns an error indicating the organization was not found

### Requirement: Check organization membership
The system SHALL provide a `check_org_membership` MCP tool that checks if a user is a member of an organization. The tool SHALL accept:
- `org` (required): organization name
- `user` (required): username to check

The tool SHALL return a boolean-style result indicating membership status.

#### Scenario: User is a member
- **WHEN** user calls `check_org_membership` with `org` and `user` where the user is a member
- **THEN** the system returns a result indicating the user is a member

#### Scenario: User is not a member
- **WHEN** user calls `check_org_membership` with `org` and `user` where the user is not a member
- **THEN** the system returns a result indicating the user is not a member

### Requirement: Remove organization member
The system SHALL provide a `remove_org_member` MCP tool that removes a user from an organization. The tool SHALL accept:
- `org` (required): organization name
- `user` (required): username to remove

The tool SHALL return a success confirmation upon removal.

#### Scenario: Remove existing member
- **WHEN** user calls `remove_org_member` with `org` and `user` where the user is a current member
- **THEN** the system removes the user from the organization and returns a success message

#### Scenario: Remove non-member
- **WHEN** user calls `remove_org_member` with `org` and `user` where the user is not a member
- **THEN** the system returns an error indicating the user is not a member of the organization
