# PRD: Issue Dependency Tools for `forge.he-int.de` API

## Problem Statement

The `forge.he-int.de` instance exposes Forgejo issue-dependency API endpoints, but they use a different request body shape than the one assumed by the upstream implementation. The upstream code sends `{"dependency_issue_index": N}` for mutations, which this instance rejects. Users of this MCP server need to read and manage issue dependencies on `forge.he-int.de`, so the implementation must be adapted to the instance's API dialect.

## Solution

Adapt the issue dependency tools to use the exact API contract published by `forge.he-int.de`:

- Use the `/dependencies` endpoints for dependency list, add, and remove operations.
- Use the `/blocks` endpoint only for listing issues that depend on the given issue.
- Send an `IssueMeta` body `{index, owner, repo}` for mutation endpoints.

This preserves the user-facing semantics from the upstream PRD while matching the instance's actual API.

## User Stories

1. As an agent, I want to list the issues that a given issue depends on, so that I can report blockers and help the user understand why work is stalled.
2. As an agent, I want to list the issues that depend on a given issue, so that I can see the impact of closing or changing an issue.
3. As an agent, I want to add a dependency between two issues, so that I can model the dependency graph on behalf of the user.
4. As an agent, I want to remove a dependency between two issues, so that I can correct outdated relationships.
5. As a user, I want the tool parameters to be directionally clear, so that I do not accidentally reverse the dependency relationship.
6. As a user, I want mutation tools to return a simple success message, so that I do not pay an extra round-trip for a full list.
7. As a user, I want the tool to surface meaningful errors when a dependency is invalid, so that I know why an operation failed.
8. As a maintainer, I want these tools to follow the existing repository patterns for raw HTTP, error mapping, and handler tests, so that the change is consistent with the rest of the codebase.
9. As a maintainer, I want this variant to coexist with the upstream implementation, so that both can be selected by branch or configuration.

## Implementation Decisions

- Keep the tools in the issue domain and register them in the existing `RegisterTool` function, mirroring the upstream pattern.
- Tool semantics remain unchanged:
  - `list_issue_dependencies` returns issues that the given issue depends on.
  - `list_issue_dependents` returns issues that depend on the given issue.
  - `add_issue_dependency` makes the given issue depend on another issue.
  - `remove_issue_dependency` removes a dependency from the given issue.
- API mapping for `forge.he-int.de`:
  - `list_issue_dependencies` → `GET /repos/{owner}/{repo}/issues/{index}/dependencies`.
  - `list_issue_dependents` → `GET /repos/{owner}/{repo}/issues/{index}/blocks`.
  - `add_issue_dependency` → `POST /repos/{owner}/{repo}/issues/{index}/dependencies` with body `{index, owner, repo}` where `index` is the issue being depended on.
  - `remove_issue_dependency` → `DELETE /repos/{owner}/{repo}/issues/{index}/dependencies` with body `{index, owner, repo}` where `index` is the dependency to remove.
- Use the existing `pkg/forgejo.DoJSON` and `pkg/forgejo.DoJSONList` raw HTTP helpers.
- Pre-validate direct self-dependency on the client side to avoid a guaranteed server error.
- Leave server-side validation (cycle detection, archived repo errors, disabled feature) to the Forgejo server.
- Mutation tools return a simple text success message.

## Testing Decisions

- Use the existing handler-level test seam (`httptest` backend + recorded requests) in the issue domain.
- Test that `list_issue_dependencies` sends the correct GET request and decodes the issue list.
- Test that `list_issue_dependents` sends a GET to the `/blocks` endpoint.
- Test that `add_issue_dependency` sends a POST with an `IssueMeta` body containing `index`, `owner`, and `repo`.
- Test that `remove_issue_dependency` sends a DELETE with an `IssueMeta` body.
- Test that self-dependency is rejected before any HTTP call is made.
- Test that API errors are surfaced to the caller and that 404s on list endpoints are treated as empty lists.
- Perform end-to-end CLI validation against a private test repo on `forge.he-int.de` to confirm all four tools work against the live instance.

## Out of Scope

- Adding pagination parameters to the list tools.
- Updating the pinned `forgejo-sdk` to gain typed dependency methods.
- Bulk add/remove operations; one dependency per call.
- UI or visualization of dependency graphs.
- Merging this variant into the upstream branch.
- Runtime auto-detection of which API dialect to use.

## Further Notes

- The `forge.he-int.de` Swagger is available at `https://forge.he-int.de/swagger.v1.json` and shows the `IssueMeta` body for the dependency mutation endpoints.
- The upstream implementation (using `dependency_issue_index`) is preserved on `feature/issue-dependency-tools-upstream-api`.
- All new Go files in this variant must begin with the SPDX license header `// SPDX-License-Identifier: GPL-3.0-or-later`.
- Cycle detection is intentionally left to the server; attempting to create a cycle (e.g. issue 1 depends on issue 3 and issue 3 depends on issue 1) returned a 500 Internal Server Error from the live instance during validation.
