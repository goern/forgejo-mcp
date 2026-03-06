## ADDED Requirements

### Requirement: List repository milestones
The system SHALL expose a `list_repo_milestones` MCP tool that returns all milestones for a given repository. The tool SHALL accept `owner` and `repo` as required string parameters. The tool SHALL accept `page` (number, required, default 1, min 1) and `limit` (number, required, default 100, min 1) parameters for pagination. The tool SHALL accept an optional `state` string parameter with values `open`, `closed`, or `all` (default: `open`). The tool SHALL call the Forgejo SDK `ListRepoMilestones` method and return a JSON array where each element contains at minimum `id` (numeric), `title` (string), `description` (string), `state` (string), `open_issues` (number), and `closed_issues` (number).

#### Scenario: List open milestones
- **WHEN** the tool is called with owner="org", repo="project", page=1, limit=100
- **THEN** the system returns a JSON array of open milestones each with id, title, description, state, open_issues, closed_issues

#### Scenario: List all milestones including closed
- **WHEN** the tool is called with state="all"
- **THEN** the system returns milestones in all states (open and closed)

#### Scenario: List milestones for repo with none defined
- **WHEN** the tool is called on a repo that has no milestones
- **THEN** the system returns an empty JSON array

#### Scenario: Paginated results
- **WHEN** the tool is called with page=2, limit=10
- **THEN** the system returns the second page of up to 10 milestones

#### Scenario: Repository does not exist
- **WHEN** the tool is called with a non-existent owner/repo combination
- **THEN** the SDK returns an error and the tool propagates it to the caller

---

### Requirement: List repository labels
The system SHALL expose a `list_repo_labels` MCP tool that returns all labels defined for a given repository. The tool SHALL accept `owner` and `repo` as required string parameters. The tool SHALL accept `page` (number, required, default 1, min 1) and `limit` (number, required, default 100, min 1) parameters for pagination. The tool SHALL call the Forgejo SDK `ListRepoLabels` method and return a JSON array where each element contains at minimum `id` (numeric), `name` (string), `color` (string, hex), and `description` (string).

#### Scenario: List labels
- **WHEN** the tool is called with owner="org", repo="project", page=1, limit=100
- **THEN** the system returns a JSON array of labels each with id, name, color, description

#### Scenario: List labels for repo with none defined
- **WHEN** the tool is called on a repo that has no labels
- **THEN** the system returns an empty JSON array

#### Scenario: Paginated results
- **WHEN** the tool is called with page=2, limit=10
- **THEN** the system returns the second page of up to 10 labels

#### Scenario: Repository does not exist
- **WHEN** the tool is called with a non-existent owner/repo combination
- **THEN** the SDK returns an error and the tool propagates it to the caller
