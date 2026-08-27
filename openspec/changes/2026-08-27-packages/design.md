# SPDX-License-Identifier: GPL-3.0-or-later

## Context

Packages belong to a user or org, not a repository. Optional `repository` on a version is a link, not the lookup key. Forgejo lists **versions** (`SearchVersions`), filters with `type` and `q`, and sets `X-Total-Count`. File listing has no paging parameters.

The MCP Go SDK still has no `WithArray` concern here: arguments are scalars. `type` is the Forgejo query/path name; Go locals use `packageType`.

## Goals / Non-Goals

**Goals**

- Four tools wrapping the four REST endpoints agents actually need.
- Honest list: one row per version; `type` optional (all types); `q` is Forgejo's name filter, not the get/delete `name`.
- Server-side paging for the version list; `total_count` only from `X-Total-Count`.
- Lossy SDK avoided: raw HTTP so `type`/`q` and `html_url` survive.
- Projected payloads so a page of versions does not embed full `User` / `Repository` objects.

**Non-Goals**

- `POST …/-/link/{repo}` and `…/-/unlink` (no SDK; rare admin).
- Upload (`PUT /api/packages/…` — different prefix, binary).
- `include_internal` (handler hard-codes `IsInternal: false`).
- Resource templates (`forgejo://owner/{owner}/package/…`). Container names contain `/`; tools first.
- GC helpers / `keep_sha` / delete-all-versions-of-a-name.
- Changing `edit_repo` / `has_packages`.

## Decisions

### D1: One row is one version

Name the list tool `list_packages` (Forgejo `listPackages`). The description's first sentence says each row is a version. Do not rename to `list_package_versions`.

### D2: `type` has no default

Empty `type` omits the query parameter (every type). Do not default to `container`. Do not reject unknown type strings in the MCP layer; Forgejo's enum grows.

### D3: Raw HTTP for the whole family

`APIPath` + `DoJSON` / `DoJSONWithHeader`. SDK `ListPackages` drops `type`/`q`. SDK `Package` drops `html_url`. Mixing SDK get/delete with raw list is a worse review than one transport.

List must not use `DoJSONList`: a 404 owner is an error, not `[]`.

### D4: Project the row

`id`, `type`, `name`, `version`, `html_url`, `created_at`, `repository` as `full_name` or omitted. Extra JSON from Forgejo is dropped by unmarshalling into a local DTO.

Files: `id`, `name`, `size`, `sha256`. Drop `md5`/`sha1`/`sha512`.

### D5: Files are in this slice, client-sliced

Fourth SDK-equivalent method, same as topics wrapping all four topic methods. Forgejo returns the full file list; we slice with `page`/`limit` and `has_next`. No `total_count` — the handler does not call `SetTotalCountHeader`. Document that the server still sends every file.

### D6: Delete one version, no preflight

204 → `{owner, type, name, version, status: "deleted"}`. 4xx/5xx stay errors. No GET-then-skip. No delete-by-name.

### D7: No resource in this slice

Percent-encoding `/` in URI templates is easy to get wrong. Sentinel for a later collection resource is `list_packages`.

### D8: Parameter is `owner`, not `repo`

User or org. Not `list_repo_packages`. `q` on list vs `name` on get/delete stay distinct.

## Risks / Trade-offs

- Older instances 404 these routes; that is an error, not an empty success.
- Client-side file paging still fetches the full file list from Forgejo.
- `params.Owner` still says “Repository owner”; changing it would churn every repo tool. Package tool descriptions state user or org.
