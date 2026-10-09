# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Types and helpers

- [x] 1.1 Add `labelDTO`, `labelCreateOption`, `labelPatchOption` in `operation/issue/label.go` and switch `ScopedLabel` to embed `labelDTO` by value; `go test` compiles
- [x] 1.2 Add `isScopedLabelName` and `requireExclusiveScopedName`; `exclusive=true` with `bug`, `/bug`, `kind/` errors with no HTTP; `kind/bug` is allowed

## 2. Create, edit, get, list

- [x] 2.1 Optional `exclusive` / `is_archived` on the four create/edit tools; PATCH via `ok` + `ptr.To`; empty edit includes the two flags in the “at least one” check
- [x] 2.2 Repo create/edit/get/list use `DoJSON` / `APIPath`; org uses the same option types; delete stays on the SDK
- [x] 2.3 Wire tests: color-only omits flags; `exclusive=false` and `is_archived=false` sent; exclusive-only and archived-only accepted; empty edit zero HTTP; create omit exclusive omits the key; create exclusive `kind/bug` POSTs `"exclusive":true`; org and repo share helpers; `t.Error` not `t.Fatal` after the request; owner `/` is `%2F`
- [x] 2.4 Get/list JSON contains `"exclusive"` and `"is_archived"` (including `false`); `go test ./operation/issue/` passes

## 3. Resources

- [x] 3.1 `labelResourcePayload` gains `exclusive` and `is_archived` without omitempty; repo single/list handlers use `DoJSON` and `headerHasMore`; org list projects the new fields
- [x] 3.2 Resource tests assert both keys on single and list payloads; existing paging tests still pass

## 4. Docs and demo

- [x] 4.1 README tool rows and `extension/manifest.json` descriptions mention the flags and the scoped-name rule
- [x] 4.2 Update `demos/label-management.md` for raw-HTTP + exclusive/archived; host is the Forgejo 16 instance actually used, not Codeberg
