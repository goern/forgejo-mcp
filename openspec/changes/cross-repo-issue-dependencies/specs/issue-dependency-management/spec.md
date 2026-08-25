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

### Requirement: Self-dependency check is scoped to the same repository

`add_issue_dependency` SHALL reject a request where the resolved dependency owner/repo equals the target issue's own owner/repo and `depends_on_index` equals `index`, and SHALL NOT reject a request where the indices match but the resolved dependency owner/repo differs from the target's, since that names a distinct issue in another repository.

#### Scenario: Same-repo self-dependency rejected

- **WHEN** a caller invokes `add_issue_dependency` with `depends_on_index` equal to `index` and no cross-repo arguments (or cross-repo arguments equal to the target's own owner/repo)
- **THEN** the tool SHALL reject the request with an error before contacting Forgejo

#### Scenario: Cross-repo same index is not a self-dependency

- **WHEN** a caller invokes `add_issue_dependency` with `depends_on_index` equal to `index` but `depends_on_owner`/`depends_on_repo` naming a different repository
- **THEN** the tool SHALL accept the request and send it to Forgejo
