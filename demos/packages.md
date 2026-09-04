# SPDX-License-Identifier: GPL-3.0-or-later

# Demo: list, get, and inspect package versions

Forgejo's built-in registry is owner-scoped. `GET /packages/{owner}` is
`SearchVersions`: each row is one **version**, not one package name. Optional
`type` and `q` filter; `page`/`limit` are sent to the server. File listing has
no paging on the API — the tool fetches the list and slices it.

Do **not** run `delete_package` against a registry you do not own. This
walkthrough shows `--help` for delete and the read-only list/get/files
surface against a fictional `OWNER`.

## Setup

```bash
export FORGEJO_URL=https://git.b4mad.industries
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"
make build
```

Spec: `openspec/specs/packages/spec.md`

## 1. Tool surface

```bash
${FORGEJO_MCP_BIN} --cli list 2>/dev/null | grep -E "list_packages|get_package|delete_package|list_package_files"
```

```
  delete_package                           Delete one package version. Removes that version only, not every version of the name. No preflight GET. HTTP 204 returns {owner, type, name, version, status: "deleted"}; 4xx/5xx stay errors.
  get_package                              Get one package version (id, type, name, version, html_url, created_at, repository full_name). Does not embed owner or creator users.
  list_package_files                       List files of one package version. Forgejo returns the full file list with no paging; page (default 1) and limit (default 30, maximum 50) slice it client-side. Returns {files, page, limit, count, has_next, total_count}. total_count is the fetched list length (the files endpoint has no X-Total-Count). Each file is id, name, size, sha256.
  list_packages                            List package versions for a user or org (Forgejo SearchVersions: one row per version, not one per name). Optional type and q filter; page (default 1) and limit (default 30, maximum 50) are sent as query parameters. Returns {packages, page, limit, count, has_next} and total_count when Forgejo sets X-Total-Count. has_next is Link rel=next. A missing owner is an error, not an empty list.
```

```bash
${FORGEJO_MCP_BIN} --cli list_packages --help
${FORGEJO_MCP_BIN} --cli get_package --help
${FORGEJO_MCP_BIN} --cli list_package_files --help
${FORGEJO_MCP_BIN} --cli delete_package --help
```

`list_packages` takes `owner` and optional `type`, `q`, `page`, `limit`.
`get_package` / `delete_package` / `list_package_files` take `owner`, `type`,
`name`, `version`. Files add `page` and `limit`.

## 2. List versions (read-only)

One row per version. Omit `type` to include every type; pass `container` or
`generic` to narrow. `q` is Forgejo's name substring, not the get/delete
`name`. A missing owner is an error, not `[]`.

```bash
${FORGEJO_MCP_BIN} --cli list_packages \
  --args '{"owner":"OWNER","type":"container","page":1,"limit":30}'

${FORGEJO_MCP_BIN} --cli list_packages \
  --args '{"owner":"OWNER","type":"generic","q":"dist","page":1,"limit":30}'
```

Envelope: `{packages, page, limit, count, has_next, total_count?}`. `total_count` is
present only when Forgejo sends `X-Total-Count`. `has_next` is `Link; rel="next"`.
Each row is `id`, `type`, `name`, `version`, plus `html_url`, `created_at`, and
`repository` (the linked repo's `full_name`) when set. Nested owner/creator
users are not returned.

Container names may contain `/` (for example `library/app`). That is one
`name` segment, not a path.

## 3. Get one version (read-only)

```bash
${FORGEJO_MCP_BIN} --cli get_package \
  --args '{"owner":"OWNER","type":"container","name":"app","version":"1.0.0"}'
```

Same projected fields as a list row.

## 4. List files (read-only)

Forgejo returns every file; the tool slices with `page`/`limit` and
`has_next`. `total_count` is the fetched list length.

```bash
${FORGEJO_MCP_BIN} --cli list_package_files \
  --args '{"owner":"OWNER","type":"generic","name":"dist","version":"1.0.0","page":1,"limit":30}'
```

Envelope: `{files, page, limit, count, has_next, total_count}`. Each file is `id`,
`name`, `size`, `sha256`.

## 5. Delete one version (do not invoke)

`delete_package` DELETEs `/packages/{owner}/{type}/{name}/{version}` with no
GET beforehand. HTTP 204 is `{owner, type, name, version, status: "deleted"}`.
4xx/5xx stay errors. It removes **that version only**, not every version of
the name.

Do not invoke it against this demo's sample owner.

```bash
${FORGEJO_MCP_BIN} --cli delete_package --help
```
