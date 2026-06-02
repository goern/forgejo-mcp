## ADDED Requirements

### Requirement: Single label resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/label/{id}` and MIME type `application/json` describing a single repo label. A label URI SHALL always carry its full scope path (`repo/{owner}/{repo}/...` or, for org labels, `org/{org}/...`); a scope-less `forgejo://label/{id}` form is NOT valid, because repo and org label ids share an integer space and would otherwise be ambiguous.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/label/{id}"`
- **AND** the template's `mimeType` SHALL be `application/json`

#### Scenario: Existing label resolves
- **WHEN** a client reads `forgejo://repo/goern/forgejo-mcp/label/335058`
- **THEN** the response SHALL contain one `application/json` content block
- **AND** the JSON SHALL include `id = 335058`, a non-empty `name`, and a `color`

#### Scenario: Non-numeric id rejected
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/label/abc`
- **THEN** the server SHALL return MCP error code `-32602`
- **AND** SHALL NOT call the upstream API

#### Scenario: Missing label returns -32003
- **WHEN** a client reads a `forgejo://repo/{owner}/{repo}/label/{id}` URI for an id that does not exist
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

#### Scenario: Private repo without access returns -32002
- **WHEN** a client reads a label URI for a private repo
- **AND** the authenticated token lacks read permission
- **THEN** the server SHALL return MCP error code `-32002`

### Requirement: Bounded label list resource template

The server SHALL register a resource template with URI `forgejo://repo/{owner}/{repo}/labels{?page,limit}` and MIME type `application/json` returning a bounded list of repo labels. The list SHALL accept optional `page` and `limit` query parameters as the **client-controlled bound** (mirroring `fetchOrgLabels`), fetch that page, and apply `operation/resource.Bounded` with `EmbeddedListCap` only as a hard safety ceiling. When the result is truncated — by `limit`, by the ceiling, or because more pages exist — the response SHALL append the shared truncation sentinel naming `list_repo_labels` and the next `page`. This satisfies `docs/design/output-bounding.md` (client-controlled bound via `page`/`limit` + resumability via the next-page sentinel + documented parameters).

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://repo/{owner}/{repo}/labels{?page,limit}"`
- **AND** the template's `mimeType` SHALL be `application/json`

#### Scenario: Client-controlled limit bounds the page
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/labels?limit=N` for a repo with more than `N` labels
- **THEN** the response SHALL contain at most `N` labels
- **AND** SHALL append a sentinel referencing the next `page`

#### Scenario: Under-cap list returns all labels with no sentinel
- **WHEN** a client reads `forgejo://repo/{owner}/{repo}/labels` for a repo with fewer than `EmbeddedListCap` labels
- **THEN** the response SHALL contain every label
- **AND** SHALL NOT contain a truncation sentinel

#### Scenario: Over-cap list is truncated with a sentinel naming the list tool
- **WHEN** a client reads the labels resource for a repo with more than `EmbeddedListCap` labels
- **THEN** the response SHALL contain at most `EmbeddedListCap` labels
- **AND** SHALL contain a truncation sentinel naming `list_repo_labels` as the enumeration fallback

#### Scenario: Missing repo returns -32003
- **WHEN** a client reads the labels resource for a repo that does not exist
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Bounded org-label list resource template

The server SHALL register a resource template with URI `forgejo://org/{org}/labels{?page,limit}` and MIME type `application/json` returning a bounded list of an organization's labels. As with the repo list, it SHALL accept optional `page`/`limit` as the client-controlled bound, apply `operation/resource.Bounded` with `EmbeddedListCap` as the ceiling, and append the shared truncation sentinel naming `list_org_labels` and the next `page` when truncated. This satisfies the `mcp-resources-core` Collection resource requirement and `docs/design/output-bounding.md`.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://org/{org}/labels{?page,limit}"`
- **AND** the template's `mimeType` SHALL be `application/json`

#### Scenario: Over-cap org-label list is truncated with a sentinel naming the list tool
- **WHEN** a client reads `forgejo://org/{org}/labels` for an org with more than `EmbeddedListCap` labels
- **THEN** the response SHALL contain at most `EmbeddedListCap` labels
- **AND** SHALL contain a truncation sentinel naming `list_org_labels` as the enumeration fallback

#### Scenario: Missing org returns -32003
- **WHEN** a client reads the org-label list for an org that does not exist
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Label resources documented

The README "Resources" section and the `AGENTS.md` resource table SHALL list all three label resource-templates.

#### Scenario: Docs list all templates
- **WHEN** the README Resources section is rendered
- **THEN** it SHALL include rows for `forgejo://repo/{owner}/{repo}/labels`, `forgejo://repo/{owner}/{repo}/label/{id}`, and `forgejo://org/{org}/labels`
