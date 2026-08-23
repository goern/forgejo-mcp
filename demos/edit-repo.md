# SPDX-License-Identifier: GPL-3.0-or-later

# Demo: get and edit repository settings

Read one repository and PATCH its settings page (description, website,
visibility, unit toggles) without falling back to raw HTTP. Topics are a
separate API and are not a field of `edit_repo`.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-./forgejo-mcp}"
make build
```

Spec: `openspec/changes/edit-repo/specs/repo-settings/spec.md`

## 1. Tool surface

```bash
${FORGEJO_MCP_BIN} --cli list 2>/dev/null | grep -E "get_repo|edit_repo"
```

```bash
${FORGEJO_MCP_BIN} --cli get_repo --help 2>/dev/null
${FORGEJO_MCP_BIN} --cli edit_repo --help 2>/dev/null
```

## 2. Read then PATCH one field

`get_repo` wraps `GET /repos/{owner}/{repo}` and includes `description` and
`website`. `edit_repo` sends only the keys you pass.

```bash
${FORGEJO_MCP_BIN} --cli get_repo --args '{"owner":"OWNER","repo":"REPO"}'
```

```bash
${FORGEJO_MCP_BIN} --cli edit_repo --args '{"owner":"OWNER","repo":"REPO","website":"https://example.com","description":"after edit"}'
```

The PATCH body contains `website` and `description`. It does not contain a
concrete `private` or `archived` boolean, so visibility and archive state stay
as they were on the server.

## 3. Explicit false is sent; empty string clears

```bash
${FORGEJO_MCP_BIN} --cli edit_repo --args '{"owner":"OWNER","repo":"REPO","private":false}'
```

```bash
${FORGEJO_MCP_BIN} --cli edit_repo --args '{"owner":"OWNER","repo":"REPO","description":""}'
```

`private: false` is JSON `false`, not omitted. `description: ""` clears the
description.

## 4. Empty edit is rejected

```bash
${FORGEJO_MCP_BIN} --cli edit_repo --args '{"owner":"OWNER","repo":"REPO"}'
```

This returns an error and does not call Forgejo. That is the guard against an
agent wiping settings by sending a PATCH with every optional defaulted to
false.

## End-to-end

An agent that needs to put a homepage on an existing repo: `get_repo` to see
the current website, `edit_repo` with only `website`, `get_repo` again to
confirm. No `curl`, no topics field.
