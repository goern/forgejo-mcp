## Why

`forgejo-mcp` has no tools for managing repository webhooks, blocking any MCP-only workflow that needs to wire a Forgejo/Codeberg repository to an external system (e.g. Pipelines-as-Code controllers, CI integrations, mirroring). Without these tools, users must leave the MCP context and resort to the Forgejo web UI or raw API calls.

## What Changes

- Add six MCP tools for repository webhook CRUD:
  - `list_repo_hooks(owner, repo, page?, limit?)` — list all hooks
  - `get_repo_hook(owner, repo, id)` — get a single hook
  - `create_repo_hook(owner, repo, url, type, events[], content_type, secret?, http_method?, active?, branch_filter?)` — create a hook
  - `edit_repo_hook(owner, repo, id, …patch fields)` — update a hook
  - `delete_repo_hook(owner, repo, id)` — delete a hook
  - `test_repo_hook(owner, repo, id)` — trigger a test delivery
- Add two `forgejo://` resource templates:
  - `forgejo://repo/{owner}/{repo}/hooks{?page,limit}` — bounded embedded list of hooks
  - `forgejo://repo/{owner}/{repo}/hook/{id}` — single hook
- `secret` is never echoed in tool results or resource reads (Forgejo masks it server-side; the MCP layer must not add it back)

## Capabilities

### New Capabilities

- `repo-webhook-tools`: MCP tools and resource templates for repository webhook CRUD, covering list/get/create/edit/delete/test operations and the two `forgejo://` resource URIs

### Modified Capabilities

*(none — no existing spec-level requirements change)*

## Impact

- New file `operation/webhook/` (or `operation/hook/`) containing tool handlers and resource handlers
- New resource templates registered alongside existing ones in `operation/operation.go`
- Uses singleton `pkg/forgejo` client and `operation/resource` helpers (`Bounded`, `MapForgejoError`)
- Output bounding required on list tool and list resource per `docs/design/output-bounding.md`
- AGENTS.md resource table updated with the two new URI templates
