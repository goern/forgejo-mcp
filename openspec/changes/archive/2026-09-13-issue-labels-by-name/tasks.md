# SPDX-License-Identifier: GPL-3.0-or-later

## 1. Resolver

- [x] 1.1 Add `listAssignableLabels` in `operation/issue/labelresolve.go` (SPDX header): page repo labels and org labels independently at 100 per page until a short page, capped by `maxLabelCatalogPages`; org 404 is empty, org 401/403 is an error
- [x] 1.2 Add `resolveIssueLabelIDs`: name match first, then numeric id present in the catalogue; duplicates collapse; unknown token, cross-scope collision, and empty-after-trim are errors with no upstream write

## 2. Tools

- [x] 2.1 `create_issue`: optional `labels`, `assignees`, `milestone` on the schema; resolve labels to ids and set `CreateIssueOption.Labels`, `Assignees`, `Milestone`; no catalogue read when `labels` is absent
- [x] 2.2 `add_issue_labels` and `remove_issue_labels`: resolve through `resolveIssueLabelIDs`; update both descriptions to say names or ids
- [x] 2.3 `update_issue`: optional `set_labels`; empty clears via `ClearIssueLabels`, otherwise `ReplaceIssueLabels`; skip the PATCH when `set_labels` is the only argument; re-read the issue after a label mutation

## 3. Tests

- [x] 3.1 Resolver: repo name, org name, numeric token, label named `123`, second-page label, unknown token, cross-scope collision, org 401
- [x] 3.2 `create_issue`: names become ids in the POST body; `assignees` and `milestone` in the body; unknown name sends no POST; bare create issues exactly one request; non-numeric milestone rejected
- [x] 3.3 `add_issue_labels` by name and by id; `remove_issue_labels` by name hits `/labels/{id}`; empty CSV is an error with zero requests
- [x] 3.4 `update_issue`: `set_labels` PUTs ids and the PATCH body carries no `labels`; empty `set_labels` deletes the collection without reading the catalogue; `set_labels` alone sends no PATCH; `title` plus `set_labels` patches then replaces; no `set_labels` still sends exactly one PATCH

## 4. Wrap-up

- [x] 4.1 README Issues table rows for `create_issue`, `add_issue_labels`, `remove_issue_labels`, `update_issue`, and the two label-list rows that point at ids
- [x] 4.2 `AGENTS.md` label example: names instead of numeric ids
- [x] 4.3 `extension/manifest.json` descriptions for the four tools
- [x] 4.4 `make build`, `go vet ./...`, `gofmt -l`, `go test ./operation/issue/`, `scripts/ci/openspec-validate.sh`
