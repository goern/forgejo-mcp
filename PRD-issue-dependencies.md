<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# PRD: Issue Dependency Tools for Forgejo MCP

## Problem Statement

Forgejo provides a native issue dependency feature in the web UI and REST API, but the `forgejo-mcp` server currently exposes no tools to read or manage these relationships. The dependency mutation endpoints use an `IssueMeta` request body (`{index, owner, repo}`), not a single `dependency_issue_index` field. Users and agents working through the MCP server cannot see which issues block or depend on other issues, nor can they add or remove such dependencies. This forces users to switch to the web UI, breaking the agent workflow.

## Solution

Add four dedicated MCP tools in the issue domain that mirror the Forgejo REST API endpoints for issue dependencies, plus the sibling `/blocks` endpoint for reverse relationships. The tools will use raw HTTP via the existing `pkg/forgejo` helper because the pinned `forgejo-sdk` (v3.0.0) does not expose typed methods for dependencies. Mutation endpoints send an `IssueMeta` body (`{index, owner, repo}`).

## User Stories

1. As an agent, I want to list the issues that a given issue depends on, so that I can report blockers and help the user understand why work is stalled.
2. As an agent, I want to list the issues that depend on a given issue, so that I can see the impact of closing or changing an issue.
3. As an agent, I want to add a dependency between two issues, so that I can model the dependency graph on behalf of the user.
4. As an agent, I want to remove a dependency between two issues, so that I can correct outdated relationships.
5. As a user, I want the tool parameters to be directionally clear, so that I do not accidentally reverse the dependency relationship.
6. As a user, I want mutation tools to return a simple success message, so that I do not pay an extra round-trip for a full list.
7. As a user, I want the tool to surface meaningful errors when a dependency is invalid, so that I know why an operation failed.
8. As a maintainer, I want these tools to follow the existing repository patterns for raw HTTP, error mapping, and handler tests, so that the change is consistent with the rest of the codebase.
10. As a user, I want these tools to coexist with the existing issue tools, so that nothing I already use is removed or changed.

## Implementation Decisions

- **Domain placement**: Add the tools to the issue domain (`operation/issue/`), alongside existing issue tools and handlers.
- **Tool names and semantics**:
  - `list_issue_dependencies`: returns the issues that the given issue depends on (`GET /repos/{owner}/{repo}/issues/{index}/dependencies`).
  - `list_issue_dependents`: returns the issues that depend on the given issue (`GET /repos/{owner}/{repo}/issues/{index}/blocks`).
  - `add_issue_dependency`: adds a dependency from the given issue to another issue (`POST /repos/{owner}/{repo}/issues/{index}/dependencies`).
  - `remove_issue_dependency`: removes a dependency from the given issue (`DELETE /repos/{owner}/{repo}/issues/{index}/dependencies`).
- **Parameter naming**: Use `owner`, `repo`, `index` for the subject issue, and `depends_on_index` for the target issue in `add_issue_dependency`. This reads naturally as "issue #X depends on issue #Y".
- **SDK gap**: The pinned `forgejo-sdk` v3.0.0 has no typed methods for dependencies. Use `pkg/forgejo.DoJSON` and `pkg/forgejo.DoJSONList` for the raw HTTP calls, consistent with other issue tools.
- **Mutation request body**: The Forgejo dependency mutation endpoints accept an `IssueMeta` body (`{index, owner, repo}`). Send this body for both `add_issue_dependency` and `remove_issue_dependency`, with `index` being the dependency issue index.
- **Response type**: The GET endpoints return an issue list. Decode them into the SDK's `Issue` type for reuse and consistency with other issue tools.
- **No pagination parameters**: The API supports `page` and `limit`, but dependency lists are typically short. Return the complete list from both list tools to keep the interface simple and symmetrical.
- **No resource changes**: The issue resource (`forgejo://repo/{owner}/{repo}/issue/{index}`) is intentionally unchanged. Dependencies are managed via tools, not embedded in the resource.
- **Mutation response**: Return a simple text success message. Callers can explicitly list dependencies after mutation if needed.
- **Error handling**: Pass through Forgejo errors using the existing `pkg/to.ErrorResult` and `pkg/forgejo` raw HTTP helpers. Pre-validate direct self-dependency on the client side to avoid a guaranteed server error; otherwise rely on the server for cycle detection. List endpoints treat 404 as an empty list.
- **Repo setting**: Forgejo has a repository flag `enable_issue_dependencies`. If disabled, the API will return an error. Document this failure mode in the tool descriptions; do not pre-check the setting.
- **Registration**: Register the new tools in the existing `RegisterDependencyTool` function in the issue domain, following the same pattern as other issue tools.

## Testing Decisions

- Use the existing handler-level test seam (`httptest` backend + recorded requests) in the issue domain.
- Test that `list_issue_dependencies` sends the correct GET request, decodes the issue list, and returns an empty list on 404.
- Test that `list_issue_dependents` sends a GET to the `/blocks` endpoint and returns an empty list on 404.
- Test that `add_issue_dependency` sends a POST with an `IssueMeta` body containing `index`, `owner`, and `repo`, and rejects self-dependency before any HTTP call.
- Test that `remove_issue_dependency` sends a DELETE with an `IssueMeta` body and surfaces API errors.
- Test that API errors are surfaced to the caller from the list tools.

## Out of Scope

- Adding pagination parameters to the list tools.
- Updating the pinned `forgejo-sdk` to gain typed dependency methods.
- Bulk add/remove operations; one dependency per call.
- UI or visualization of dependency graphs.
- Runtime auto-detection of which API dialect to use.

## Further Notes

- The Forgejo API dependency mutation endpoints use an `IssueMeta` body (`{index, owner, repo}`). An earlier implementation assumed a `dependency_issue_index` field, but live testing against Codeberg confirmed that the `IssueMeta` body is required.
- All new Go files in this change must begin with the SPDX license header `// SPDX-License-Identifier: GPL-3.0-or-later`.
- Cycle detection is intentionally left to the server; attempting to create a cycle may return a server error.
