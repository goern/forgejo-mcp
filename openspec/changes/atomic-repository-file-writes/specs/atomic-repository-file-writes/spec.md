## ADDED Requirements

### Requirement: Atomic multi-file repository write tool

The server SHALL register an MCP tool `change_files` accepting `owner`, `repo`, `message`, `branch_name`, optional `new_branch_name`, and a non-empty `files` array. The server SHALL validate the complete batch before making an upstream write and SHALL submit a valid batch to Forgejo's native repository contents endpoint in one request.

Each file operation SHALL contain `operation` and `path`. Supported operations SHALL be `create`, `update`, and `delete`. Multiple operations targeting the same path SHALL be rejected before the upstream request.

#### Scenario: Mixed batch is committed through one native request

- **WHEN** a client calls `change_files` with valid create, update, and delete operations
- **THEN** the server SHALL submit all operations to Forgejo in one repository contents request
- **AND** SHALL NOT emulate the batch through multiple single-file writes

#### Scenario: Empty batch is rejected

- **WHEN** a client calls `change_files` with an empty `files` array
- **THEN** the server SHALL reject the request before calling Forgejo

#### Scenario: Duplicate target paths are rejected

- **WHEN** two operations in one `change_files` request target the same repository path
- **THEN** the server SHALL reject the complete batch before calling Forgejo

### Requirement: Operation-specific SHA validation

A `create` operation SHALL omit `sha`. An `update` or `delete` operation SHALL provide a non-empty current file `sha`. Invalid SHA presence SHALL cause the complete batch to be rejected before the upstream request.

#### Scenario: Create with SHA is rejected

- **WHEN** a `create` operation supplies `sha`
- **THEN** `change_files` SHALL reject the complete batch before calling Forgejo

#### Scenario: Update without SHA is rejected

- **WHEN** an `update` operation omits `sha` or supplies an empty SHA
- **THEN** `change_files` SHALL reject the complete batch before calling Forgejo

#### Scenario: Delete without SHA is rejected

- **WHEN** a `delete` operation omits `sha` or supplies an empty SHA
- **THEN** `change_files` SHALL reject the complete batch before calling Forgejo

### Requirement: Binary-safe repository write content

`create_file`, `update_file`, and create/update operations in `change_files` SHALL accept exactly one of `content` or `content_base64`.

`content` SHALL represent plain text and SHALL be encoded as standard Base64 before it is sent to Forgejo.

`content_base64` SHALL represent the exact desired file bytes as strict RFC 4648 standard Base64. The server SHALL validate it and SHALL forward the validated Base64 string unchanged.

Argument selection SHALL be based on field presence. An explicitly supplied empty `content` or empty `content_base64` SHALL therefore be valid and SHALL represent a zero-byte file.

#### Scenario: Existing plain-text write behavior is preserved

- **WHEN** a client supplies `content="hello"` to a repository create/update operation
- **THEN** the server SHALL send `aGVsbG8=` as the Forgejo file content

#### Scenario: Binary content is forwarded without text conversion

- **WHEN** a client supplies valid `content_base64="AP8="`
- **THEN** the server SHALL send `AP8=` unchanged as the Forgejo file content

#### Scenario: Invalid Base64 is rejected

- **WHEN** a client supplies `content_base64` that is not strict standard Base64
- **THEN** the server SHALL reject the request before calling Forgejo

#### Scenario: Missing content representation is rejected

- **WHEN** a create/update operation supplies neither `content` nor `content_base64`
- **THEN** the server SHALL reject the request before calling Forgejo

#### Scenario: Ambiguous content representation is rejected

- **WHEN** a create/update operation supplies both `content` and `content_base64`
- **THEN** the server SHALL reject the request before calling Forgejo

#### Scenario: Zero-byte file is representable

- **WHEN** a create/update operation explicitly supplies `content=""` or `content_base64=""`
- **THEN** the server SHALL accept the representation
- **AND** SHALL send an empty Base64 payload to Forgejo

### Requirement: Delete operations carry no content

A `delete` operation in `change_files` SHALL reject both `content` and `content_base64`, including explicitly empty values.

#### Scenario: Delete with content is rejected

- **WHEN** a delete operation contains `content` or `content_base64`
- **THEN** `change_files` SHALL reject the complete batch before calling Forgejo

### Requirement: Existing single-file tools remain available

The server SHALL continue to register `create_file`, `update_file`, and `delete_file`. Existing clients supplying plain-text `content` to `create_file` or `update_file` SHALL retain their previous file-content behavior.

#### Scenario: Existing plain-text caller remains compatible

- **WHEN** an existing client calls `create_file` or `update_file` with the previously supported plain-text `content` argument
- **THEN** the resulting repository file bytes SHALL be unchanged from the behavior before this capability was added

### Requirement: File-write capabilities are documented

The README file-tool table SHALL list `change_files` and SHALL state that `create_file` and `update_file` accept either plain-text `content` or binary-safe `content_base64`.

#### Scenario: README exposes atomic and binary-safe writes

- **WHEN** a user reads the README file-tool table
- **THEN** `change_files` SHALL be listed
- **AND** the `create_file` and `update_file` rows SHALL describe `content_base64`
