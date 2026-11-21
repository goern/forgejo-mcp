<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# action-variables Specification

## Purpose

List, get, create, update, and delete Forgejo Actions variables for a user, an organization, or a repository through one `scope` argument. A list page does not carry values. Create is POST and update is PUT. Forgejo has no REST API for an instance-global variable, so these tools do not return one.

## Requirements

### Requirement: Scope coordinates

The system SHALL require `scope` of `user`, `org`, or `repo` on `list_action_variables`, `get_action_variable`, `create_action_variable`, `update_action_variable`, and `delete_action_variable`. Scope `org` SHALL require `org`. Scope `repo` SHALL require `owner` and `repo`. Scope `user` SHALL reject a non-empty `org`, `owner`, or `repo`. Scope `org` SHALL reject a non-empty `owner` or `repo`. Scope `repo` SHALL reject a non-empty `org`. An unknown or empty `scope` SHALL be an error. The system SHALL return these errors before any HTTP request.

#### Scenario: User scope rejects owner

- **WHEN** the caller invokes `list_action_variables` with `scope` `user` and `owner` set
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Org scope requires org

- **WHEN** the caller invokes `list_action_variables` with `scope` `org` and no `org`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Unknown scope

- **WHEN** the caller invokes `list_action_variables` with `scope` `instance`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

### Requirement: List action variables

The `list_action_variables` tool SHALL accept `page` (default 1, minimum 1) and `limit` (default 30, maximum 50) and SHALL call the SDK list method for the scope. Each row SHALL contain `name`, `owner_id`, and `repo_id` and SHALL NOT contain `data`. The envelope SHALL contain `scope`, `variables`, `page`, `limit`, and `count`. The system SHALL include `total_count` only when the response carries a parsable `X-Total-Count` header.

#### Scenario: List omits values

- **WHEN** the caller invokes `list_action_variables` with `scope` `user`
- **AND** the upstream row includes `data`
- **THEN** the system SHALL return `name`, `owner_id`, and `repo_id`
- **AND** the system SHALL NOT return `data`

#### Scenario: List omits total_count without the header

- **WHEN** the caller invokes `list_action_variables` and the response has no `X-Total-Count` header
- **THEN** the system SHALL omit `total_count`

### Requirement: Get one action variable

The `get_action_variable` tool SHALL require `name` and SHALL return `name`, `data`, `owner_id`, and `repo_id`. A missing name SHALL be an error, not an empty object.

#### Scenario: Get returns the value

- **WHEN** the caller invokes `get_action_variable` with `scope` `repo` and a name the upstream knows
- **THEN** the system SHALL return `data`

#### Scenario: Missing name is an error

- **WHEN** the caller invokes `get_action_variable` and the upstream responds 404
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT return an empty object

### Requirement: Create and update are different methods

The `create_action_variable` tool SHALL POST the variable. The `update_action_variable` tool SHALL PUT the variable. The argument `data` SHALL be sent as JSON `value`. The tool result SHALL NOT echo `data`. Empty `data` SHALL be rejected by SDK validation before any HTTP request. A `name` containing `/`, `?`, or `#` SHALL be rejected before any HTTP request.

#### Scenario: Empty data sends no request

- **WHEN** the caller invokes `create_action_variable` with empty `data`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Slash in the name sends no request

- **WHEN** the caller invokes `create_action_variable` with `name` `a/b`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Create posts and does not echo

- **WHEN** the caller invokes `create_action_variable` with `scope` `repo` and a value
- **THEN** the system SHALL POST `/repos/{owner}/{repo}/actions/variables/{name}` with JSON `value`
- **AND** the result SHALL contain `status` `created`
- **AND** the result SHALL NOT contain the value

#### Scenario: Update puts

- **WHEN** the caller invokes `update_action_variable` with `scope` `user`
- **THEN** the system SHALL PUT `/user/actions/variables/{name}`
- **AND** the result SHALL contain `status` `updated`

### Requirement: Delete an action variable

The `delete_action_variable` tool SHALL DELETE the named variable and SHALL return `status` `deleted` on success.

#### Scenario: Delete succeeds

- **WHEN** the caller invokes `delete_action_variable` with `scope` `org` and the upstream responds 204
- **THEN** the system SHALL return `status` `deleted`

### Requirement: No instance-global variable

The system SHALL NOT call an instance-global variable route. Tool descriptions and the README SHALL state that Forgejo has no REST API for that level.

#### Scenario: Tools do not invent a global scope

- **WHEN** the caller passes `scope` `instance`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request
