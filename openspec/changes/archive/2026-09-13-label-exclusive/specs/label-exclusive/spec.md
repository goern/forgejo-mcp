# SPDX-License-Identifier: GPL-3.0-or-later

## Purpose

Lets MCP create, edit, and read Forgejo scoped-exclusive and archived labels so a taxonomy setup can set Exclusive and see the flag come back, instead of writing names the UI treats as decorations.

## ADDED Requirements

### Requirement: Create accepts exclusive and is_archived

The `create_repo_label` and `create_org_label` tools SHALL accept optional booleans `exclusive` and `is_archived`. When `exclusive` is true the JSON body SHALL contain `"exclusive":true`. When `is_archived` is true the JSON body SHALL contain `"is_archived":true`. When a flag is omitted the JSON body SHALL NOT contain that key.

#### Scenario: Create scoped exclusive label

- **WHEN** the caller invokes `create_repo_label` with `name` `"kind/bug"` and `exclusive` true
- **THEN** the system SHALL POST a body containing `"exclusive":true`

#### Scenario: Create omits exclusive

- **WHEN** the caller invokes `create_repo_label` without `exclusive`
- **THEN** the POST body SHALL NOT contain the key `exclusive`

#### Scenario: Org create shares the contract

- **WHEN** the caller invokes `create_org_label` with `name` `"kind/bug"` and `exclusive` true
- **THEN** the system SHALL POST a body containing `"exclusive":true`

### Requirement: Exclusive true requires a scoped name

A label name is scoped when it contains `/` not at either end. When `exclusive` is true on create, or on edit when a new `name` is supplied, and the name is not scoped, the system SHALL return an error and SHALL NOT send an HTTP request.

#### Scenario: Exclusive create without slash

- **WHEN** the caller invokes `create_repo_label` with `exclusive` true and `name` `"bug"`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Exclusive create with slash only at an end

- **WHEN** the caller invokes `create_repo_label` with `exclusive` true and `name` `"/bug"` or `"kind/"`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Exclusive-only edit does not re-fetch the name

- **WHEN** the caller invokes `edit_repo_label` with `exclusive` true and does not supply `name`
- **THEN** the system SHALL PATCH without first GET-ing the label
- **AND** the system SHALL NOT reject the call for lack of a scoped name

### Requirement: Edit PATCH omitempty for exclusive and is_archived

The `edit_repo_label` and `edit_org_label` tools SHALL treat `exclusive` and `is_archived` as optional PATCH fields. An omitted key SHALL be absent from the body. An explicit `false` SHALL appear on the wire. Exclusive-only and archived-only edits are valid. Supplying none of `name`, `color`, `description`, `exclusive`, `is_archived` SHALL be an error with no HTTP request.

#### Scenario: Color-only edit omits the flags

- **WHEN** the caller invokes `edit_repo_label` with only `color`
- **THEN** the PATCH body SHALL contain `color`
- **AND** the PATCH body SHALL NOT contain `exclusive` or `is_archived`

#### Scenario: Exclusive false is sent

- **WHEN** the caller invokes `edit_repo_label` with `exclusive` set to boolean false
- **THEN** the PATCH body SHALL contain `"exclusive":false`

#### Scenario: Archived false is sent

- **WHEN** the caller invokes `edit_repo_label` with `is_archived` set to boolean false
- **THEN** the PATCH body SHALL contain `"is_archived":false`

#### Scenario: Exclusive-only edit is accepted

- **WHEN** the caller invokes `edit_repo_label` with only `exclusive` true
- **THEN** the system SHALL PATCH
- **AND** the PATCH body SHALL contain `"exclusive":true`

#### Scenario: Archived-only edit is accepted

- **WHEN** the caller invokes `edit_repo_label` with only `is_archived` true
- **THEN** the system SHALL PATCH
- **AND** the PATCH body SHALL contain `"is_archived":true`

#### Scenario: Empty edit is rejected

- **WHEN** the caller invokes `edit_repo_label` with only `owner`, `repo`, and `id`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Org edit shares the contract

- **WHEN** the caller invokes `edit_org_label` with only `exclusive` false
- **THEN** the PATCH body SHALL contain `"exclusive":false`

### Requirement: Reads return exclusive and is_archived

`get_repo_label`, `get_org_label`, `list_repo_labels`, `list_org_labels`, `forgejo://repo/{owner}/{repo}/label/{id}`, `forgejo://repo/{owner}/{repo}/labels{?page,limit}`, and `forgejo://org/{org}/labels{?page,limit}` SHALL include `exclusive` and `is_archived` as JSON booleans. A false value SHALL be present as `false`, not omitted.

#### Scenario: Get includes both flags

- **WHEN** the caller invokes `get_repo_label` and the upstream label has exclusive true and is_archived false
- **THEN** the result SHALL contain `"exclusive":true`
- **AND** the result SHALL contain `"is_archived":false`

#### Scenario: List includes both flags

- **WHEN** the caller invokes `list_repo_labels` and a row is exclusive
- **THEN** that row SHALL contain `"exclusive":true`

#### Scenario: Single-label resource includes both flags

- **WHEN** the caller reads `forgejo://repo/{owner}/{repo}/label/{id}`
- **THEN** the JSON block SHALL contain `exclusive` and `is_archived` as booleans

#### Scenario: List resources include both flags

- **WHEN** the caller reads `forgejo://repo/{owner}/{repo}/labels` or `forgejo://org/{org}/labels`
- **THEN** each label object SHALL contain `exclusive` and `is_archived` as booleans

### Requirement: Path segments are escaped

Create, edit, get, and list paths SHALL escape every segment so an `owner` or `org` containing `/` or `?` cannot retarget the request at another endpoint.

#### Scenario: Owner slash does not retarget

- **WHEN** the caller invokes `get_repo_label` with `owner` `"acme/org"`
- **THEN** the request path SHALL contain `acme%2Forg` as a single segment
