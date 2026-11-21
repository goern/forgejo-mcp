# SPDX-License-Identifier: GPL-3.0-or-later

# Demo: list, get, and inspect package versions

Forgejo's built-in registry is owner-scoped. `GET /packages/{owner}` is
`SearchVersions`: each row is one **version**, not one package name. Optional
`type` and `q` filter; `page`/`limit` are sent to the server. File listing has
no paging on the API — the tool fetches the list and slices it. A repository's
Packages tab stays empty until the package name is linked.

Do **not** run `delete_package` against a registry you do not own. This
walkthrough shows `--help` for delete and the read-only list/get/files
surface against a fictional `OWNER`. Link/unlink live capture is section 6.

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
${FORGEJO_MCP_BIN} --cli list 2>/dev/null | grep -E "list_packages|get_package|delete_package|list_package_files|link_package|unlink_package"
```

```
  delete_package                           Delete one package version. Removes that version only, not every version of the name. No preflight GET. HTTP 204 returns {owner, type, name, version, status: "deleted"}; 4xx/5xx stay errors.
  get_package                              Get one package version (id, type, name, version, html_url, created_at, repository full_name). Does not embed owner or creator users.
  link_package                             Link a package name to a repository under the same owner. repo is the repository name, not owner/repo. Cross-owner is 404 upstream. Link is on the package name, not a version; every version of that name shares the repository. HTTP 201 returns {owner, type, name, repo, status: "linked"}; 4xx/5xx stay errors.
  list_package_files                       List files of one package version. Forgejo returns the full file list with no paging; page (default 1) and limit (default 30, maximum 50) slice it client-side. Returns {files, page, limit, count, has_next, total_count}. total_count is the fetched list length (the files endpoint has no X-Total-Count). Each file is id, name, size, sha256.
  list_packages                            List package versions for a user or org (Forgejo SearchVersions: one row per version, not one per name). Optional type and q filter; page (default 1) and limit (default 30, maximum 50) are sent as query parameters. Returns {packages, page, limit, count, has_next} and total_count when Forgejo sets X-Total-Count. has_next is Link rel=next. A missing owner is an error, not an empty list.
  unlink_package                           Unlink a package name from its repository. Unlink is on the package name, not a version. HTTP 204 returns {owner, type, name, status: "unlinked"}; 4xx/5xx stay errors.
```

```bash
${FORGEJO_MCP_BIN} --cli list_packages --help
${FORGEJO_MCP_BIN} --cli get_package --help
${FORGEJO_MCP_BIN} --cli list_package_files --help
${FORGEJO_MCP_BIN} --cli delete_package --help
${FORGEJO_MCP_BIN} --cli link_package --help
${FORGEJO_MCP_BIN} --cli unlink_package --help
```

`list_packages` takes `owner` and optional `type`, `q`, `page`, `limit`.
`get_package` / `delete_package` / `list_package_files` take `owner`, `type`,
`name`, `version`. Files add `page` and `limit`. `link_package` takes `owner`,
`type`, `name`, `repo` (same-owner repository name, not `owner/repo`).
`unlink_package` takes `owner`, `type`, `name`. Neither link tool takes
`version`.

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

## 6. Link and unlink a package name

Link is on the **name**, not a version. `repo` is the repository name under the
same owner. Cross-owner is 404 upstream ([forgejo#11483](https://codeberg.org/forgejo/forgejo/issues/11483)).
Do not delete versions as part of this walkthrough.

Host for this slice: `https://git.b4mad.industries` (Forgejo 17.0.2). Sections
2-5 above are the original `#543` capture and do not show link/unlink.

### 6a. `owner/repo` as `repo` is rejected with no HTTP

```bash
${FORGEJO_MCP_BIN} --cli link_package \
  --args '{"owner":"cnvr","type":"npm","name":"oss","repo":"cnvr/oss"}'
```

