## ADDED Requirements

### Requirement: Resource capability registration

The server SHALL register MCP resource capabilities at construction time by calling `server.WithResourceCapabilities(subscribe=false, listChanged=false)` in `operation/operation.go`'s `newMCPServer` function. Subscribe and listChanged SHALL both be `false` for the v1 surface.

#### Scenario: Server advertises resource capability
- **WHEN** an MCP client issues `initialize` against the server
- **THEN** the server response's `capabilities` object SHALL include a `resources` entry
- **AND** the `resources.subscribe` field SHALL be `false`
- **AND** the `resources.listChanged` field SHALL be `false`

#### Scenario: Subscribe request rejected
- **WHEN** a client sends `resources/subscribe` for any URI
- **THEN** the server SHALL respond with a method-not-supported error

### Requirement: URI scheme namespace

The server SHALL serve resources exclusively under the `forgejo://` URI scheme. No other scheme SHALL be used by resource templates registered by this server.

#### Scenario: All registered templates use forgejo scheme
- **WHEN** a client issues `resources/templates/list`
- **THEN** every `uriTemplate` returned SHALL begin with the literal string `forgejo://`

### Requirement: Per-domain registration entry points

Each `operation/{domain}/` package that owns one or more resources SHALL expose a `RegisterResources(s *server.MCPServer)` function. `operation/operation.go` SHALL invoke each such function exactly once during server construction, parallel to the existing `RegisterXTool` calls.

#### Scenario: Domain registers its resources
- **WHEN** the server starts up
- **THEN** for every domain that owns resources, its `RegisterResources(s)` function SHALL have been invoked
- **AND** the templates it added SHALL appear in `resources/templates/list` output

### Requirement: Shared URI parsing

A package `operation/resource/` SHALL provide URI parsing helpers that translate a `forgejo://...` URI from `mcp.ReadResourceRequest` into a typed struct for each entity (owner, repo, commit, issue, pr, comment, status). Per-domain handlers SHALL use these helpers rather than parsing URIs themselves.

#### Scenario: Valid URI parses into typed struct
- **WHEN** a handler receives a `ReadResourceRequest` with URI `forgejo://repo/goern/forgejo-mcp/commit/df15877777b177d9d237af26fab3c4a829ba61de`
- **THEN** the shared parser SHALL return a struct with `owner="goern"`, `repo="forgejo-mcp"`, `sha="df15877777b177d9d237af26fab3c4a829ba61de"`
- **AND** SHALL return no error

#### Scenario: Malformed URI returns parse error
- **WHEN** a handler receives a URI whose path does not match the expected entity template
- **THEN** the shared parser SHALL return a non-nil error
- **AND** the handler SHALL propagate it as MCP error code `-32602` (invalid params)

### Requirement: Embedded-list bounding and resumability

When a resource handler embeds a variable-size list inside its response (e.g. comments on an issue, statuses on a commit), the response SHALL cap the embedded list at a documented maximum (default 30 items) and SHALL append a sentinel block when truncation occurs. The sentinel SHALL name the existing list tool that callers can use to fetch the remaining items.

#### Scenario: List under cap returns in full
- **WHEN** a resource embeds a list of `M` items where `M ≤ 30`
- **THEN** all `M` items SHALL be included in the response
- **AND** no truncation sentinel SHALL appear

#### Scenario: List over cap is truncated with sentinel
- **WHEN** a resource embeds a list of `M` items where `M > 30`
- **THEN** the first 30 items SHALL be included
- **AND** a sentinel block SHALL be appended containing the substring `[truncated:` and the total count `M`
- **AND** the sentinel SHALL name the corresponding list tool (e.g. `list_issue_comments`)

### Requirement: Coexistence with existing tools

The introduction of resources SHALL NOT remove, rename, or alter the behavior of any tool registered prior to this change. All existing tools SHALL continue to be registered and to respond identically.

#### Scenario: Existing tool list unchanged
- **WHEN** a client issues `tools/list` after this change ships
- **THEN** every tool present before this change SHALL still appear in the response
- **AND** no tool's input schema SHALL have changed

### Requirement: Authentication and error mapping

Resource handlers SHALL use the existing `pkg/forgejo` singleton client for all upstream calls. On HTTP `403` from Forgejo the handler SHALL return MCP error code `-32002`; on HTTP `404` it SHALL return code `-32003`. Each error SHALL include a human-readable message identifying the requested URI and the upstream status.

#### Scenario: Forbidden upstream maps to -32002
- **WHEN** a resource handler receives HTTP `403` from Forgejo while resolving a URI
- **THEN** the MCP error returned SHALL have code `-32002`
- **AND** the error message SHALL include the requested URI

#### Scenario: Not-found upstream maps to -32003
- **WHEN** a resource handler receives HTTP `404` from Forgejo while resolving a URI
- **THEN** the MCP error returned SHALL have code `-32003`
- **AND** the error message SHALL include the requested URI
