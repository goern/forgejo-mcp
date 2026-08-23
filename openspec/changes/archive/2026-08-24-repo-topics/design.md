# SPDX-License-Identifier: GPL-3.0-or-later

## Context

Topics are a separate REST surface from `PATCH /repos/{owner}/{repo}`. Mixing them into `edit_repo` would make an omitted or empty `topics` key wipe tags. The MCP Go SDK in this repo has no `mcp.WithArray`; list arguments are CSV strings (`assignees`, `status_check_contexts`). CLI `--args` with a JSON array without `WithArray` fails a type assert.

Forgejo topic names: `^[a-z0-9][-.a-z0-9]*$`, at most 35 characters, at most 25 per repository. The API lowercases on write; the SDK does not.

## Goals / Non-Goals

**Goals**

- Four tools wrapping the four SDK methods.
- CSV for `set_repo_topics`; key required; empty CSV = clear.
- Client-side name normalisation and validation before network.
- `get_repo` includes `topics` after this slice.

**Non-Goals**

- `search_topics` / `GET /topics/search` (no SDK method).
- `mcp.WithArray`.
- Topics as an `edit_repo` argument.
- Changing `forgejo://repo/{owner}/{repo}`.

## Decisions

### D1: Four tools, not a field on `edit_repo`

`list_repo_topics`, `set_repo_topics`, `add_repo_topic`, `delete_repo_topic`. Replace-all stays an explicit `set_*` so an agent cannot "forget a field and wipe tags".

### D2: CSV string; missing key is an error

`set_repo_topics` requires `topics` as a string. Split on commas. Empty string after split → `SetRepoTopics(owner, repo, []string{})`. Absent key → error, no PUT.

### D3: Normalise and validate on the MCP boundary

`normalizeTopic`: `TrimSpace`, `ToLower`, match `^[a-z0-9][-.a-z0-9]*$`, length 1–35. `splitTopics`: CSV, each token normalised, de-duped, at most 25. Invalid or too many → error before any SDK call.

### D4: `search_topics` is out of scope

No SDK `SearchTopics`. Do not add raw `DoJSON` for this slice.

### D5: `get_repo` joins `ListRepoTopics`

`loadRepo` calls `GetRepo` then `ListRepoTopics` and serialises a wrapper with a `topics` array. If `ListRepoTopics` returns 404, use an empty array (repo without the feature / older instance) and still return the repository.

### D6: List uses the bounding envelope

`list_repo_topics` accepts `page` (default 1) and `limit` (default 100) and returns `{topics, page, limit, count}` like `list_repo_labels`. Forgejo caps at 25 topics; page/limit still exist for the bounding contract.

## Risks / Trade-offs

- CSV cannot express a topic containing a comma; Forgejo names cannot contain commas either.
- Joining topics on `get_repo` adds a second round-trip; cheaper than a silent empty `Topics` field that sends agents to raw HTTP.