```output
Error: tool execution failed: repo must be the repository name under the same owner, not owner/repo, got "cnvr/oss"
```

Captured 2026-09-13 against this binary (no request is sent).

### 6b. Live link, get, unlink

Captured 2026-11-21 against Forgejo 17.0.2 with one disposable generic
package, `nesvet/forgejo-mcp-link-demo@0.0.1`, deleted afterwards. Each output
block is the `text` field of the tool result.

```bash
${FORGEJO_MCP_BIN} --cli get_package \
  --args '{"owner":"nesvet","type":"generic","name":"forgejo-mcp-link-demo","version":"0.0.1"}'
```

```output
{"Result":{"id":992,"type":"generic","name":"forgejo-mcp-link-demo","version":"0.0.1","html_url":"https://git.b4mad.industries/nesvet/-/packages/generic/forgejo-mcp-link-demo/0.0.1","created_at":"2026-11-21T17:08:16Z"}}
```

No `repository` key: the name is not linked yet.

```bash
${FORGEJO_MCP_BIN} --cli link_package \
  --args '{"owner":"nesvet","type":"generic","name":"forgejo-mcp-link-demo","repo":"forgejo-mcp"}'
```

```output
{"Result":{"owner":"nesvet","type":"generic","name":"forgejo-mcp-link-demo","repo":"forgejo-mcp","status":"linked"}}
```

```bash
${FORGEJO_MCP_BIN} --cli get_package \
  --args '{"owner":"nesvet","type":"generic","name":"forgejo-mcp-link-demo","version":"0.0.1"}'
```

```output
{"Result":{"id":992,"type":"generic","name":"forgejo-mcp-link-demo","version":"0.0.1","html_url":"https://git.b4mad.industries/nesvet/-/packages/generic/forgejo-mcp-link-demo/0.0.1","created_at":"2026-11-21T17:08:16Z","repository":"nesvet/forgejo-mcp"}}
```

```bash
${FORGEJO_MCP_BIN} --cli unlink_package \
  --args '{"owner":"nesvet","type":"generic","name":"forgejo-mcp-link-demo"}'
```

```output
{"Result":{"owner":"nesvet","type":"generic","name":"forgejo-mcp-link-demo","status":"unlinked"}}
```

A `get_package` after unlink returns the first output again, without
`repository`.

### 6c. Upstream refusals stay tool errors

Linking a name that is already linked, and unlinking one that is not, are
upstream 400s. The tool reports them as errors, not as `"linked"` or
`"unlinked"`:

```output
Error: tool execution failed: link package: POST https://git.b4mad.industries/api/v1/packages/nesvet/generic/forgejo-mcp-link-demo/-/link/forgejo-mcp: 400 Bad Request: {"message":"invalid argument","url":"https://git.b4mad.industries/api/swagger"}
Error: tool execution failed: unlink package: POST https://git.b4mad.industries/api/v1/packages/nesvet/generic/forgejo-mcp-link-demo/-/unlink: 400 Bad Request: {"message":"invalid argument","url":"https://git.b4mad.industries/api/swagger"}
```

A repository with the Packages unit disabled refuses the link with 403:

```output
Error: tool execution failed: link package: POST https://git.b4mad.industries/api/v1/packages/nesvet/generic/forgejo-mcp-link-demo/-/link/forgejo-mcp: 403 Forbidden: {"message":"permission denied","url":"https://git.b4mad.industries/api/swagger"}
```

`PathEscape` leaves `.` and `..` unchanged, and the HTTP client does not
normalise them away: `repo: ".."` reaches the link handler as a repository
name and is a 404, not a request one level up:

```output
Error: tool execution failed: link package: POST https://git.b4mad.industries/api/v1/packages/nesvet/generic/forgejo-mcp-link-demo/-/link/..: 404 Not Found: {"message":"repository does not exist [id: 0, uid: 39, owner_name: , name: ..]","url":"https://git.b4mad.industries/api/swagger"}
```
