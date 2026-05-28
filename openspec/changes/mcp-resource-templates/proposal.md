## Why

`forgejo-mcp` is tool-only today. Core Forgejo entities (owner, repo, commit, issue, pr, comment, status) are noun-like, sha- or index-addressable, and often stable — natural MCP `resource` candidates. Exposing them as URI-addressable resources unlocks auto-resolution from LLM context (e.g. a sha mentioned in a PR diff is fetched without an explicit tool call), content-addressable caching for immutable entities (commits, statuses pinned to a sha), and composable references between entities. Doing this one entity at a time would leave the server in a mixed tool/resource state with inconsistent URI conventions; this change designs all seven entities at once and lets tasks land incrementally.

## What Changes

- Add resource-template support to `operation/operation.go` server construction: register `server.WithResourceCapabilities(...)` and define resource templates for each entity (no implementation yet — design only).
- Introduce a `forgejo://` URI scheme with one template per entity:
  - `forgejo://owner/{owner}` — user or organization
  - `forgejo://repo/{owner}/{repo}` — repository metadata
  - `forgejo://repo/{owner}/{repo}/commit/{sha}` — commit metadata
  - `forgejo://repo/{owner}/{repo}/issue/{index}` — issue
  - `forgejo://repo/{owner}/{repo}/pr/{index}` — pull request
  - `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` — comment (`kind` ∈ `issue`, `pr`)
  - `forgejo://repo/{owner}/{repo}/commit/{sha}/status` — combined CI status for a commit
- Establish a per-domain `resource/` subpackage pattern under `operation/{domain}/` paralleling the existing tool pattern, with a `RegisterResources(s *server.MCPServer)` entry point.
- Define coexistence rules with existing tools: resources are read-only entity *fetchers*; mutating actions (create issue, merge PR, post comment) stay tools. Existing read-only list tools (`list_repo_issues`, `list_pull_request_files`, …) stay — they are bounded queries, not single-entity fetches.
- Apply output-bounding contract from `docs/design/output-bounding.md` to resource responses: any embedded list (e.g. comment threads on an issue resource, statuses on a commit resource) must be capped with an explicit truncation marker and link to a list tool for enumeration.
- Document client-compatibility expectations: targets clients that implement MCP `resources/templates/list` and `resources/read` (Claude Code, Claude Desktop, Codex, current Cursor). Clients without template support fall back to existing tools — no functionality removed.
- Subscription semantics: explicitly OUT of scope for v1. No `resources/subscribe` handler. Resources are read-on-demand only.
- Auth & errors: resources reuse the existing singleton `pkg/forgejo` client; private-entity reads return MCP error with the same `403` / `404` mapping the tools use today.

## Capabilities

### New Capabilities

- `mcp-resources-core`: Resource-template registration framework — URI scheme, capability declaration on the MCP server, per-domain `RegisterResources` entry point, shared URI parser, error mapping, output-bounding rules for embedded lists, and the coexistence contract between resources and tools.
- `mcp-resource-owner`: `forgejo://owner/{owner}` template — resolves a user or organization, returns identity metadata.
- `mcp-resource-repo`: `forgejo://repo/{owner}/{repo}` template — repository metadata (name, default branch, visibility, description, counts).
- `mcp-resource-commit`: `forgejo://repo/{owner}/{repo}/commit/{sha}` template — commit metadata (author, committer, message, parents, tree sha); immutable, marked cacheable.
- `mcp-resource-issue`: `forgejo://repo/{owner}/{repo}/issue/{index}` template — issue body, labels, assignees, state, milestone; recent comments embedded with bound + truncation marker.
- `mcp-resource-pr`: `forgejo://repo/{owner}/{repo}/pr/{index}` template — PR metadata, head/base refs, mergeability; recent comments and review summaries embedded with bound + truncation marker.
- `mcp-resource-comment`: `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` template — single comment body and metadata; `kind` discriminates issue vs PR comments.
- `mcp-resource-status`: `forgejo://repo/{owner}/{repo}/commit/{sha}/status` template — combined CI status (state + per-context statuses) for a commit sha; pinned to sha, cacheable.

### Modified Capabilities

<!-- None. Existing tool capabilities are unchanged. This is purely additive: resources sit alongside tools and clients may use either. The coexistence rule is captured under `mcp-resources-core`, not as a modification of any single existing tool spec. -->

## Impact

- **Affected code**: `operation/operation.go` (server construction, capability flag, new `RegisterResources*` calls); seven new `operation/{domain}/resources.go` files (one per entity); a new shared `operation/resource/` package for URI parsing, template registration helpers, and embedded-list bounding. No changes to existing tool files.
- **No new external dependencies.** Uses `mcp-go` resource APIs already pulled in transitively via `server.MCPServer`. Uses the singleton `pkg/forgejo` client unchanged.
- **No breaking changes.** All existing tools remain. Clients that don't support resource templates see exactly today's surface.
- **Documentation**: README gains a "Resources" section paralleling the tool table; `docs/design/output-bounding.md` is referenced (not amended) for the embedded-list rule.
- **Rollout**: per-entity tasks let `mcp-resources-core` + `mcp-resource-commit` land first as a thin slice (highest immediate value for CI/status workflows), with the remaining six entities following independently. Each entity entry-point is gated behind its own `RegisterResources*` call so partial rollout is safe.
- **Out of scope for this change**: implementation (specs + design only), wiki/projects resources (blocked upstream per `docs/plans/`), webhook/event resources, `resources/subscribe`, write-through resources.
