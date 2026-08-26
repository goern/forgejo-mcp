# SPDX-License-Identifier: GPL-3.0-or-later

# Demo: repository topics

Topics live on `GET|PUT /repos/{owner}/{repo}/topics` and per-topic
`PUT|DELETE …/topics/{name}`. They are **not** a field of `edit_repo`:
`PUT /topics` is replace-all, so an omitted or empty field on a PATCH would
wipe every tag.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"
make build
```

Spec: `openspec/changes/repo-topics/specs/repo-topics/spec.md`

## 1. Tool surface

```bash
${FORGEJO_MCP_BIN} --cli list 2>/dev/null | grep -E "repo_topic"
```

## 2. Replace, list, add, delete

`set_repo_topics` takes a **CSV string**, not a JSON array. Names are
trimmed and lowercased. An empty string is an explicit clear.

```bash
${FORGEJO_MCP_BIN} --cli set_repo_topics --args '{"owner":"OWNER","repo":"REPO","topics":"go, MCP"}'
```

That sends `{"topics":["go","mcp"]}`. Then:

```bash
${FORGEJO_MCP_BIN} --cli list_repo_topics --args '{"owner":"OWNER","repo":"REPO"}'
```

The list tool returns `{topics, page, limit, count}` (defaults page 1, limit
100). Forgejo itself caps a repo at 25 topics.

```bash
${FORGEJO_MCP_BIN} --cli add_repo_topic --args '{"owner":"OWNER","repo":"REPO","topic":"forgejo"}'
${FORGEJO_MCP_BIN} --cli delete_repo_topic --args '{"owner":"OWNER","repo":"REPO","topic":"go"}'
```

`get_repo` includes a `topics` array (it calls `ListRepoTopics` as well as
`GetRepo`), so a tool-only agent does not need a second round-trip to see tags.

## 3. Guards that never hit the network

```bash
${FORGEJO_MCP_BIN} --cli set_repo_topics --args '{"owner":"OWNER","repo":"REPO"}'
```

Missing `topics` is an error — no PUT — so an agent cannot forget the field
and wipe tags. `"Bad Name"` and more than 25 names are rejected the same way.

```bash
${FORGEJO_MCP_BIN} --cli set_repo_topics --args '{"owner":"OWNER","repo":"REPO","topics":""}'
```

Empty CSV **does** PUT `{"topics":[]}`: that is the explicit clear.

## End-to-end

Tag a scratch repo, confirm with `list_repo_topics` and `get_repo`, add one
more name, delete one, leave the rest. Do not pass `topics` to `edit_repo`.
