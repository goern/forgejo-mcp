# list-milestones-labels Specification

## Purpose
TBD - created by archiving change list-milestones-labels. Update Purpose after archive.
## Requirements
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
The system SHALL expose a `list_repo_labels` MCP tool that returns labels usable on issues in a given repository. The tool SHALL accept `owner` and `repo` as required string parameters. The tool SHALL accept `page` (number, required, default 1, min 1) and `limit` (number, required, default 100, min 1) parameters for pagination. The tool SHALL accept an optional `include_org_labels` boolean parameter (default `true`).

When `include_org_labels` is `true` and the `owner` corresponds to an organization, the tool SHALL also fetch `/orgs/{owner}/labels` via `pkg/forgejo.DoJSONList` and merge the returned labels into the response. Each label in the response SHALL carry a `scope` field of either `"repo"` or `"org"` so callers can disambiguate ID spaces. Each element SHALL contain at minimum `id` (numeric), `name` (string), `color` (string, hex), `description` (string), and `scope` (string).

When `include_org_labels` is `false`, the tool SHALL return only repo-scoped labels, each tagged `scope: "repo"`.

If the org-labels fetch returns 404 (owner is not an org, or org has no labels), the tool SHALL omit org labels without raising an error.

#### Scenario: List labels for org-owned repo with merge enabled
- **WHEN** the tool is called with owner="codeberg-org", repo="project", page=1, limit=100 (include_org_labels defaults to true)
- **THEN** the system returns a JSON array containing both repo labels (each with `scope: "repo"`) and org labels (each with `scope: "org"`)

#### Scenario: List labels for user-owned repo
- **WHEN** the tool is called with owner="alice", repo="project" where alice is a user, not an org
- **THEN** the system returns repo labels only, each with `scope: "repo"`; the org-labels fetch returns 404 and is silently treated as empty

#### Scenario: Disable org-label merge
- **WHEN** the tool is called with include_org_labels=false on an org-owned repo
- **THEN** the system returns repo labels only, each with `scope: "repo"`, even though the owner is an org

#### Scenario: List labels for repo with none defined
- **WHEN** the tool is called on a repo with no repo labels and the owner is a user (or include_org_labels=false)
- **THEN** the system returns an empty JSON array

#### Scenario: Paginated results
- **WHEN** the tool is called with page=2, limit=10
- **THEN** the system returns the second page of up to 10 labels in each scope, merged

#### Scenario: Repository does not exist
- **WHEN** the tool is called with a non-existent owner/repo combination
- **THEN** the SDK returns an error from the repo-labels fetch and the tool propagates it to the caller

#### Scenario: Org-labels fetch authentication failure
- **WHEN** the tool is called with include_org_labels=true and the org-labels fetch returns 401 or 403
- **THEN** the tool returns an error wrapping `forgejo.ErrUnauthorized` (does not silently drop org labels on auth failure)

