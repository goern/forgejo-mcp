# SPDX-License-Identifier: GPL-3.0-or-later

## Why

Forgejo 17 hangs the same action-variable routes on the user, an organization, and a repository: list, get, POST, PUT, and DELETE. forgejo-mcp can already list and write action secrets for a repository and an organization. It cannot read or write a variable at any of those three levels. A job still resolves an instance-global variable, and the REST API has no route for it, so that level stays out of the tool.

## What Changes

- Five tools, one `scope` argument (`user`, `org`, `repo`): `list_action_variables`, `get_action_variable`, `create_action_variable`, `update_action_variable`, `delete_action_variable`.
- List returns `name`, `owner_id`, and `repo_id` only, plus `{page, limit, count, total_count?}`. `total_count` is present only when `X-Total-Count` is set.
- Get returns `name`, `data`, `owner_id`, and `repo_id`. A missing name is an error.
- Create is POST and update is PUT, matching the Forgejo routes. `data` is the value. The SDK JSON field is `value`. Neither call echoes the value.
- A slash, question mark, or hash in `name` is rejected before any request. SDK v3.0.0 does not escape the name on org and repo create.
- README tool rows and `extension/manifest.json`. No new URI. No secret-tool rename.

## Capabilities

### New Capabilities

- `action-variables`: scoped list, get, create, update, and delete of Forgejo Actions variables.

### Modified Capabilities

- None.

## Impact

- **Affected code**: `operation/actions/variables.go`, `actions.go` (`RegisterTool`), `operation/params/params.go`, tests, README, `extension/manifest.json`.
- **APIs**: Forgejo `GET`/`POST`/`PUT`/`DELETE` on `/user/actions/variables`, `/orgs/{org}/actions/variables`, and `/repos/{owner}/{repo}/actions/variables`. SDK v3.0.0 already has the methods. No SDK upgrade.
- **Output bounding**: list is a page of entities (`page` default 1, `limit` default 30, maximum 50). The other four tools return one object (`docs/design/output-bounding.md`).
- **Not breaking**: new tool names. Existing secret tools stay.

## Out of Scope

- Replacing the six per-scope secret tools.
- Instance-global variables. There is no REST route; scraping `/admin/actions/variables` is not a tool.
- Runner registration tokens.
