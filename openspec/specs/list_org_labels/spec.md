# list_org_labels Specification

## Purpose
TBD - created by archiving change add-org-label-support. Update Purpose after archive.
## Requirements
### Requirement: List organization labels
The system SHALL expose a `list_org_labels` MCP tool that returns all labels defined at the organization level for a given Forgejo organization. The tool SHALL accept `org` as a required string parameter. The tool SHALL accept `page` (number, required, default 1, min 1) and `limit` (number, required, default 100, min 1) parameters for pagination. The tool SHALL fetch `/orgs/{org}/labels` via the existing raw-HTTP helper `pkg/forgejo.DoJSONList` and return a JSON array where each element contains at minimum `id` (numeric), `name` (string), `color` (string, hex), `description` (string), and `scope` (string, always `"org"`).

#### Scenario: List org labels
- **WHEN** the tool is called with org="codeberg", page=1, limit=100
- **THEN** the system returns a JSON array of org-scoped labels each with `id`, `name`, `color`, `description`, and `scope: "org"`

#### Scenario: Org has no labels defined
- **WHEN** the tool is called on an org with no labels
- **THEN** the system returns an empty JSON array

#### Scenario: Org does not exist
- **WHEN** the tool is called with an unknown org name
- **THEN** the system returns an empty JSON array (`DoJSONList` maps 404 to empty)

#### Scenario: Paginated results
- **WHEN** the tool is called with page=2, limit=10
- **THEN** the system returns the second page of up to 10 org labels

#### Scenario: Authentication failure propagates
- **WHEN** the tool is called and the Forgejo API responds with 401 or 403
- **THEN** the tool returns an error wrapping `forgejo.ErrUnauthorized`

