# SPDX-License-Identifier: GPL-3.0-or-later

## ADDED Requirements

### Requirement: Link a package to a repository

The `link_package` tool SHALL accept required `owner`, `type`, `name`, and `repo`, and SHALL NOT accept `version`. `repo` SHALL be the repository name under the same owner. The system SHALL POST `/packages/{owner}/{type}/{name}/-/link/{repo_name}` with no body. HTTP 201 SHALL return `{owner, type, name, repo, status: "linked"}`. The system SHALL NOT decode a Forgejo response body.

#### Scenario: Link succeeds

- **WHEN** the caller invokes `link_package` with a same-owner repository name
- **AND** the upstream responds 201
- **THEN** the system SHALL POST `/packages/{owner}/{type}/{name}/-/link/{repo_name}`
- **AND** the system SHALL return an object containing `owner`, `type`, `name`, `repo`, and `status` `"linked"`

#### Scenario: Link of a missing package is an error

- **WHEN** the caller invokes `link_package` and the upstream responds 404
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"linked"`

#### Scenario: Link of a missing repository is an error

- **WHEN** the caller invokes `link_package` and the upstream responds 404 for the repository name
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"linked"`

#### Scenario: Link without write access is an error

- **WHEN** the caller invokes `link_package` and the upstream responds 403
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"linked"`

#### Scenario: Already-linked is an error

- **WHEN** the caller invokes `link_package` for a name that is already linked
- **AND** the upstream responds 400
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"linked"`
- **AND** the system SHALL NOT invent a status `"already_linked"`

### Requirement: Unlink a package from a repository

The `unlink_package` tool SHALL accept required `owner`, `type`, and `name`, and SHALL NOT accept `version`. The system SHALL POST `/packages/{owner}/{type}/{name}/-/unlink` with no body. HTTP 204 SHALL return `{owner, type, name, status: "unlinked"}`. The system SHALL NOT decode a Forgejo response body.

#### Scenario: Unlink succeeds

- **WHEN** the caller invokes `unlink_package` for a linked name
- **AND** the upstream responds 204
- **THEN** the system SHALL POST `/packages/{owner}/{type}/{name}/-/unlink`
- **AND** the system SHALL return an object containing `owner`, `type`, `name`, and `status` `"unlinked"`

#### Scenario: Unlink of a missing package is an error

- **WHEN** the caller invokes `unlink_package` and the upstream responds 404
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"unlinked"`

#### Scenario: Unlink when not linked is an error

- **WHEN** the caller invokes `unlink_package` for a name that is not linked
- **AND** the upstream responds 400
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"unlinked"`

### Requirement: Slash in repo is rejected with no request

A `repo` argument containing `/` SHALL be an error before any HTTP request. Agents pass `owner/repo`; Forgejo would look up a repository whose name is that string and 404.

#### Scenario: owner/repo as repo sends no HTTP

- **WHEN** the caller invokes `link_package` with `repo` `"cnvr/oss"`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

### Requirement: Link and unlink missing arguments never reach the network

The system SHALL reject a `link_package` call that lacks `owner`, `type`, `name`, or `repo`, and an `unlink_package` call that lacks `owner`, `type`, or `name`, before any HTTP request.

#### Scenario: Missing repo does not POST

- **WHEN** the caller invokes `link_package` without `repo`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Missing name on unlink does not POST

- **WHEN** the caller invokes `unlink_package` without `name`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

### Requirement: Link path segments are escaped

Link and unlink paths SHALL escape every segment so an `owner` or package `name` containing `/` cannot retarget the request. The `/-/` in the Forgejo path SHALL be the path segment `"-"`.

#### Scenario: Slash in the package name is one link segment

- **WHEN** the caller invokes `link_package` with `name` `"youscore/core"`
- **THEN** the request path SHALL contain `youscore%2Fcore` as a single segment
- **AND** the request path SHALL contain `/-/link/` as the link suffix

#### Scenario: Owner with a slash stays one segment

- **WHEN** the caller invokes `link_package` with `owner` `"acme/org"`
- **THEN** the request path SHALL contain `acme%2Forg` as a single owner segment

### Requirement: After link, get_package shows repository

After a successful link, `get_package` on a version of that name SHALL include `repository` as the repository `full_name`. No new get tool.

#### Scenario: Get after link includes repository

- **WHEN** `link_package` has succeeded for a name
- **AND** the caller invokes `get_package` for a version of that name
- **AND** the upstream get body contains `repository.full_name`
- **THEN** the result SHALL contain `repository` as that `full_name` string
