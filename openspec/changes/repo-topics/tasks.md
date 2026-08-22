# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [ ] 1.1 Implement `normalizeTopic` and `splitTopics` in `operation/repo/topics.go` (SPDX header); do not import `operation/issue`
- [ ] 1.2 Implement `list_repo_topics` with page/limit envelope `{topics, page, limit, count}`
- [ ] 1.3 Implement `set_repo_topics` (required CSV `topics`; empty = clear; missing key = error, no PUT)
- [ ] 1.4 Implement `add_repo_topic` and `delete_repo_topic`
- [ ] 1.5 Extend `loadRepo` to join `ListRepoTopics` (404 → empty `topics`); serialise a wrapper
- [ ] 1.6 Register the four tools from `repo.RegisterTool`

## 2. Tests

- [ ] 2.1 list: GET `.../topics`, envelope keys present
- [ ] 2.2 set `"go, MCP"` → PUT `{"topics":["go","mcp"]}`
- [ ] 2.3 set `""` → PUT `{"topics":[]}`
- [ ] 2.4 missing `topics` key → error, zero PUT
- [ ] 2.5 add PUT `.../topics/ci`; delete DELETE `.../topics/ci`
- [ ] 2.6 `"Bad Name"` → error, zero requests; 26 topics → error, zero PUT
- [ ] 2.7 `get_repo` hits GetRepo and ListRepoTopics; JSON contains `topics`

## 3. Wrap-up

- [ ] 3.1 README Topics subsection + `demos/repo-topics.md` + `demos/README.md` + `extension/manifest.json`
- [ ] 3.2 `make build` + `go test ./operation/repo/`
