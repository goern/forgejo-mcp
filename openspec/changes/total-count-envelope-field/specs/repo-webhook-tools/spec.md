<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## MODIFIED Requirements

### Requirement: List repository hooks
The server SHALL expose a `list_repo_hooks` MCP tool that returns a paginated list of all webhooks registered on a repository. The tool MUST accept `owner`, `repo`, `page` (default 1), and `limit` (default 30) parameters with no server-imposed ceiling (the tool is the unbounded enumeration path). The response MUST include a truncation sentinel naming the `list_repo_hooks` tool only when used from the resource path; the tool itself returns whatever the SDK page returns. The response SHALL also carry an optional `total_count`, populated from the upstream `X-Total-Count` response header via `pkg/forgejo.TotalCountPtr` and omitted (`omitempty`) — never `0` — when the header is absent or unparsable.

#### Scenario: List hooks returns results
- **WHEN** a client calls `list_repo_hooks` with a valid `owner`/`repo`
- **THEN** the tool returns a JSON object containing a `hooks` array with each hook's `id`, `type`, `config` (without `secret`), `events`, `active`, and `created` fields

#### Scenario: List hooks respects page and limit
- **WHEN** a client calls `list_repo_hooks` with `page=2` and `limit=10`
- **THEN** the tool returns at most 10 hooks from the second page

#### Scenario: List hooks truncation sentinel
- **WHEN** the repository has more hooks than the requested `limit`
- **THEN** the response includes `truncated: true` and a `list_tool: "list_repo_hooks"` sentinel signalling that more results exist; the sentinel does NOT report the total repository-wide hook count (it reflects the fetched window only)

#### Scenario: List hooks on repo with no hooks
- **WHEN** a client calls `list_repo_hooks` on a repository with zero hooks
- **THEN** the tool returns an empty `hooks` array and no truncation sentinel

#### Scenario: Total count surfaced when Forgejo reports it
- **WHEN** the upstream response carries `X-Total-Count: 12`
- **THEN** the response SHALL contain `total_count: 12`

#### Scenario: Total count omitted, not zeroed, when unavailable
- **WHEN** the upstream response carries no `X-Total-Count` header, or an unparsable one
- **THEN** the response SHALL NOT contain a `total_count` key
