## Why

`list_repo_labels` only returns repo-scoped labels (`/repos/{owner}/{repo}/labels`). Forgejo also supports organization-level labels (`/orgs/{org}/labels`) which can be applied to issues in any repo owned by that org. The MCP exposes none of them, so an AI agent has no way to discover org-label IDs needed by `add_issue_labels` and cannot distinguish org-labels already attached to issues. Reported as [Codeberg issue #125](https://codeberg.org/goern/forgejo-mcp/issues/125) by enbyted.

The Forgejo SDK (`forgejo-sdk/forgejo/v3@v3.0.0`) does not bind the org-label endpoint, but the project already has a raw-HTTP helper (`pkg/forgejo.DoJSONList`) used for similar SDK gaps (attachments). No SDK upgrade is required.

## What Changes

- Add `list_org_labels` MCP tool — returns all org-level labels for a given organization with their names and numeric IDs.
- Augment `list_repo_labels` with an `include_org_labels` boolean parameter (default `true`). When the repo owner is an organization and the flag is on, the tool merges org labels into the repo label result.
- Each label entry returned by `list_repo_labels` (with merge enabled) is annotated with a `scope` field of `"repo"` or `"org"` so callers can disambiguate IDs that may overlap by name.
- Use `pkg/forgejo.DoJSONList` for both endpoints — no new SDK dependency. 404 from the org endpoint maps to an empty list (helper already handles this).

## Capabilities

### New Capabilities

- `list_org_labels`: discover org-level label names → IDs for use with `add_issue_labels` on any repo owned by the organization.

### Modified Capabilities

- `list_repo_labels`: gains optional `include_org_labels` parameter and `scope` field on each returned label. The capability spec is introduced by the in-flight `list-milestones-labels` change; this change layers on top once that lands.

## Impact

- **Code**: `operation/issue/issue.go` — new `ListOrgLabelsTool` definition + handler + registration; modified `ListRepoLabelsFn` to optionally merge org labels and stamp `scope`. Possibly a small response-shape helper if scope tagging needs a wrapper type.
- **APIs**: One additional GET per `list_repo_labels` call when merge is enabled and the owner is an org. Standalone `list_org_labels` is a single GET.
- **Dependencies**: None added. Reuses `pkg/forgejo.DoJSONList`.
- **Order**: Depends on `list-milestones-labels` change archiving first so the `list_repo_labels` capability spec exists. If that has not landed, the `list_repo_labels` modified-capability delta in this change is held back, but `list_org_labels` (new capability) ships independently.
- **Risk**: Low. Read-only, additive. One extra HTTP call per merged listing.
