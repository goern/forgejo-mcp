# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [ ] 1.1 Implement projected DTOs and helpers in `operation/packages/` (SPDX header); do not import `operation/actions`
- [ ] 1.2 Implement `list_packages` with `type`/`q`/`page`/`limit` and envelope `{packages, page, limit, count, total_count?}`
- [ ] 1.3 Implement `get_package` (same projection as a list row)
- [ ] 1.4 Implement `delete_package` (no preflight; 204 → `status: "deleted"`)
- [ ] 1.5 Implement `list_package_files` (client-side page/limit; `{files, page, limit, count, has_next}`)
- [ ] 1.6 Register the four tools from `packages.RegisterTool`, `operation.RegisterPackagesTool`, and the CLI domain map

## 2. Tests

- [ ] 2.1 list: GET query `page`/`limit`/`type`/`q`; envelope keys present
- [ ] 2.2 list `X-Total-Count: 4` → `"total_count":4`; header absent → key omitted
- [ ] 2.3 list 404 owner → error, not empty success
- [ ] 2.4 list JSON `null` → `packages: []`, not `null`
- [ ] 2.5 fat upstream `owner` User → result has `html_url`, no nested `"login"`
- [ ] 2.6 get path encodes `/` in `name`; same projection
- [ ] 2.7 delete 204 → `status: "deleted"`; delete 404 → MCP error, DELETE still sent
- [ ] 2.8 files two items, `limit=1` → `has_next`; page 2 is the second file; no `md5`/`sha1`/`sha512`
- [ ] 2.9 missing owner / empty type on get → error, zero requests
- [ ] 2.10 owner with `/` → `APIPath` does not retarget
- [ ] 2.11 `page=0` → handler error

## 3. Wrap-up

- [ ] 3.1 README Packages subsection + CLI example + `demos/packages.md` + `demos/README.md` + `extension/manifest.json`
- [ ] 3.2 `make build` + `go test ./operation/packages/ ./pkg/forgejo/` + `scripts/ci/check-api-path-escaping.sh`
