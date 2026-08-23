# SPDX-License-Identifier: GPL-3.0-or-later

## Why

Forgejo repository topics live on `GET|PUT /repos/{owner}/{repo}/topics` and per-topic `PUT|DELETE /repos/{owner}/{repo}/topics/{name}`. They are **not** a field on `EditRepo`. `PUT /topics` is replace-all: sending an empty list wipes every tag.

No MCP tool exposes those endpoints. Putting `topics` on `edit_repo` would either omit-and-wipe or surprise reviewers. The pinned `forgejo-sdk/v3` already has `ListRepoTopics`, `SetRepoTopics`, `AddRepoTopic`, and `DeleteRepoTopic`. `GET /topics/search` has no SDK method and stays out of this change.

SDK `Repository` has no `Topics` field, so `GetRepo` drops tags. After this slice, `get_repo` (from `edit-repo`) SHALL join `ListRepoTopics` so a tool-only agent can see topics without a second call.

## What Changes

- Add MCP tools `list_repo_topics`, `set_repo_topics`, `add_repo_topic`, `delete_repo_topic`.
- `set_repo_topics` takes a required CSV string `topics`; missing key is an error with no PUT; empty CSV is an explicit clear.
- Normalise names (`TrimSpace` + `ToLower`), reject invalid names and more than 25 topics before any network call.
- Extend `get_repo` to include a `topics` array from `ListRepoTopics` (404 → empty array).
- README Topics subsection, `demos/repo-topics.md`, `extension/manifest.json`.

## Capabilities

### New Capabilities

- `repo-topics`: List, replace, add, and delete repository topics via existing SDK methods; join topics onto `get_repo`.

### Modified Capabilities

- None. `get_repo` is extended inside this change's `repo-topics` capability (a second SDK call). `edit_repo` is unchanged.

## Impact

- **Affected code**: `operation/repo/topics.go`, `loadRepo` in `edit.go`, README, demos, `extension/manifest.json`.
- **APIs / SDK**: `ListRepoTopics`, `SetRepoTopics`, `AddRepoTopic`, `DeleteRepoTopic`.
- **Output bounding**: `list_repo_topics` is a list (`page`/`limit`, envelope `{topics, page, limit, count}`); add/delete/set return a single object or success — exempt.
