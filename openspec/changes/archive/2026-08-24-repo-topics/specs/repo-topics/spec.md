# SPDX-License-Identifier: GPL-3.0-or-later

## ADDED Requirements

### Requirement: List repository topics

The `list_repo_topics` tool SHALL accept required `owner` and `repo` and optional `page` (default 1) and `limit` (default 100), call `Client.ListRepoTopics`, and return a JSON object with keys `topics`, `page`, `limit`, and `count`.

#### Scenario: List returns an envelope

- **WHEN** the caller invokes `list_repo_topics` with `owner` and `repo`
- **THEN** the system SHALL GET `/repos/{owner}/{repo}/topics`
- **AND** the system SHALL return an object containing `topics`, `page`, `limit`, and `count`

### Requirement: Replace all repository topics

The `set_repo_topics` tool SHALL require a string argument `topics` (CSV), normalise each name, and call `Client.SetRepoTopics`; a missing `topics` key SHALL be an error with no upstream PUT.

#### Scenario: CSV is lowercased and sent as an array

- **WHEN** the caller invokes `set_repo_topics` with `topics` set to `"go, MCP"`
- **THEN** the PUT body SHALL be `{"topics":["go","mcp"]}`

#### Scenario: Empty CSV clears topics

- **WHEN** the caller invokes `set_repo_topics` with `topics` set to `""`
- **THEN** the PUT body SHALL be `{"topics":[]}`

#### Scenario: Missing topics key does not PUT

- **WHEN** the caller invokes `set_repo_topics` without a `topics` key
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send a PUT

### Requirement: Add and delete a single topic

The `add_repo_topic` and `delete_repo_topic` tools SHALL accept required `owner`, `repo`, and `topic`, normalise the name, and call `Client.AddRepoTopic` or `Client.DeleteRepoTopic`.

#### Scenario: Add one topic

- **WHEN** the caller invokes `add_repo_topic` with `topic` set to `"ci"`
- **THEN** the system SHALL PUT `/repos/{owner}/{repo}/topics/ci`

#### Scenario: Delete one topic

- **WHEN** the caller invokes `delete_repo_topic` with `topic` set to `"ci"`
- **THEN** the system SHALL DELETE `/repos/{owner}/{repo}/topics/ci`

### Requirement: Invalid topic names never reach the network

The system SHALL reject a topic that fails Forgejo's name rules (`^[a-z0-9][-.a-z0-9]*$`, at most 35 characters) or a `set_repo_topics` list longer than 25 after normalisation, before any SDK call.

#### Scenario: Invalid name is rejected

- **WHEN** the caller invokes `add_repo_topic` with `topic` set to `"Bad Name"`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

#### Scenario: More than 25 topics is rejected

- **WHEN** the caller invokes `set_repo_topics` with 26 distinct valid names
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send a PUT

### Requirement: get_repo includes topics

After this change, `get_repo` SHALL call `ListRepoTopics` in addition to `GetRepo` and SHALL include a `topics` array in the returned JSON.

#### Scenario: get_repo JSON contains topics

- **WHEN** the caller invokes `get_repo` for a repository that has topics
- **THEN** the system SHALL request both the repository and `/topics`
- **AND** the returned JSON SHALL contain a `topics` array
