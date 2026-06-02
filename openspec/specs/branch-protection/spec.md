# branch-protection Specification

## Purpose
TBD - created by archiving change branch-protection-management. Update Purpose after archive.
## Requirements
### Requirement: List branch protection rules (bounded)

The server SHALL provide a `list_branch_protections` tool that returns the branch protection rules of a repository given `owner` and `repo`. The tool SHALL expose caller-controlled `page` and `limit` bounds and the response SHALL be resumable by echoing the page that was returned, satisfying `docs/design/output-bounding.md`.

#### Scenario: List returns the repository's rules

- **WHEN** `list_branch_protections` is called with a valid `owner` and `repo`
- **THEN** the result SHALL contain the repository's branch protection rules as returned by `GET /repos/{owner}/{repo}/branch_protections`
- **AND** each rule SHALL include at least its `rule_name`, `branch_name`, `enable_status_check`, `status_check_contexts`, and `required_approvals`

#### Scenario: Caller bounds the page size

- **WHEN** `list_branch_protections` is called with `limit` set to N
- **THEN** the request to Forgejo SHALL carry a page size of N
- **AND** the response SHALL indicate the page returned so the caller can request the next page

### Requirement: Get a branch protection rule

The server SHALL provide a `get_branch_protection` tool that returns a single rule given `owner`, `repo`, and the rule identifier (`rule`).

#### Scenario: Existing rule is returned

- **WHEN** `get_branch_protection` is called with an `owner`, `repo`, and a `rule` that exists
- **THEN** the result SHALL contain that rule's full protection state

#### Scenario: Missing rule yields an error

- **WHEN** `get_branch_protection` is called with a `rule` that does not exist
- **THEN** the tool SHALL return an error result rather than an empty success

### Requirement: Create a branch protection rule

The server SHALL provide a `create_branch_protection` tool that creates a rule via `POST /repos/{owner}/{repo}/branch_protections`. The tool SHALL require `owner`, `repo`, and `branch_name`; `rule_name` SHALL be optional (Forgejo defaults it to `branch_name`). The tool SHALL round-trip `status_check_contexts` exactly.

#### Scenario: Create enforces status checks

- **WHEN** `create_branch_protection` is called with `enable_status_check` true and `status_check_contexts` set to a list of contexts
- **THEN** the request body sent to Forgejo SHALL contain `enable_status_check: true` and the exact `status_check_contexts` list
- **AND** the result SHALL reflect the created rule with those contexts

#### Scenario: Create requires a branch name

- **WHEN** `create_branch_protection` is called without `branch_name`
- **THEN** the tool SHALL return an error result and SHALL NOT call Forgejo

### Requirement: Edit a branch protection rule with PATCH semantics

The server SHALL provide an `edit_branch_protection` tool that updates a rule via `PATCH /repos/{owner}/{repo}/branch_protections/{rule}`. The tool SHALL only send fields the caller explicitly provides; fields the caller omits SHALL be left unchanged on the server.

#### Scenario: Editing one field leaves others untouched

- **WHEN** `edit_branch_protection` is called with only `required_approvals` set to 2
- **THEN** the request body sent to Forgejo SHALL set `required_approvals` to 2
- **AND** the other protection fields the caller did not pass SHALL be sent as `null` (leave-unchanged), never as a concrete value such as `false` that would silently relax protection

#### Scenario: Editing status checks round-trips the contexts

- **WHEN** `edit_branch_protection` is called with `status_check_contexts` set to a list
- **THEN** the request body SHALL contain that exact list

### Requirement: Delete a branch protection rule

The server SHALL provide a `delete_branch_protection` tool that removes a rule via `DELETE /repos/{owner}/{repo}/branch_protections/{rule}` and reports success.

#### Scenario: Existing rule is deleted

- **WHEN** `delete_branch_protection` is called for a rule that exists
- **THEN** Forgejo SHALL receive a delete request for that rule
- **AND** the tool SHALL return a success result confirming removal

### Requirement: Branch protections collection resource (bounded)

The server SHALL expose a resource-template `forgejo://repo/{owner}/{repo}/branch_protections` that returns the repository's protection rules as a read-only JSON document. The embedded list SHALL be bounded at `EmbeddedListCap` using the shared `resource.Bounded` helper, and when truncated SHALL carry the truncation sentinel and name `list_branch_protections` as the tool to fetch the remainder.

#### Scenario: Collection resource returns bounded rules

- **WHEN** the resource `forgejo://repo/{owner}/{repo}/branch_protections` is read for a repo with rules
- **THEN** the payload SHALL list the rules up to `EmbeddedListCap`
- **AND** when more than `EmbeddedListCap` rules exist, the payload SHALL set a truncation indicator naming `list_branch_protections`

#### Scenario: Unknown repository maps to a not-found resource error

- **WHEN** the collection resource is read for a repository that does not exist
- **THEN** the handler SHALL return a resource error mapped from the Forgejo 404 response

### Requirement: Single branch protection resource

The server SHALL expose a resource-template `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` that returns one rule's protection state as a read-only JSON document.

#### Scenario: Single rule resource returns its state

- **WHEN** the resource `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` is read for an existing rule
- **THEN** the payload SHALL contain that rule's protection state including `status_check_contexts` and `required_approvals`

#### Scenario: Malformed URI maps to an invalid-params error

- **WHEN** a `forgejo://repo/...` branch-protection URI is read that does not match the template
- **THEN** the handler SHALL return a resource error with the invalid-params code

