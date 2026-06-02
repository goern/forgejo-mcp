## Why

`forgejo-mcp` exposes **read** and **assignment** label tools — `list_repo_labels`, `list_org_labels`, `add_issue_labels`, `remove_issue_labels` (all in `operation/issue/issue.go`) — but has **no label CRUD**: you cannot create, edit, or delete a repo or org label through MCP.

This surfaced 2026-05-31 (Codeberg #190) while creating the `RFC - Request For Comments` label for #176: the work fell back to raw `curl` against the Forgejo REST API because no MCP tool exists. Under the project's MCP-only policy that is a hard stop, not a minor annoyance — any project bootstrapping its own labels (RFC, triage states, priority tiers) is blocked.

Repo-label CRUD is a thin wrapper over `forgejo-sdk/v3` methods that **already ship in the pinned dependency**; org-label CRUD has no SDK method and goes through the existing `pkg/forgejo` raw-HTTP `DoJSON` helper, exactly like `fetchOrgLabels`. The change also extends the additive `forgejo://` resource-template surface (per `mcp-resources-core`) to labels, so labels become URI-addressable like the existing issue / pr / commit resources.

## What Changes

- Add **repo-label** CRUD MCP tools, naming consistent with existing `create_*` / `edit_*` / `delete_*` patterns:
  - `create_repo_label(owner, repo, name, color, description?)` → created label (incl. numeric `id`)
  - `edit_repo_label(owner, repo, id, name?, color?, description?)` → updated label (PATCH: only supplied fields change)
  - `delete_repo_label(owner, repo, id, delete_mode?)` → success — **refuses an in-use label unless `delete_mode="force"`** (see below)
  - `get_repo_label(owner, repo, id)` → single label (fills the read-one gap; `list_repo_labels` already lists)
- Add **org-label** CRUD MCP tools (same shape, raw-HTTP `DoJSON` against `/orgs/{org}/labels[/{id}]`):
  - `create_org_label(org, name, color, description?)`
  - `edit_org_label(org, id, name?, color?, description?)`
  - `delete_org_label(org, id, delete_mode?)` → refuses an in-use label unless `delete_mode="force"`
  - `get_org_label(org, id)` → single org label
- **Safe delete**: `delete_repo_label` / `delete_org_label` first count issues/PRs referencing the label. If the count is `> 0` they **stop and report the in-use count** instead of deleting; `delete_mode="force"` overrides and deletes anyway. (Repo count via the SDK `ListRepoIssues` pagination filtered by label name — `pkg/forgejo.DoJSON` cannot read response headers, so no raw `X-Total-Count`; org count iterates the org's repos and is **best-effort** — it MAY under-count labels used in repos the token cannot read, and the refusal message says so. See design D9.)
- Implement repo-label CRUD via the pinned `codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3` methods (`CreateLabel`, `EditLabel`, `DeleteLabel`, `GetRepoLabel`); org-label CRUD via `pkg/forgejo` `DoJSON` — **no new dependency**.
- Normalise `color` to the Forgejo hex form (`#rrggbb`) before send so a missing `#` does not `422` (both repo and org tools).
- Add additive `forgejo://` resource-templates, coexisting with the tools (no tool removed):
  - `forgejo://repo/{owner}/{repo}/labels` → bounded list of repo labels
  - `forgejo://repo/{owner}/{repo}/label/{id}` → single repo label
  - `forgejo://org/{org}/labels` → bounded list of org labels (gate lifted now org CRUD lands here)
- Reuse `operation/resource` helpers (`ParseXxx`, `Bounded` + `EmbeddedListCap`, `MapForgejoError`) and satisfy `docs/design/output-bounding.md` for the list resources.
- Surface the new tools and resource-templates in the README tool/resource tables and `AGENTS.md`.

## Capabilities

### New Capabilities

- `label-crud`: Repo- **and org**-label create / edit / delete / get-one MCP tools. Repo labels wrap `forgejo-sdk/v3`; org labels use `pkg/forgejo` raw-HTTP `DoJSON`. Shared hex-color normalisation, PATCH edit semantics, and safe-delete (`delete_mode` enum + in-use guard).
- `mcp-resource-label`: `forgejo://repo/{owner}/{repo}/labels` (bounded list), `forgejo://repo/{owner}/{repo}/label/{id}` (single), and `forgejo://org/{org}/labels` (bounded org-label list) resource-templates, following the `mcp-resources-core` convention.

### Modified Capabilities

- `mcp-resources-core`: Adds a **Collection resource** requirement. `forgejo://repo/{owner}/{repo}/labels` is the first standalone collection-as-resource in the scheme (every prior resource is a single-entity fetch; lists were embedded or tool-only). Rather than let that precedent harden implicitly, this change records it normatively in the core framework: a bounded collection MAY be a resource, capped with `EmbeddedListCap` + the shared sentinel, without removing the corresponding list tool. Future list entities (org labels, branches, milestones) cite this rule. No existing requirement is altered — this is a new requirement on the capability.

Existing list/assignment label tools (`list_repo_labels`, `list_org_labels`, `add_issue_labels`, `remove_issue_labels`) are unchanged.

## Dependency / ordering

The `mcp-resources-core` delta above assumes the `mcp-resource-templates` change has archived (it is the home of the `mcp-resources-core` capability, currently unarchived). This change SHOULD archive after `mcp-resource-templates`; if it lands first, the core-spec delta is held until the base capability exists in `openspec/specs/`.

## Impact

- **Affected code**: a label tool file under `operation/<domain>/` (issue domain currently owns labels — keep them together or split a `label` domain, implementer's call) registered from `operation/operation.go`; `resources*.go` for the three label templates; a `ParseLabel` (repo) and `ParseOrgLabels` (org list) parser in `operation/resource`; an org-label `DoJSON` block in `pkg/forgejo` mirroring `fetchOrgLabels`; a shared in-use-count helper.
- **No new external dependencies.** Repo-label CRUD uses SDK methods already pinned; org-label CRUD uses the existing `pkg/forgejo` `DoJSON` helper; resources use the existing `mcp-go` resource APIs and the singleton `pkg/forgejo` client.
- **No breaking changes.** All existing tools remain; resource-unaware clients see exactly today's surface.
- **Documentation**: README tool table gains the eight label tools; README "Resources" section + `AGENTS.md` resource table gain the three label templates.

## Out of Scope

- **`exclusive` / `is_archived` label fields**: SDK v3 `Label` does not model them (exposes `id`, `name`, `color`, `description`, `url` only); for parity the org-label raw-HTTP path also omits them in the first cut. Revisit via raw-HTTP passthrough (both paths) if a use case appears.
- **Implementation**: this change is proposal + design + specs + tasks. Code lands when tasks are applied.
