# SPDX-License-Identifier: GPL-3.0-or-later

## Context

Forgejo edits repository properties with `PATCH /repos/{owner}/{repo}` (`EditRepoOption`: only set fields change). The SDK models every scalar as a pointer with `json:",omitempty"`. MCP booleans that are parsed with `v, _ := args["private"].(bool)` become `false` when omitted — that is correct for `create_repo` and fatal for edit (it would un-private a repo).

## Goals / Non-Goals

**Goals**

- `get_repo` and `edit_repo` as thin SDK wrappers.
- PATCH: omitted keys are not sent; explicit `false` is sent; empty `description`/`website` clears.
- Empty edit (only `owner`/`repo`) is an error with no upstream call.

**Non-Goals**

- Topics (`PUT /repos/{owner}/{repo}/topics` — separate API; `openspec/changes/repo-topics`).
- Merge-style cluster, internal/external tracker objects, mirror interval/prune, `globally_editable_wiki`, `wiki_branch`.
- Changing the `forgejo://repo/{owner}/{repo}` resource payload.

## Decisions

### D1: `edit_repo`, not `update_repo`

Newer PATCH tools in this repo are `edit_*` (`edit_release`, `edit_repo_label`, `edit_repo_hook`, `edit_org`). `update_*` is the older/content-replace family. Name the tool `edit_repo`.

### D2: `get_repo` ships in the same slice

Every recent `edit_*` has a get-one sibling. `list_my_repos` cannot read another owner's repo. The resource template does not include `website`. Expose `get_repo` wrapping `GetRepo`. Topics on that object come in the `repo-topics` slice (`Repository` in SDK v3 has no `Topics` field).

### D3: Optional fields via `ok` + `ptr.To`

```go
if v, ok := args["private"].(bool); ok {
    opt.Private = ptr.To(v)
}
```

`EditRepoOption` uses `omitempty` on pointers: a nil pointer is **omitted** from JSON (not sent as `null`). Tests MUST NOT assert JSON `null` the way branch-protection tests do (`EditBranchProtectionOption` lacks `omitempty`).

Do not copy `create_repo`'s `private, _ := args["private"].(bool)`.

### D4: Empty edit is rejected

If no optional argument key is present, return an error and do not call `EditRepo`. Same as `edit_repo_label`.

### D5: Empty string on `description` and `website` is a clear

If the caller supplies the key, send the pointer even when the value is `""`. Empty `name` or `default_branch` is ignored (do not rename to blank / unset the default branch by accident).

### D6: No `mcp.DefaultBool` on PATCH flags

A default would make clients always send a value and clobber server state.

### D7: Output bounding exempt

One `Repository` object. Tick the semantics-bounded box in the PR; do not add `page`/`limit`.

### D8: Field set is the flat settings page

In: `name`, `description`, `website`, `default_branch`, `private`, `template`, `archived`, `has_issues`, `has_wiki`, `has_pull_requests`, `has_projects`, `has_releases`, `has_packages`, `has_actions`.

Out: merge-style, tracker structs, mirror, wiki branch / globally editable wiki, topics.

## Risks / Trade-offs

- Org visibility changes can `422` if the token is not an owner — surface the SDK error.
- `name` renames the repo; the next call must use the new name. Document in the tool description.
