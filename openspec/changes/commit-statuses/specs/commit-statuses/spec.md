# SPDX-License-Identifier: GPL-3.0-or-later

## ADDED Requirements

### Requirement: List commit statuses for a SHA

The `get_commit_statuses` tool SHALL accept required `owner`, `repo`, and `sha`, and optional `page` (default 1) and `limit` (default 30, maximum 50), call `Client.ListStatuses`, and return a JSON object with keys `sha`, `statuses`, `page`, `limit`, and `count`. Each status item SHALL use keys `context`, `state`, `target_url`, `description`, and `created_at`. The system SHALL NOT compute a combined aggregate state.

#### Scenario: List returns a bounding envelope

- **WHEN** the caller invokes `get_commit_statuses` with `owner`, `repo`, and a valid `sha`
- **THEN** the system SHALL GET `/repos/{owner}/{repo}/commits/{sha}/statuses` with `page` and `limit` query parameters
- **AND** the system SHALL return an object containing `sha`, `statuses`, `page`, `limit`, and `count`
- **AND** each element of `statuses` SHALL include `state` (not `status`)

#### Scenario: Empty list is an empty array

- **WHEN** the upstream returns an empty status list
- **THEN** the system SHALL return `count` 0
- **AND** the system SHALL return `statuses` as an empty array, not JSON null

#### Scenario: Page is forwarded

- **WHEN** the caller invokes `get_commit_statuses` with `page` set to 2
- **THEN** the GET query SHALL include `page=2`

#### Scenario: Missing commit is an error

- **WHEN** the upstream responds 404
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT return an empty `statuses` array as success

### Requirement: SHA is a full 40-character hex

The system SHALL reject a `sha` that is not exactly 40 hexadecimal characters before any HTTP request, using the same rule as the commit status resource (`resource.ValidateSHA`).

#### Scenario: Short SHA does not request

- **WHEN** the caller invokes `get_commit_statuses` with `sha` set to `"abc123"`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request
