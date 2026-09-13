# SPDX-License-Identifier: GPL-3.0-or-later

## Why

Forgejo scoped labels (`kind/bug`) are how Exclusive enforces one-of-a-family on an issue. MCP can create the name; it cannot set or return `exclusive` or `is_archived`. The Exclusive checkbox stays off in the UI, and `list_repo_labels` / `get_repo_label` / `forgejo://…/label/{id}` do not even show the flag — the pinned SDK `Label` type has `id`, `name`, `color`, `description`, `url` only. An agent that “sets up a taxonomy” therefore writes decorations, not the invariant.

`label-crud` reserved this revisit: raw-HTTP passthrough on both repo and org paths, optional params, no third code path. The use case is now live (Forgejo 16, Exclusive labels whose list JSON omits the field). `is_archived` is the other checkbox on the same Edit label form, the same JSON, the same `*bool` PATCH — not a second domain. A new DTO that dropped it would hide a field we now parse.

This change does not archive `label-crud`. That change still carries an `mcp-resources-core` delta and an archive-ordering promise. When it archives, “repo via SDK CreateLabel” is superseded by this transport switch; that sentence is their archive, not this one.

## What Changes

- Optional `exclusive` and `is_archived` on `create_repo_label`, `create_org_label`, `edit_repo_label`, `edit_org_label`.
- PATCH as `edit_repo`: omitted key is absent; `false` is sent; exclusive-only and archived-only edits are valid; empty edit is still an error with no request.
- `list_repo_labels`, `list_org_labels`, `get_repo_label`, `get_org_label`, and the existing label resource templates return both fields as booleans (`false` is present, not omitted).
- Repo create/edit/get/list and label resources use `DoJSON` and a local DTO. Org already used `DoJSON`. Delete stays on the SDK (exclusive is not on that wire).
- Create with `exclusive=true` and a name that is not scoped (`/` not at either end) is an MCP error with no request. Exclusive-only edit does not extra-GET the current name.
- README tool rows, `extension/manifest.json`, and `demos/label-management.md`. No new URI.

## Capabilities

### New Capabilities

- `label-exclusive`: optional `exclusive` and `is_archived` on create and edit of repo and org labels; both fields on every label read (tools and existing resources); repo transport is raw HTTP so the JSON contract is one type.

### Modified Capabilities

- None. Resource **templates** are unchanged; their JSON **body** gains two fields, recorded as read scenarios on `label-exclusive`. `label-crud` is still an unarchived change and is not reopened here.

## Impact

- **Affected code**: `operation/issue/label.go`, `issue.go` (`ScopedLabel`, list), `resources_label.go`, tests, README, `extension/manifest.json`, `demos/label-management.md`.
- **APIs**: Forgejo `POST`/`PATCH`/`GET` `/repos/{owner}/{repo}/labels[/{id}]` and `/orgs/{org}/labels[/{id}]`. No SDK upgrade. No new dependency.
- **Output bounding**: exempt — create/edit/get return one object; existing list envelopes are untouched (`docs/design/output-bounding.md`).
- **Not breaking**: new optional params; existing calls omit the keys.

## Out of Scope

- Label templates, org-wide defaults, tool renames.
- Issue assignment (open `#591`).
- Exclusive-on-apply (Forgejo already does this once the flag is set).
- Archiving `label-crud`.
- New resource URIs.
