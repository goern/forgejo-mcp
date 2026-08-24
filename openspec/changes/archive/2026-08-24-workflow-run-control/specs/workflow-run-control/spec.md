# SPDX-License-Identifier: GPL-3.0-or-later

## ADDED Requirements

### Requirement: Cancel a workflow run

The `cancel_workflow_run` tool SHALL accept required `owner`, `repo`, and `run_id`, POST `/repos/{owner}/{repo}/actions/runs/{run_id}/cancel` with no body, and treat HTTP 204 as success whether the run was pending, running, or already finished.

#### Scenario: Cancel of a running run succeeds

- **WHEN** the caller invokes `cancel_workflow_run` for a running run
- **THEN** the system SHALL POST `/repos/{owner}/{repo}/actions/runs/{run_id}/cancel`
- **AND** the system SHALL return an object containing `run_id` and `status` `"cancelled"`

#### Scenario: Cancel of an already-finished run is success

- **WHEN** the caller invokes `cancel_workflow_run` for a run that has already finished
- **AND** the upstream responds 204
- **THEN** the system SHALL return success with `status` `"cancelled"`
- **AND** the system SHALL NOT treat the 204 as an error

### Requirement: Delete a completed workflow run

The `delete_workflow_run` tool SHALL accept required `owner`, `repo`, and `run_id`, DELETE `/repos/{owner}/{repo}/actions/runs/{run_id}` without a prior GET, and SHALL surface any 4xx or 5xx as an MCP error without mapping it to success.

#### Scenario: Delete of a completed run succeeds

- **WHEN** the caller invokes `delete_workflow_run` for a completed run
- **AND** the upstream responds 204
- **THEN** the system SHALL DELETE `/repos/{owner}/{repo}/actions/runs/{run_id}`
- **AND** the system SHALL return an object containing `run_id` and `status` `"deleted"`

#### Scenario: Delete of a live run is an error

- **WHEN** the caller invokes `delete_workflow_run` for a run that has not completed
- **AND** the upstream responds 4xx or 5xx
- **THEN** the system SHALL return an MCP error
- **AND** the system SHALL NOT return `status` `"deleted"`
- **AND** the system SHALL still have sent the DELETE

### Requirement: List artifacts of a workflow run

The `list_action_run_artifacts` tool SHALL accept required `owner`, `repo`, and `run_id` and optional `page` (default 1), `limit` (default 30, maximum 50), and `name`, GET `/repos/{owner}/{repo}/actions/runs/{run_id}/artifacts` with those query parameters, and return a JSON object with keys `run_id`, `artifacts`, `page`, `limit`, and `count`. The system SHALL include `total_count` only when the response carries a parsable `X-Total-Count` header. A 404 SHALL be an error, not an empty list.

#### Scenario: List returns a bounding envelope

- **WHEN** the caller invokes `list_action_run_artifacts` with `owner`, `repo`, and `run_id`
- **THEN** the system SHALL GET `/repos/{owner}/{repo}/actions/runs/{run_id}/artifacts` with `page` and `limit` query parameters
- **AND** the system SHALL return an object containing `run_id`, `artifacts`, `page`, `limit`, and `count`

#### Scenario: Name filter is forwarded

- **WHEN** the caller invokes `list_action_run_artifacts` with `name` set to `"dist"`
- **THEN** the GET query SHALL include `name=dist`

#### Scenario: Total count comes from the header

- **WHEN** the upstream lists artifacts and sets `X-Total-Count` to `4`
- **THEN** the returned object SHALL include `"total_count": 4`

#### Scenario: Missing total header omits the key

- **WHEN** the upstream lists artifacts without `X-Total-Count`
- **THEN** the returned object SHALL omit `total_count`

#### Scenario: Missing run is an error

- **WHEN** the upstream responds 404 for the run
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT return an empty `artifacts` array as success

### Requirement: Get artifact metadata

The `get_action_artifact` tool SHALL accept required `owner`, `repo`, and `artifact_id`, GET `/repos/{owner}/{repo}/actions/artifacts/{artifact_id}`, and return that artifact's metadata. The system SHALL NOT GET `/repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip`.

#### Scenario: Get returns metadata fields

- **WHEN** the caller invokes `get_action_artifact` with a valid `artifact_id`
- **THEN** the system SHALL GET `/repos/{owner}/{repo}/actions/artifacts/{artifact_id}`
- **AND** the result SHALL include `id`, `name`, `run_id`, `size_in_bytes`, `expired`, `expires_at`, `created_at`, `updated_at`, and `archive_download_url`
- **AND** the request path SHALL NOT end in `/zip`

### Requirement: Missing required arguments never reach the network

The system SHALL reject a call that lacks `owner`, `repo`, or the relevant id (`run_id` or `artifact_id`) before any HTTP request.

#### Scenario: Missing run_id does not POST

- **WHEN** the caller invokes `cancel_workflow_run` without `run_id`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send an HTTP request

### Requirement: Path segments are escaped

The system SHALL build every API path with `forgejo.APIPath` so an `owner` or `repo` containing `/` cannot retarget the request.

#### Scenario: Owner with a slash stays one segment

- **WHEN** the caller invokes `cancel_workflow_run` with `owner` `"o/x"`
- **THEN** the request path SHALL contain `o%2Fx` as a single owner segment
