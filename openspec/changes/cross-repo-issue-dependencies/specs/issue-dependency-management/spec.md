<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## ADDED Requirements

### Requirement: List issue dependencies and dependents

The server SHALL expose `list_issue_dependencies` and `list_issue_dependents` tools, each returning a paginated list of issues related to the given issue: dependencies (issues it depends on) and dependents (issues that depend on it), respectively. Both tools SHALL use `page` (1-based, default 1) and `limit` (default 20) and SHALL echo the effective `page` and `limit` in the response so a caller can fetch the next page. Both tools SHALL fail if the repository has disabled issue dependencies.

#### Scenario: List dependencies

- **WHEN** a caller invokes `list_issue_dependencies` for an issue that depends on others
- **THEN** the response SHALL contain those issues
- **AND** SHALL echo the effective `page` and `limit`

#### Scenario: List dependents

- **WHEN** a caller invokes `list_issue_dependents` for an issue that other issues depend on
- **THEN** the response SHALL contain those dependent issues
- **AND** SHALL echo the effective `page` and `limit`

### Requirement: Add a cross-repo issue dependency

The `add_issue_dependency` tool SHALL make the issue identified by `owner`/`repo`/`index` depend on `depends_on_index`, and SHALL accept optional `depends_on_owner`/`depends_on_repo` arguments naming the repository the dependency issue lives in, defaulting to the target issue's own `owner`/`repo` when omitted. The resolved dependency owner and repository SHALL be sent as the `owner`/`repo` fields of the IssueMeta request body, not the target issue's own `owner`/`repo`.

#### Scenario: Same-repo dependency (no cross-repo arguments)

- **WHEN** a caller invokes `add_issue_dependency` without `depends_on_owner`/`depends_on_repo`
- **THEN** the request body SHALL carry the target issue's own `owner`/`repo` as the dependency's owner/repo

#### Scenario: Cross-repo dependency

- **WHEN** a caller invokes `add_issue_dependency` with `depends_on_owner`/`depends_on_repo` naming a different repository
- **THEN** the request body SHALL carry that owner/repo, not the target issue's own `owner`/`repo`

### Requirement: Remove a cross-repo issue dependency

The `remove_issue_dependency` tool SHALL remove `dependency_index` as a dependency of the issue identified by `owner`/`repo`/`index`, and SHALL accept optional `dependency_owner`/`dependency_repo` arguments naming the repository the dependency issue lives in, defaulting to the target issue's own `owner`/`repo` when omitted. The resolved dependency owner and repository SHALL be sent as the `owner`/`repo` fields of the IssueMeta request body, not the target issue's own `owner`/`repo`.

#### Scenario: Same-repo removal (no cross-repo arguments)

- **WHEN** a caller invokes `remove_issue_dependency` without `dependency_owner`/`dependency_repo`
- **THEN** the request body SHALL carry the target issue's own `owner`/`repo` as the dependency's owner/repo

#### Scenario: Cross-repo removal

- **WHEN** a caller invokes `remove_issue_dependency` with `dependency_owner`/`dependency_repo` naming a different repository
- **THEN** the request body SHALL carry that owner/repo, not the target issue's own `owner`/`repo`

### Requirement: An optional cross-repo argument is absent or malformed, never silently defaulted

The optional cross-repo arguments — `depends_on_owner`/`depends_on_repo` on `add_issue_dependency` and `dependency_owner`/`dependency_repo` on `remove_issue_dependency` — SHALL distinguish an absent value from a malformed one. An argument that is omitted, `null`, or an empty string SHALL resolve to the target issue's own `owner`/`repo`. An argument that is present but is not a string SHALL be rejected with an error naming the argument, before any request is sent to Forgejo.

Collapsing the two would write a dependency against the target's own repository for a caller that asked for a different one, and report success for a thing it did not do. A refused call is the better outcome.

#### Scenario: Omitted, null or empty resolves to the target's own repository

- **WHEN** a caller invokes `add_issue_dependency` or `remove_issue_dependency` with a cross-repo argument omitted, set to `null`, or set to an empty string
- **THEN** the tool SHALL use the target issue's own `owner`/`repo` for that field of the IssueMeta request body

#### Scenario: A non-string cross-repo argument is refused

- **WHEN** a caller invokes `add_issue_dependency` or `remove_issue_dependency` with a cross-repo argument that is present but not a string
- **THEN** the tool SHALL return an error naming that argument
- **AND** SHALL NOT send any request to Forgejo

### Requirement: Self-dependency check is scoped to the same repository

`add_issue_dependency` SHALL reject a request where the resolved dependency owner/repo equals the target issue's own owner/repo and `depends_on_index` equals `index`, and SHALL NOT reject a request where the indices match but the resolved dependency owner/repo differs from the target's, since that names a distinct issue in another repository. Owner and repository names SHALL be compared case-insensitively, because Forgejo treats them so: a differently-cased spelling names the same repository, and comparing exactly would let it past this check to be refused server-side with a less clear message.

#### Scenario: Same-repo self-dependency rejected

- **WHEN** a caller invokes `add_issue_dependency` with `depends_on_index` equal to `index` and no cross-repo arguments (or cross-repo arguments equal to the target's own owner/repo)
- **THEN** the tool SHALL reject the request with an error before contacting Forgejo

#### Scenario: Case-differing spelling of the same repository is a self-dependency

- **WHEN** a caller invokes `add_issue_dependency` with `depends_on_index` equal to `index` and `depends_on_owner`/`depends_on_repo` differing from the target's own owner/repo only in letter case
- **THEN** the tool SHALL reject the request with an error before contacting Forgejo

#### Scenario: Cross-repo same index is not a self-dependency

- **WHEN** a caller invokes `add_issue_dependency` with `depends_on_index` equal to `index` but `depends_on_owner`/`depends_on_repo` naming a different repository
- **THEN** the tool SHALL accept the request and send it to Forgejo
