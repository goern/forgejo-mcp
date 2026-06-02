## Why

A repository with no branch protection let Renovate automerge a pull request while its required commit status was still `pending` (forgejo-mcp-f6h) — CI had not gone green. The server today cannot see or fix that misconfiguration: there is no MCP surface for Forgejo branch protection rules. Operators need to read protection state and enforce it (require status checks, approvals) through MCP so the gap is visible and correctable.

## What Changes

- **Add** five MCP tools wrapping the forgejo-sdk branch-protection endpoints (`/repos/{owner}/{repo}/branch_protections`):
  - `list_branch_protections` — bounded list of a repo's protection rules.
  - `get_branch_protection` — one rule by name.
  - `create_branch_protection` — create a rule (status-check + approval enforcement).
  - `edit_branch_protection` — update a rule with PATCH semantics (only fields the caller passes change).
  - `delete_branch_protection` — remove a rule.
- **Add** two `forgejo://` resource-templates exposing protection state read-only:
  - Collection: `forgejo://repo/{owner}/{repo}/branch_protections` (bounded embedded list).
  - Single entity: `forgejo://repo/{owner}/{repo}/branch_protection/{rule}`.
- The list tool and collection resource satisfy `docs/design/output-bounding.md`: client-controlled `page`/`limit`, a resumable response, and the shared `EmbeddedListCap` truncation sentinel naming `list_branch_protections`.
- `create`/`edit` round-trip `status_check_contexts` (the load-bearing field for the motivating bug) correctly; `edit` leaves unmentioned fields unchanged.
- No tool is removed; the new tools and resources coexist with all existing surfaces.

## Capabilities

### New Capabilities

- `branch-protection`: read and manage Forgejo branch protection rules through MCP — five CRUD tools plus a bounded collection resource and a single-rule resource under the `forgejo://repo/{owner}/{repo}/branch_protection(s)` URI scheme.

### Modified Capabilities

<!-- None. The resource-templates follow the established forgejo:// scheme and the
     output-bounding invariant, but the core resource capability (mcp-resources-core)
     is not yet a live spec (still owned by the unarchived mcp-resource-templates
     change), so this change does not declare a delta against it. The bounding +
     URI-scheme conventions are cited as design constraints in design.md. -->

## Impact

- **New code**: `operation/branchprotection/` (tools + `resources_branchprotection.go`); a `ParseBranchProtection`/`ParseBranchProtections` helper in `operation/resource/parse.go`; registration wiring in `operation/operation.go`.
- **SDK**: uses existing `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3` `Client.{List,Get,Create,Edit,Delete}BranchProtection` — no dependency change.
- **Docs**: README tool table gains the five tools with their bound parameters (output-bounding documentation contract).
- **Tests**: httptest-based handler + resource tests, including `status_check_contexts` round-trip and truncation-sentinel behavior.
- **Out of scope**: webhook/event-driven enforcement; auto-remediation; protection for tags or rulesets beyond the `branch_protections` API; exposing every one of the ~22 rule fields as tool params (a focused, documented subset is exposed).
