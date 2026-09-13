# SPDX-License-Identifier: GPL-3.0-or-later

## Why

Forgejo packages belong to a user or org. A repository's Packages tab stays empty until the package is linked. Containers sometimes autolink; npm, generic, maven, and most other types do not unless the published manifest already carries `repository.url`.

`get_package` (PR `#543`) already returns `repository` as the repo `full_name` when a link exists and **omits the key** when it does not. After that PR an agent can see “this generic package has no repository” and cannot close the gap. Link is the second button on the package Settings page. Unlink is the neighbouring button. SDK v3.0.0 `package.go` has List / Get / Delete / Files only — no Link / Unlink methods.

`packages` design listed `POST …/-/link/{repo}` and `…/-/unlink` as a non-goal (“no SDK; rare admin”). That was the revisit. The use case is now the hole `#543` made visible.

This is the same family, not a new domain. Two tools, not one tool with a mode: two buttons and two endpoints, same split as cancel vs delete on workflow runs (`#533`). Unlink is in this change because it is the other control on that page, the same `owner/type/name` coordinates, and a sibling POST. Deferring it is a third PR for the neighbouring button.

Not folded into `#543`: that PR's spec is archived; it is the read/delete surface. Link is a different REST path (name-scoped, no version), same split as topics vs `edit_repo`.

Raw HTTP because the SDK has no method and the packages family in this server is already `DoJSON`. Mixing SDK get with raw link would be a third path.

## What Changes

- Add MCP tools `link_package` and `unlink_package`.
- `link_package(owner, type, name, repo)` → `POST /packages/{owner}/{type}/{name}/-/link/{repo_name}` → 201 empty → `{owner, type, name, repo, status: "linked"}`.
- `unlink_package(owner, type, name)` → `POST …/-/unlink` → 204 → `{owner, type, name, status: "unlinked"}`.
- No `version` argument. Link is on the package name; every version of that name shares the repository.
- `repo` is the repository name under the same owner, not `owner/repo`. A `repo` containing `/` is an MCP error with no request.
- Cross-owner is upstream 404 ([forgejo#11483](https://codeberg.org/forgejo/forgejo/issues/11483)); the error is passed through.
- 4xx/5xx stay errors. Success statuses are not invented. Already-linked and unlink-when-not-linked are upstream 400 (`ErrInvalidArgument`); MCP does not map them to `"linked"` / `"unlinked"`.
- Read-back is existing `get_package`. No new get tool. No new field on the version DTO.
- README tool table, `extension/manifest.json`, `demos/packages.md`. No new URI.

## Capabilities

### New Capabilities

- None. Link and unlink are the Settings page of the same owner-scoped registry `#543` already exposed.

### Modified Capabilities

- `packages`: add `link_package` and `unlink_package`. List, get, delete, and files requirements stay as they are.

## Impact

- **Affected code**: `operation/packages/link.go`, `packages.go` (`RegisterTool`, name-scoped coords), `operation/params/params.go`, tests, README, `extension/manifest.json`, `demos/packages.md`.
- **APIs / SDK**: no SDK methods. `POST /packages/{owner}/{type}/{name}/-/link/{repo_name}`, `POST /packages/{owner}/{type}/{name}/-/unlink`. No new dependency.
- **Output bounding**: exempt — success is a fixed object (`docs/design/output-bounding.md`).
- **Not breaking**: two new tools. Existing list/get/delete/files unchanged. Lands after `#543` or stacked on `feat/packages`.

## Out of Scope

- Version list of a name, `/-/latest`, zip / blob download.
- Repo-scoped package list.
- Container autolink (publish-time behaviour, not this Settings button).
- Changes to `#543` paging or projection.
- SDK methods. Stay on `DoJSON`.
- New resource URIs. `AGENTS.md` resource table is unchanged.
