<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

### Requirement: List branch protection rules (bounded)

The server SHALL provide a `list_branch_protections` tool that returns the branch protection rules of a repository given `owner` and `repo`. The tool SHALL expose caller-controlled `page` and `limit` bounds and the response SHALL be resumable by echoing the page that was returned, satisfying `docs/design/output-bounding.md`. The response SHALL also carry an optional `total_count`, populated from the upstream `X-Total-Count` response header via `pkg/forgejo.TotalCountPtr` and omitted (`omitempty`) — never `0` — when the header is absent or unparsable.

#### Scenario: List returns the repository's rules

- **WHEN** `list_branch_protections` is called with a valid `owner` and `repo`
- **THEN** the result SHALL contain the repository's branch protection rules as returned by `GET /repos/{owner}/{repo}/branch_protections`
- **AND** each rule SHALL include at least its `rule_name`, `branch_name`, `enable_status_check`, `status_check_contexts`, and `required_approvals`

#### Scenario: Caller bounds the page size

- **WHEN** `list_branch_protections` is called with `limit` set to N
- **THEN** the request to Forgejo SHALL carry a page size of N
- **AND** the response SHALL indicate the page returned so the caller can request the next page

#### Scenario: Total count surfaced when Forgejo reports it

- **WHEN** the upstream response carries `X-Total-Count: 3`
- **THEN** the response SHALL contain `total_count: 3`

#### Scenario: Total count omitted, not zeroed, when unavailable

- **WHEN** the upstream response carries no `X-Total-Count` header, or an unparsable one
- **THEN** the response SHALL NOT contain a `total_count` key
