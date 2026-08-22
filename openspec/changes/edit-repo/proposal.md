# SPDX-License-Identifier: GPL-3.0-or-later

## Why

`create_repo` can set a description (and privacy) only at creation time. After that there is no MCP tool to change a repository's description, website, default branch, visibility, archive flag, or unit toggles (`has_wiki`, `has_issues`, …). Agents fall back to raw `curl` against `PATCH /repos/{owner}/{repo}`.

The pinned `forgejo-sdk/v3` already wraps that endpoint as `EditRepo` / `EditRepoOption` and the matching read as `GetRepo`. There is no `get_repo` tool today: `list_my_repos` only lists the caller's own repos, and the `forgejo://repo/{owner}/{repo}` resource omits website. Tool-only clients cannot read an arbitrary `owner/repo` to verify a PATCH.

## What Changes

- Add MCP tool `get_repo(owner, repo)` wrapping `Client.GetRepo`.
- Add MCP tool `edit_repo(owner, repo, …)` wrapping `Client.EditRepo` with PATCH semantics: only caller-supplied fields are sent; supplying none is an error and does not call upstream.
- Optional fields in this slice: `name`, `description`, `website`, `default_branch`, `private`, `template`, `archived`, `has_issues`, `has_wiki`, `has_pull_requests`, `has_projects`, `has_releases`, `has_packages`, `has_actions`.
- Surface both tools in the README Repositories table, `demos/edit-repo.md`, and `extension/manifest.json`.
- No new dependency. Topics, merge-style, tracker objects, and mirror settings stay out of this change (see `openspec/changes/repo-topics` for topics).

## Capabilities

### New Capabilities

- `repo-settings`: Read one repository and PATCH its flat settings page (metadata, visibility/archive, unit toggles) via existing SDK methods.

### Modified Capabilities

- None.

## Impact

- **Affected code**: `operation/repo/` (new `edit.go`), `operation/params` (`Website`), `create_org`/`edit_org` descriptions reuse `params.Website`, README, demos, `extension/manifest.json`.
- **APIs / SDK**: `GetRepo`, `EditRepo` already in `forgejo-sdk/v3`.
- **Output bounding**: both tools return one repository object — exempt (`docs/design/output-bounding.md`).
