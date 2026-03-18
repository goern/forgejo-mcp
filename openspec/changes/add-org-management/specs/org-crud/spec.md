## ADDED Requirements

### Requirement: Create organization
The system SHALL provide a `create_org` MCP tool that creates a new organization on the Forgejo instance. The tool SHALL accept:
- `name` (required): organization username
- `full_name` (optional): display name
- `description` (optional): organization description
- `website` (optional): organization website URL
- `location` (optional): organization location
- `visibility` (optional): one of `public`, `limited`, `private` (default: `public`)

The tool SHALL return the created organization object.

#### Scenario: Successful organization creation
- **WHEN** user calls `create_org` with `name` set to `"my-org"`
- **THEN** the system creates the organization and returns the organization object with matching `username`

#### Scenario: Create organization with all fields
- **WHEN** user calls `create_org` with `name`, `full_name`, `description`, `website`, `location`, and `visibility` set to `"private"`
- **THEN** the system creates the organization with all provided fields and returns the organization object

#### Scenario: Create organization with duplicate name
- **WHEN** user calls `create_org` with a `name` that already exists
- **THEN** the system returns an error indicating the organization name is taken

### Requirement: Get organization details
The system SHALL provide a `get_org` MCP tool that retrieves organization details by name. The tool SHALL accept:
- `org` (required): organization name

The tool SHALL return the organization object including id, username, full_name, description, website, location, visibility, and avatar_url.

#### Scenario: Get existing organization
- **WHEN** user calls `get_org` with `org` set to an existing organization name
- **THEN** the system returns the organization object with all fields populated

#### Scenario: Get non-existent organization
- **WHEN** user calls `get_org` with `org` set to a name that does not exist
- **THEN** the system returns an error indicating the organization was not found

### Requirement: List authenticated user's organizations
The system SHALL provide a `list_my_orgs` MCP tool that lists organizations the authenticated user belongs to. The tool SHALL accept:
- `page` (required, default 1): page number
- `limit` (required, default 100): page size

The tool SHALL return a list of organization objects.

#### Scenario: List organizations for authenticated user
- **WHEN** user calls `list_my_orgs` with default pagination
- **THEN** the system returns a list of organizations the authenticated user is a member of

#### Scenario: List organizations with pagination
- **WHEN** user calls `list_my_orgs` with `page` set to 2 and `limit` set to 10
- **THEN** the system returns the second page of results with at most 10 organizations

### Requirement: List user's organizations
The system SHALL provide a `list_user_orgs` MCP tool that lists organizations for a given user. The tool SHALL accept:
- `user` (required): username
- `page` (required, default 1): page number
- `limit` (required, default 100): page size

The tool SHALL return a list of organization objects visible to the authenticated user.

#### Scenario: List organizations for a specific user
- **WHEN** user calls `list_user_orgs` with `user` set to an existing username
- **THEN** the system returns a list of organizations that user belongs to (filtered by visibility)

### Requirement: Edit organization
The system SHALL provide an `edit_org` MCP tool that updates organization settings. The tool SHALL accept:
- `org` (required): organization name
- `full_name` (optional): new display name
- `description` (optional): new description
- `website` (optional): new website URL
- `location` (optional): new location
- `visibility` (optional): new visibility (`public`, `limited`, `private`)

Only provided fields SHALL be updated; omitted fields SHALL remain unchanged.

#### Scenario: Update organization description
- **WHEN** user calls `edit_org` with `org` and `description` set to a new value
- **THEN** the system updates only the description and returns the updated organization object

#### Scenario: Edit non-existent organization
- **WHEN** user calls `edit_org` with `org` set to a name that does not exist
- **THEN** the system returns an error indicating the organization was not found

### Requirement: Delete organization
The system SHALL provide a `delete_org` MCP tool that deletes an organization. The tool description SHALL include a warning that this action is destructive and irreversible. The tool SHALL accept:
- `org` (required): organization name

The tool SHALL return a success confirmation upon deletion.

#### Scenario: Delete existing organization
- **WHEN** user calls `delete_org` with `org` set to an existing organization the user owns
- **THEN** the system deletes the organization and returns a success message

#### Scenario: Delete organization without permission
- **WHEN** user calls `delete_org` with `org` set to an organization the user does not own
- **THEN** the system returns an error indicating insufficient permissions
