<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# packages Specification

## Purpose

List package versions for a user or org, get or delete one version, and list
that version's files through `list_packages`, `get_package`,
`delete_package`, and `list_package_files`. List is Forgejo `SearchVersions`
(one row per version) and is server-paged. Files are client-sliced because
the files endpoint has no page query. Delete removes one version, not every
version of a name.

## Requirements

### Requirement: List package versions of an owner

The `list_packages` tool SHALL accept required `owner` and optional `type`, `q`, `page` (default 1), and `limit` (default 30, maximum 50), GET `/packages/{owner}` with those query parameters (omitting empty `type` and `q`), and return a JSON object with keys `packages`, `page`, `limit`, `count`, and `has_next`. Each element of `packages` SHALL be one package **version**. The system SHALL set `has_next` from a `Link` header `rel="next"` (quoted or unquoted) and SHALL include `total_count` only when the response carries a parsable `X-Total-Count` header. A 404 SHALL be an error, not an empty list. Each version row SHALL contain `id`, `type`, `name`, `version` and MAY contain `html_url`, `created_at`, and `repository` as the repository `full_name`. The row SHALL NOT embed Forgejo `User` or `Repository` objects.

#### Scenario: List returns a bounding envelope

- **WHEN** the caller invokes `list_packages` with `owner`
- **THEN** the system SHALL GET `/packages/{owner}` with `page` and `limit` query parameters
- **AND** the system SHALL return an object containing `packages`, `page`, `limit`, `count`, and `has_next`

#### Scenario: Type and name filters are forwarded

- **WHEN** the caller invokes `list_packages` with `type` set to `"container"` and `q` set to `"core"`
- **THEN** the GET query SHALL include `type=container` and `q=core`

#### Scenario: Total count comes from the header

- **WHEN** the upstream lists packages and sets `X-Total-Count` to `4`
- **THEN** the returned object SHALL include `"total_count": 4`

#### Scenario: Missing total header omits the key

- **WHEN** the upstream lists packages without `X-Total-Count`
- **THEN** the returned object SHALL omit `total_count`

#### Scenario: Link rel=next sets has_next

- **WHEN** the upstream lists packages and sends `Link` with `rel="next"`
- **THEN** the returned object SHALL include `"has_next": true`

#### Scenario: Missing Link leaves has_next false

- **WHEN** the upstream lists packages without `Link` `rel="next"`
- **THEN** the returned object SHALL include `"has_next": false`

#### Scenario: Missing owner is an error

- **WHEN** the upstream responds 404 for the owner
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT return an empty `packages` array as success

#### Scenario: JSON null body is an empty page

- **WHEN** the upstream body is JSON `null`
- **THEN** the system SHALL return success with `packages` equal to an empty array
- **AND** the system SHALL NOT return a JSON `null` for `packages`

### Requirement: Get one package version

The `get_package` tool SHALL accept required `owner`, `type`, `name`, and `version`, GET `/packages/{owner}/{type}/{name}/{version}`, and return the same projected fields as one list row. Path segments SHALL be escaped so a `name` containing `/` does not retarget the request.

#### Scenario: Get returns a projected version

- **WHEN** the caller invokes `get_package` with a valid version
- **THEN** the system SHALL GET `/packages/{owner}/{type}/{name}/{version}`
- **AND** the result SHALL include `id`, `type`, `name`, `version`
- **AND** the result SHALL NOT embed a Forgejo `User` object

#### Scenario: Slash in the package name is escaped

- **WHEN** the caller invokes `get_package` with `name` `"youscore/core"`
- **THEN** the request path SHALL contain `youscore%2Fcore` as a single segment

### Requirement: Delete one package version

The `delete_package` tool SHALL accept required `owner`, `type`, `name`, and `version`, DELETE `/packages/{owner}/{type}/{name}/{version}` without a prior GET, and SHALL surface any 4xx or 5xx as an MCP error without mapping it to success.

#### Scenario: Delete of an existing version succeeds

- **WHEN** the caller invokes `delete_package` for an existing version
- **AND** the upstream responds 204
- **THEN** the system SHALL DELETE `/packages/{owner}/{type}/{name}/{version}`
- **AND** the system SHALL return an object containing `owner`, `type`, `name`, `version`, and `status` `"deleted"`

#### Scenario: Delete of a missing version is an error

- **WHEN** the caller invokes `delete_package` and the upstream responds 404
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"deleted"`
- **AND** the system SHALL still have sent the DELETE

### Requirement: List files of a package version

The `list_package_files` tool SHALL accept required `owner`, `type`, `name`, and `version` and optional `page` (default 1) and `limit` (default 30, maximum 50), GET `/packages/{owner}/{type}/{name}/{version}/files`, slice the returned array client-side, and return `{files, page, limit, count, has_next, total_count}`. The system SHALL set `total_count` to the length of the fetched list. Each file row SHALL contain `id`, `name`, `size` and MAY contain `sha256`. The row SHALL NOT include `md5`, `sha1`, or `sha512`.

#### Scenario: Files are sliced with has_next

- **WHEN** the upstream returns two files and the caller sets `limit` to `1` and `page` to `1`
- **THEN** the system SHALL return one file, `count` 1, `has_next` true, and `total_count` 2

#### Scenario: Second page

- **WHEN** the upstream returns two files and the caller sets `limit` to `1` and `page` to `2`
- **THEN** the system SHALL return the second file, `has_next` false, and `total_count` 2

### Requirement: Missing required arguments never reach the network

The system SHALL reject a call that lacks `owner` or, for get/delete/files, `type`, `name`, or `version` before any HTTP request.

#### Scenario: Missing owner does not GET

- **WHEN** the caller invokes `get_package` without `owner`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: Empty type does not GET

- **WHEN** the caller invokes `get_package` with an empty `type`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

### Requirement: Path segments are escaped

The system SHALL build every API path with `forgejo.APIPath` so an `owner` or package `name` containing `/` cannot retarget the request.

#### Scenario: Owner with a slash stays one segment

- **WHEN** the caller invokes `list_packages` with `owner` `"o/x"`
- **THEN** the request path SHALL contain `o%2Fx` as a single owner segment
