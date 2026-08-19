<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `pkg/forgejo.TotalCount` / `TotalCountPtr` — parse `X-Total-Count` from an
      `*http.Response`; report absent/unparsable as "no value" rather than `0`.
- [x] `search_issues` envelope gains `total_count` (omitempty); remove the
      "SHALL NOT report a total" line from `search-issues/spec.md` (moves to
      MODIFIED delta here).
- [x] `list_wiki_pages` / `get_wiki_revisions` envelopes gain `total_count`;
      `list_wiki_pages` documents its value as an upper bound (upstream counts
      raw tree entries), and `get_wiki_revisions` reads the body `count` field
      rather than the header.
- [x] `list_repo_hooks` envelope gains `total_count`.
- [x] Confirm which endpoints actually send `X-Total-Count` before adding the
      field: `list_branch_protections`, `list_issue_dependencies` and
      `list_issue_dependents` are excluded because their upstream handlers
      never call `SetTotalCountHeader`.
- [x] `operation/issue/resources_list.go`'s `headerTotalCount` delegates to the
      shared helper instead of duplicating the header parse.
- [x] Unit tests per tool: header present (value flows through), header absent
      (key omitted, not `0`), header unparsable (key omitted).
- [x] `pkg/forgejo/pagination_test.go` covers the helper directly: valid
      integer, missing header, non-numeric value, negative value.
- [x] README.md tool table rows updated for the tools that gained the field.
- [x] `docs/design/output-bounding.md` sub-rule 3 documents the convention
      once, naming the shared helper and the omit-on-absent rule.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` clean.
