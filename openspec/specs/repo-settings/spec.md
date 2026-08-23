<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# repo-settings Specification

## Purpose

Read one repository and PATCH its flat settings page — metadata, visibility,
archive flag, and unit toggles — through `get_repo` and `edit_repo`. Only
caller-supplied fields are sent; an edit with no fields is an error.

## Requirements

### Requirement: Get a single repository

The `get_repo` tool SHALL accept required string arguments `owner` and `repo` and SHALL return the Forgejo repository object from `Client.GetRepo`.

#### Scenario: Repository exists

- **WHEN** the caller invokes `get_repo` with a valid `owner` and `repo`
- **THEN** the system SHALL return the repository JSON including `description` and `website`

#### Scenario: Repository not found

- **WHEN** the caller invokes `get_repo` with an unknown `owner`/`repo`
- **THEN** the system SHALL return an MCP error wrapping the SDK error
- **AND** the system SHALL NOT invent a repository object

### Requirement: Edit repository settings with PATCH semantics

The `edit_repo` tool SHALL accept required string arguments `owner` and `repo` and the optional arguments `name`, `description`, `website`, `default_branch` (strings) and `private`, `template`, `archived`, `has_issues`, `has_wiki`, `has_pull_requests`, `has_projects`, `has_releases`, `has_packages`, `has_actions` (booleans), and SHALL send only caller-supplied fields to `Client.EditRepo`.

#### Scenario: Description-only update does not send private

- **WHEN** the caller invokes `edit_repo` with only `owner`, `repo`, and `description`
- **THEN** the PATCH body SHALL contain `description`
- **AND** the PATCH body SHALL NOT contain a concrete boolean for `private` or `archived`

#### Scenario: Explicit private false is sent

- **WHEN** the caller invokes `edit_repo` with `private` set to boolean `false`
- **THEN** the PATCH body SHALL contain `"private": false`

#### Scenario: Empty description clears the field

- **WHEN** the caller invokes `edit_repo` with `description` set to the empty string
- **THEN** the PATCH body SHALL contain `"description": ""`

#### Scenario: No optional fields is rejected

- **WHEN** the caller invokes `edit_repo` with only `owner` and `repo`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT call `EditRepo`

### Requirement: Discoverable in tool index and docs

The `get_repo` and `edit_repo` tools SHALL appear in the README Repositories table and in `extension/manifest.json`.

#### Scenario: README lists the tools

- **WHEN** a reader opens the README Available Tools table
- **THEN** the Repositories section SHALL include `get_repo` and `edit_repo`
