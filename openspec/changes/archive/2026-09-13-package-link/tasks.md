# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [x] 1.1 Add `requiredPackageNameCoords` (owner, type, name — no version) and `params.PackageRepo` in one or two sentences
- [x] 1.2 Implement `link_package` in `operation/packages/link.go`: POST `APIPath("packages", owner, type, name, "-", "link", repo)`, `DoJSON` nil out, 201 → `{owner, type, name, repo, status: "linked"}`
- [x] 1.3 Implement `unlink_package`: POST `APIPath(..., "-", "unlink")`, 204 → `{owner, type, name, status: "unlinked"}`
- [x] 1.4 Reject `repo` containing `/` with no HTTP; register both tools from `packages.RegisterTool`

## 2. Tests

- [x] 2.1 Missing `owner` / `type` / `name` / `repo` on link: error, `capture.count == 0`
- [x] 2.2 Missing `owner` / `type` / `name` on unlink: error, zero requests
- [x] 2.3 `repo` contains `/`: error, zero requests
- [x] 2.4 `name` `youscore/core`: `escapedPath` contains `youscore%2Fcore` as one segment and `/-/link/`
- [x] 2.5 Owner containing `/` does not retarget (`acme%2Forg`)
- [x] 2.6 Link 201 empty (or non-JSON) body → `status` `"linked"`, `repo` echoed; unlink 204 → `"unlinked"`; `t.Error` not `t.Fatal` after the handler returned
- [x] 2.7 Link 404 package, link 404 repo, 403, already-linked 400: MCP error, not `"linked"`
- [x] 2.8 Unlink 404 and unlink-when-not-linked 400: MCP error, not `"unlinked"`
- [x] 2.9 After link, mocked `get_package` returns `repository` as `full_name`
- [x] 2.10 `go test ./operation/packages/` and `scripts/ci/check-api-path-escaping.sh` pass

## 3. Docs and demo

- [x] 3.1 README tool table two rows and CLI examples; `extension/manifest.json` two tools
- [x] 3.2 Append a link/unlink section to `demos/packages.md`; update `demos/README.md` index; host is the instance actually used, or `--help` plus honest gaps
