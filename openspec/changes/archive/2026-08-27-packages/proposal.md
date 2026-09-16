# SPDX-License-Identifier: GPL-3.0-or-later

## Why

Forgejo's built-in registry is owner-scoped (`GET|DELETE /packages/{owner}/…`). Agents that publish container, generic, npm, or maven packages have no MCP tools for it: they fall back to raw HTTP. The pinned `forgejo-sdk/v3` looks like it covers the family (`ListPackages`, `GetPackage`, `DeletePackage`, `ListPackageFiles`) but the DTOs and list query are lossy: `ListPackages` does not send `type` or `q`, and `Package` has no `html_url`.

`GET /packages/{owner}` is `SearchVersions`: each row is one **version**, not one package name. A tool that hid that would make garbage-collection and “what tags exist” lie.

## What Changes

- Add MCP tools `list_packages`, `get_package`, `delete_package`, `list_package_files`.
- Call Forgejo over raw HTTP (`APIPath` + `DoJSON` / `DoJSONWithHeader`). Do not wrap the SDK methods.
- `list_packages`: server-paged GET with `type`/`q`/`page`/`limit`; envelope `{packages, page, limit, count, total_count?}`. A missing owner is 404, not an empty list. Each row is one version.
- `get_package` / `delete_package`: one version. Delete has no preflight; 4xx/5xx stay errors. Success `{owner, type, name, version, status: "deleted"}`.
- `list_package_files`: GET all files, then client-side `page`/`limit`. Envelope `{files, page, limit, count, has_next}` — no `total_count` (the endpoint does not set `X-Total-Count`).
- Project list/get rows to `id`, `type`, `name`, `version`, `html_url`, `created_at`, `repository` (`full_name` or omitted). Project files to `id`, `name`, `size`, `sha256`.
- README Packages table, `demos/packages.md`, `extension/manifest.json`.

## Capabilities

### New Capabilities

- `packages`: List package versions for an owner, get or delete one version, and list that version's files. Raw HTTP against current Forgejo package endpoints. Delete removes one version, not every version of a name.

### Modified Capabilities

- None.

## Impact

- **Affected code**: `operation/packages/`, `operation/operation.go`, `cmd/cli.go`, README, demos, `extension/manifest.json`.
- **APIs / SDK**: no new SDK methods. `GET /packages/{owner}`, `GET|DELETE /packages/{owner}/{type}/{name}/{version}`, `GET /packages/{owner}/{type}/{name}/{version}/files`.
- **Output bounding**: get and delete return one object or success — exempt. List is server-paged (`page`/`limit`, default 30, max 50) with `count` plus `total_count` from `X-Total-Count`. Files are client-sliced with `has_next`; the API has no page query.
