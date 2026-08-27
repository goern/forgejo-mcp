# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [x] 1.1 Export `resource.ValidateSHA` and call it from `ParseCommit` / `ParseStatus`
- [x] 1.2 Implement `get_commit_statuses` in `operation/repo/statuses.go` (SPDX header); envelope `{sha, statuses, page, limit, count}`; items share `statusItem`
- [x] 1.3 Register the tool from `repo.RegisterTool` next to `list_repo_commits`

## 2. Tests

- [x] 2.1 Short SHA → error, zero requests
- [x] 2.2 Empty list → envelope `count=0`, `statuses` is `[]` not null
- [x] 2.3 Success page: GET `…/commits/{sha}/statuses?page=&limit=`, item field `state`
- [x] 2.4 404 → error
- [x] 2.5 `page=2` is forwarded in the query

## 3. Wrap-up

- [x] 3.1 README Commits row after `list_repo_commits` + `demos/mcp-resource-templates.md` §5 + `extension/manifest.json`
- [x] 3.2 `make build` + `go test ./operation/repo/`
