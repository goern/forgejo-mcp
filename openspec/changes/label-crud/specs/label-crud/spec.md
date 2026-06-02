## ADDED Requirements

### Requirement: Create repo label tool

The server SHALL register an MCP tool `create_repo_label` accepting `owner` (string), `repo` (string), `name` (string), `color` (string), and optional `description` (string). It SHALL create the label via the pinned `forgejo-sdk/v3` `CreateLabel` method and return the created label including its numeric `id`, `name`, `color`, and `description`.

#### Scenario: Create returns the new label with id
- **WHEN** a client calls `create_repo_label` with `owner`, `repo`, `name="RFC"`, `color="#0e8a16"`
- **THEN** the tool SHALL create the label upstream
- **AND** the result SHALL include a numeric `id`, `name = "RFC"`, and `color = "#0e8a16"`

#### Scenario: Description is optional
- **WHEN** a client calls `create_repo_label` without `description`
- **THEN** the tool SHALL create the label with an empty description
- **AND** SHALL NOT fail for the missing field

### Requirement: Color normalisation

The `create_repo_label` and `edit_repo_label` tools SHALL normalise the `color` argument to the Forgejo hex form before sending: accept `rrggbb` or `#rrggbb` (6-digit only), lowercase it, and prepend `#` if absent. A `color` value that is not a valid 6-digit hex color SHALL be rejected with MCP error code `-32602` (invalid params) rather than forwarded to the API. 3-digit shorthand SHALL NOT be silently expanded (upstream acceptance unverified — see the change's Open Questions).

#### Scenario: Missing hash is prepended
- **WHEN** a client calls `create_repo_label` with `color="0e8a16"`
- **THEN** the value sent upstream SHALL be `#0e8a16`
- **AND** the call SHALL NOT return a `422`

#### Scenario: Invalid color rejected at the boundary
- **WHEN** a client calls `create_repo_label` with `color="not-a-color"`
- **THEN** the tool SHALL return MCP error code `-32602`
- **AND** SHALL NOT call the upstream API

### Requirement: Edit repo label tool (PATCH semantics)

The server SHALL register an MCP tool `edit_repo_label` accepting `owner` (string), `repo` (string), `id` (number), and optional `name`, `color`, `description`. Only supplied fields SHALL be sent in the `EditLabelOption`; unsupplied fields SHALL be left unchanged upstream. It SHALL return the updated label.

#### Scenario: Only supplied fields change
- **WHEN** a client calls `edit_repo_label` with `id` and `color="#d32f2f"` only
- **THEN** the label's `color` SHALL change to `#d32f2f`
- **AND** the label's `name` and `description` SHALL be unchanged

#### Scenario: No fields supplied is rejected
- **WHEN** a client calls `edit_repo_label` with only `owner`, `repo`, `id` and no editable field
- **THEN** the tool SHALL return MCP error code `-32602`
- **AND** SHALL NOT call the upstream API

### Requirement: Delete repo label tool with in-use guard

The server SHALL register an MCP tool `delete_repo_label` accepting `owner` (string), `repo` (string), `id` (number), and optional `delete_mode` (string enum `"safe"` | `"force"`, default `"safe"`). Before deleting, the tool SHALL count issues and pull requests in the repo that reference the label. If that count is `> 0` and `delete_mode` is not `"force"`, the tool SHALL NOT delete the label and SHALL return an error reporting the in-use count. If the count is `0`, or `delete_mode` is `"force"`, the tool SHALL delete the label via `DeleteLabel` and report success. (A string enum, not a boolean, reserves room for future modes such as `"dry_run"` or `"reassign"` without a breaking parameter change.)

#### Scenario: Unused label is deleted
- **WHEN** a client calls `delete_repo_label` for a label referenced by no issue or PR
- **THEN** the tool SHALL delete the label upstream
- **AND** SHALL return a success result

#### Scenario: In-use label without force is refused with a count
- **WHEN** a client calls `delete_repo_label` for a label referenced by 5 issues/PRs
- **AND** `delete_mode` is omitted (or `"safe"`)
- **THEN** the tool SHALL NOT delete the label
- **AND** SHALL return an error reporting the in-use count (`5`)

#### Scenario: In-use label with force is deleted
- **WHEN** a client calls `delete_repo_label` for an in-use label with `delete_mode="force"`
- **THEN** the tool SHALL delete the label upstream
- **AND** SHALL return a success result

#### Scenario: Missing label maps to not-found
- **WHEN** a client calls `delete_repo_label` with an `id` that does not exist
- **AND** the upstream returns `404`
- **THEN** the tool SHALL surface a not-found error to the client

### Requirement: Get one repo label tool

The server SHALL register an MCP tool `get_repo_label` accepting `owner` (string), `repo` (string), `id` (number), returning a single label (`id`, `name`, `color`, `description`) via `GetRepoLabel`.

#### Scenario: Reads one label by id
- **WHEN** a client calls `get_repo_label` with a valid `id`
- **THEN** the result SHALL contain that label's `id`, `name`, `color`, and `description`

#### Scenario: Missing label returns not-found
- **WHEN** a client calls `get_repo_label` with a non-existent `id`
- **AND** the upstream returns `404`
- **THEN** the tool SHALL surface a not-found error

### Requirement: Org-label CRUD tools via raw HTTP

The server SHALL register MCP tools `create_org_label`, `edit_org_label`, `delete_org_label`, and `get_org_label` operating on `/orgs/{org}/labels[/{id}]`. Because `forgejo-sdk/v3` provides no org-label method, these SHALL be implemented through the `pkg/forgejo` raw-HTTP `DoJSON` helper, mirroring the existing `fetchOrgLabels` precedent. They SHALL share the repo tools' `color` normalisation, PATCH edit semantics, and (for delete) the in-use guard and `delete_mode` enum.

#### Scenario: Create org label
- **WHEN** a client calls `create_org_label` with `org`, `name`, `color`
- **THEN** the tool SHALL `POST /orgs/{org}/labels` via `DoJSON`
- **AND** SHALL return the created label including a numeric `id`

#### Scenario: Edit org label is PATCH
- **WHEN** a client calls `edit_org_label` with `id` and `color` only
- **THEN** only `color` SHALL change upstream
- **AND** `name` and `description` SHALL be unchanged

#### Scenario: Get one org label
- **WHEN** a client calls `get_org_label` with a valid `id`
- **THEN** the result SHALL contain that label's `id`, `name`, `color`, and `description`

#### Scenario: In-use org label without force is refused with a best-effort count
- **WHEN** a client calls `delete_org_label` for a label referenced by issues/PRs across the org's repos
- **AND** `delete_mode` is omitted (or `"safe"`)
- **THEN** the tool SHALL NOT delete the label
- **AND** SHALL return an error reporting the in-use count
- **AND** the error SHALL state the count is best-effort and excludes repos the token cannot access

#### Scenario: Org label color normalised
- **WHEN** a client calls `create_org_label` with `color="0e8a16"`
- **THEN** the value sent upstream SHALL be `#0e8a16`

### Requirement: Discoverable in tool index and docs

The eight label tools SHALL be listed in the README tool table and `AGENTS.md` so a `ToolSearch` for `label` surfaces them.

#### Scenario: README lists the new tools
- **WHEN** the README tool table is rendered
- **THEN** it SHALL include rows for `create_repo_label`, `edit_repo_label`, `delete_repo_label`, `get_repo_label`, `create_org_label`, `edit_org_label`, `delete_org_label`, and `get_org_label`
