# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [x] 1.1 Implement `normalizeTopic` and `splitTopics` in `operation/repo/topics.go` (SPDX header); do not import `operation/issue`
- [x] 1.2 Implement `list_repo_topics` with page/limit envelope `{topics, page, limit, count}`
- [x] 1.3 Implement `set_repo_topics` (required CSV `topics`; empty = clear; missing key = error, no PUT)
- [x] 1.4 Implement `add_repo_topic` and `delete_repo_topic`
- [x] 1.5 Extend `loadRepo` to join `ListRepoTopics` (404 → empty `topics`); serialise a wrapper
- [x] 1.6 Register the four tools from `repo.RegisterTool`

## 2. Tests

- [x] 2.1 list: GET `.../topics`, envelope keys present
- [x] 2.2 set `"go, MCP"` → PUT `{"topics":["go","mcp"]}`
- [x] 2.3 set `""` → PUT `{"topics":[]}`
- [x] 2.4 missing `topics` key → error, zero PUT
- [x] 2.5 add PUT `.../topics/ci`; delete DELETE `.../topics/ci`
- [x] 2.6 `"Bad Name"` → error, zero requests; 26 topics → error, zero PUT
- [x] 2.7 `get_repo` hits GetRepo and ListRepoTopics; JSON contains `topics`

## 3. Wrap-up

- [x] 3.1 README Topics subsection + `demos/repo-topics.md` + `demos/README.md` + `extension/manifest.json`
- [x] 3.2 `make build` + `go test ./operation/repo/`
