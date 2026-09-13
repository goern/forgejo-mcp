# SPDX-License-Identifier: GPL-3.0-or-later

## Context

See proposal.md Why. Capability `packages` is already archived inside `#543`. That change listed link/unlink as a non-goal. This change is Modified `packages`, so it lands after `#543` merges or stacks on `feat/packages`. It does not reopen `#543` nits (`has_next`, `total_count`, clamp).

Link is on the package **name**. The Forgejo path has no version segment. Putting `version` on the tool would be a lie. `repo` is looked up with `GetRepositoryByName(ctx, ctx.ContextUser.ID, repo_name)` — same-owner name, not `owner/repo`. A slash in `repo` would be a repo whose name is `owner/repo` and 404; reject at the MCP boundary.

## Goals / Non-Goals

**Goals:**

- Two tools for the two Settings buttons.
- Name-scoped coords (owner, type, name). No dummy version.
- `APIPath` with the `"-"` segment so `/-/` is not a path injection.
- 201/204 empty bodies via `DoJSON(..., nil)`.
- `/` in `repo` is an error with no HTTP.
- 4xx/5xx stay errors; no `"already_linked"` / `"already_unlinked"`.

**Non-Goals:**

- Folding into `#543`.
- Changing get/list projection, paging, or `has_next`.
- A `get_package_link` tool.
- Container autolink.
- Repo-scoped package list.
- Resource URIs.

## Decisions

### D1: Modified `packages`, not a new capability

This is the Settings page of the same registry. A new capability would imply a different family. Land after `#543` or stack; do not push link commits into `#543`.

### D2: Two tools, no version

`link_package` and `unlink_package`. Link is on the name; every version shares the repository. Same discipline as `#533` (two buttons, two endpoints).

### D3: `repo` is a same-owner name

Tool description says so in one sentence. `params.PackageRepo` is one or two sentences. A `repo` containing `/` is invalid-params, zero HTTP. Cross-owner is upstream 404; do not rewrite it into a guessed “wrong owner” string.

### D4: `APIPath` and the `"-"` segment

```
forgejo.APIPath("packages", owner, packageType, name, "-", "link", repo)
forgejo.APIPath("packages", owner, packageType, name, "-", "unlink")
```

Literal `/` in `name` (npm scopes, container paths) is `%2F` as one segment. The `/-/` in the Forgejo path is the segment `"-"`.

### D5: Empty 201/204, `DoJSON` nil out

`DoJSON(ctx, method, path, nil, nil)` already treats 2xx including 201 with empty body as success (`out == nil` skips decode). Do not decode a Forgejo body. Success envelopes are local:

- link: `{owner, type, name, repo, status: "linked"}`
- unlink: `{owner, type, name, status: "unlinked"}`

Do not return those statuses on 404 or 403. Copy `#543` delete.

### D6: Already-linked and unlink-when-not-linked are 4xx

Forgejo/Gitea `LinkToRepository` returns `ErrInvalidArgument` when `pkg.RepoID > 0` (handler 400). `UnlinkFromRepository` returns `ErrInvalidArgument` when `pkg.RepoID == 0` (handler 400). Swagger lists unlink 201; the handler sends 204 on success. This change maps 2xx to the success object and every 4xx/5xx to an MCP error. It does not invent `"already_linked"`. Live instance was not probed this session (no token); these codes are pinned from the upstream handler, not from git.b4mad.industries.

### D7: Read-back is `get_package`

After a successful link, `get_package` on any version of that name includes `repository` as `full_name` (existing projector, `omitempty` when absent). No new field. No new get tool.

### D8: Output bounding exempt

Success is a small fixed object. No `page`/`limit`.

### D9: Autolink is out of scope

Container autolink is publish-time behaviour (image name or OCI label). This change is the Settings button.

## Risks / Trade-offs

- [Agents pass `owner/repo` as `repo`] → Mitigation: reject `/` at MCP with a message that names the rule.
- [Already-linked is 400, not 201] → Mitigation: pass through; do not invent a third status. Documented.
- [Swagger unlink 201 vs handler 204] → Mitigation: match the handler and `#543` delete (204 empty).
- [Stack on `#543`] → Mitigation: branch from `feat/packages`; open against `main` only after rebase.

## Migration Plan

Additive. Existing package tools unchanged. After merge, an agent that sees a missing `repository` key can call `link_package`.

## Open Questions

None. Live table on the PR stays empty until a token exists; 400 for already-linked / not-linked is pinned from the upstream handler.
