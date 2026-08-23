# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [x] 1.1 Add `params.Website` and use it from `create_org`, `edit_org`, and `edit_repo`
- [x] 1.2 Implement `get_repo` via `Client.GetRepo` in `operation/repo/edit.go` (SPDX header); extract `loadRepo` for a later topics join
- [x] 1.3 Implement `edit_repo` with `ok` + `ptr.To` for every optional; reject empty edit; ignore empty `name` / `default_branch`; send empty `description` / `website`
- [x] 1.4 Register both tools from `repo.RegisterTool`

## 2. Tests

- [x] 2.1 description-only PATCH: `description` present; `private`/`archived` not a concrete boolean
- [x] 2.2 `private: false` → JSON `false`; `archived: true` → JSON `true`
- [x] 2.3 empty edit: error, zero PATCH
- [x] 2.4 `description: ""` → `"description": ""`
- [x] 2.5 `get_repo` hits `GET /repos/{owner}/{repo}` and returns `website`

## 3. Wrap-up

- [x] 3.1 README Repositories rows + `demos/edit-repo.md` + `demos/README.md` + `extension/manifest.json`
- [x] 3.2 `make build` + `go test ./operation/repo/ ./operation/org/`
