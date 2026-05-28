## ADDED Requirements

### Requirement: Owner resource template

The server SHALL register a resource template with URI `forgejo://owner/{owner}` and MIME type `application/json` describing a Forgejo user or organization.

#### Scenario: Template appears in templates list
- **WHEN** a client issues `resources/templates/list`
- **THEN** the response SHALL include a template with `uriTemplate = "forgejo://owner/{owner}"`
- **AND** the template's `mimeType` SHALL be `application/json`

### Requirement: Owner resource read

`resources/read` for a `forgejo://owner/{owner}` URI SHALL return a JSON content block containing the owner's identity metadata: `login`, `full_name`, `kind` (`"user"` or `"organization"`), `avatar_url`, `description`, `created_at`. Fields absent from the upstream response SHALL be omitted from the JSON.

#### Scenario: Existing user resolves
- **WHEN** a client reads `forgejo://owner/goern`
- **AND** `goern` exists on the configured Forgejo instance
- **THEN** the response SHALL contain one `application/json` content block
- **AND** the JSON SHALL include `login = "goern"` and a `kind` field

#### Scenario: Missing owner returns -32003
- **WHEN** a client reads `forgejo://owner/does-not-exist`
- **AND** the upstream returns `404`
- **THEN** the server SHALL return MCP error code `-32003`

### Requirement: Owner resource is not flagged as immutable

The owner template SHALL NOT carry a description claiming immutability. Owner metadata is mutable (display name, avatar, description can change).

#### Scenario: Description omits cache claim
- **WHEN** a client inspects the owner template's description in `resources/templates/list`
- **THEN** the description SHALL NOT contain the word "immutable" or "cacheable"
