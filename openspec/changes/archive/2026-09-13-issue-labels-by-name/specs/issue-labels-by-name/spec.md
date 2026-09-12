# SPDX-License-Identifier: GPL-3.0-or-later

## ADDED Requirements

### Requirement: Resolve label names and ids to label ids

The system SHALL resolve every token of a label CSV to a numeric label id before any upstream write, matching a token first against label names and only then, if no name matches and the token parses as an integer and a label with that id exists, against label ids; duplicate tokens SHALL collapse to a single id.

#### Scenario: A repository label name resolves to its id

- **WHEN** the caller names a label that exists in the repository
- **THEN** the system SHALL send that label's numeric id upstream

#### Scenario: An organization label name resolves to its id

- **WHEN** the caller names a label that exists only at the organization level of an org-owned repository
- **THEN** the system SHALL send that label's numeric id upstream

#### Scenario: A numeric token is still accepted

- **WHEN** the caller passes a numeric token that matches no label name and is the id of a label in the catalogue
- **THEN** the system SHALL send that id upstream

#### Scenario: A name that looks like a number wins over the id

- **WHEN** a label is named `123` and the caller passes `123`
- **THEN** the system SHALL send the id of the label named `123`

#### Scenario: A label on a later catalogue page still resolves

- **WHEN** the named label appears only on the second page of the repository or organization label list
- **THEN** the system SHALL resolve it rather than report it unknown

### Requirement: Reject unresolvable labels before any write

The system SHALL return an error and send no create, add, remove, or replace request when a label token matches no label name and is not the id of a label in the catalogue, when a name matches in both the repository and the organization scope, or when a label argument is present but names nothing after trimming.

#### Scenario: An unknown name is an error, not a silent drop

- **WHEN** the caller names a label that does not exist
- **THEN** the system SHALL return an error naming the unresolved token
- **AND** the system SHALL NOT send the mutating request

#### Scenario: A name in two scopes is ambiguous

- **WHEN** a name matches both a repository label and an organization label
- **THEN** the system SHALL return an error reporting both scopes and both ids
- **AND** the system SHALL NOT send the mutating request

#### Scenario: A present but empty label argument is an error

- **WHEN** the caller passes `labels` as `""` to `create_issue`, `add_issue_labels`, or `remove_issue_labels`
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send the mutating request

#### Scenario: An unreadable organization label list fails the call

- **WHEN** the organization label request returns 401 or 403
- **THEN** the system SHALL return an error rather than resolve against repository labels alone

### Requirement: Create an issue with labels, assignees, and a milestone

The `create_issue` tool SHALL accept optional `labels` (CSV of label names or ids), `assignees` (CSV of usernames), and `milestone` (numeric milestone id), resolve label names to ids before the request, and send them in the single `POST /repos/{owner}/{repo}/issues` call that creates the issue.

#### Scenario: Label names are sent as ids on creation

- **WHEN** the caller invokes `create_issue` with `labels` naming two existing labels
- **THEN** the POST body SHALL carry those labels as numeric ids
- **AND** the issue SHALL be created carrying both labels

#### Scenario: Assignees and milestone reach the create request

- **WHEN** the caller invokes `create_issue` with `assignees` set to `"alice, bob"` and `milestone` set to `"4"`
- **THEN** the POST body SHALL carry `assignees` as `["alice","bob"]` and `milestone` as `4`

#### Scenario: An unknown label name creates no issue

- **WHEN** the caller invokes `create_issue` with a label name that does not exist
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send the create request

#### Scenario: Creating without the new arguments reads no catalogue

- **WHEN** the caller invokes `create_issue` with only `owner`, `repo`, and `title`
- **THEN** the system SHALL send exactly one upstream request

#### Scenario: A non-numeric milestone is rejected

- **WHEN** the caller invokes `create_issue` with `milestone` set to a non-numeric string
- **THEN** the system SHALL return an error
- **AND** the system SHALL NOT send the create request

### Requirement: Add and remove issue labels by name

The `add_issue_labels` and `remove_issue_labels` tools SHALL accept label names as well as numeric ids in their `labels` CSV, and their descriptions SHALL state that both forms are accepted.

#### Scenario: Adding by name

- **WHEN** the caller invokes `add_issue_labels` with `labels` set to `"bug,triage"`
- **THEN** the system SHALL POST the two label ids to `/repos/{owner}/{repo}/issues/{index}/labels`
- **AND** the system SHALL return the updated issue

#### Scenario: Adding by id is unchanged

- **WHEN** the caller invokes `add_issue_labels` with `labels` set to `"5,6"` and those ids exist
- **THEN** the system SHALL POST those ids

#### Scenario: Removing by name

- **WHEN** the caller invokes `remove_issue_labels` with `labels` set to `"bug"`
- **THEN** the system SHALL DELETE `/repos/{owner}/{repo}/issues/{index}/labels/{id}` for that label
- **AND** the system SHALL return the updated issue

### Requirement: Replace or clear an issue's labels from update_issue

The `update_issue` tool SHALL accept an optional `set_labels` CSV that replaces the issue's entire label set through `PUT /repos/{owner}/{repo}/issues/{index}/labels`, SHALL clear every label through `DELETE /repos/{owner}/{repo}/issues/{index}/labels` when `set_labels` is present and empty, and SHALL NOT send `labels` on the issue PATCH because the Forgejo edit-issue endpoint ignores that field.

#### Scenario: Replacing the label set

- **WHEN** the caller invokes `update_issue` with `set_labels` set to `"bug"`
- **THEN** the system SHALL PUT that label's id to the issue labels endpoint
- **AND** the PATCH body SHALL NOT contain a `labels` field

#### Scenario: An empty set_labels clears every label

- **WHEN** the caller invokes `update_issue` with `set_labels` set to `""`
- **THEN** the system SHALL DELETE the issue labels collection
- **AND** the system SHALL NOT read the label catalogue

#### Scenario: set_labels alone sends no PATCH

- **WHEN** the caller invokes `update_issue` with `set_labels` and no other editable field
- **THEN** the system SHALL NOT send a PATCH to the issue

#### Scenario: set_labels with another field patches first, then replaces

- **WHEN** the caller invokes `update_issue` with both `title` and `set_labels`
- **THEN** the system SHALL send the PATCH before the label replacement
- **AND** the system SHALL return the issue as it stands after the replacement

#### Scenario: Omitting set_labels leaves today's behaviour intact

- **WHEN** the caller invokes `update_issue` without `set_labels`
- **THEN** the system SHALL send exactly one PATCH request and read no label catalogue
