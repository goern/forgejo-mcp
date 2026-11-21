# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Tools

- [x] 1.1 `operation/actions/variables.go`: five tools, `scope` checks before HTTP, list projection without `data`, `TotalCountPtr`
- [x] 1.2 Register the five tools from `RegisterTool`
- [x] 1.3 Reject `/`, `?`, and `#` in `name` before HTTP; empty `data` stays on SDK `Validate`

## 2. Tests

- [x] 2.1 Wrong coordinates, unknown scope, and a slashed name send no request
- [x] 2.2 List omits `data`; `total_count` follows the header; get returns `data`; create is POST and does not echo; update is PUT; delete returns `deleted`
- [x] 2.3 `gofmt` and `go test ./operation/actions/`

## 3. Docs

- [x] 3.1 README Actions rows and `extension/manifest.json`
- [x] 3.2 Archive this change in the same pull request
