# SPDX-License-Identifier: GPL-3.0-or-later

## Context

Forgejo `addActionsRoutes` registers variables at three owners. SDK v3.0.0 (`user_action.go`, `org_action.go`, `repo_action.go`) already calls them. The secret tools in this repo are six names, two owners, and one `PUT`. Variables are a different wire: list rows include `data`, and create is `POST` while update is `PUT`.

## Goals / Non-Goals

**Goals:**

- One scope argument for all three owners.
- A list page that does not carry values.
- Create and update as two tools, because the HTTP methods differ.
- `total_count` only when the header exists.

**Non-Goals:**

- A global variable tool.
- A breaking rename of the secret tools.

## Decisions

### D1: `scope` instead of fifteen names

`user`, `org`, and `repo` are one required argument. `org` requires `org`. `repo` requires `owner` and `repo`. Any other coordinate for that scope is an error before HTTP. An unknown scope is the same.

Fifteen names would teach the agent a grid. The secret tools already show what that grid costs, and a later change can fold them onto this argument.

### D2: Five tools, not one write

`Create*ActionVariable` is POST and returns 201 or 204. `Update*ActionVariable` is PUT and returns 200 or 204. A missing name on update is not found. A duplicate name on create is a conflict. One `PUT` tool would hide that.

### D3: SDK, not a second raw client

Link and unlink stayed on `DoJSON` because the SDK has no method. These fifteen SDK methods exist and already escape path segments on get, update, delete, and user create. Raw HTTP here would duplicate `ListOptions` and `CreateVariableOption.Validate`.

### D4: List rows drop `data`

`ActionVariable` includes `data`. The list tool copies only `name`, `owner_id`, and `repo_id`, the same idea as `actionSecretSummary`. The value is `get_action_variable`.

### D5: Reject `/`, `?`, and `#` in `name` before HTTP

`escapeValidatePathSegments` percent-encodes a segment. `CreateOrgActionVariable` and `CreateRepoActionVariable` in SDK v3.0.0 interpolate `opt.Name` without that helper, so `a/b` becomes another path. User create does escape. The tool rejects those three characters for every scope, so org and repo create cannot retarget. Empty `data` is left to `CreateVariableOption.Validate`, which returns before the client sends.

### D6: No global level

`models/actions/variable.go` allows `OwnerID` and `RepoID` both zero, and the admin HTML edits that row. `routers/api/v1` does not. The five descriptions and the README say so. A job still sees the global value on its own.

### D7: `total_count` is the header or absent

`ListVariables` calls `SetTotalCountHeader`. The envelope uses `pkg/forgejo.TotalCountPtr`. A missing or unparsable header omits the key. `count` is the page length.

## Risks / Trade-offs

- [SDK create for org and repo still interpolates the name] → The tool rejects the dangerous characters first. A name that is only odd in some other way still goes through the SDK escape on get, update, and delete.
- [Callers learn `scope` here and per-scope names for secrets] → Secrets are a separate breaking change. This PR does not rename them.

## Migration Plan

Additive. No tool name is removed. Rollback is reverting the five registrations.
