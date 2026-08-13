<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `resource.ParseIssues` — parse `forgejo://repo/{owner}/{repo}/issues`, ignore the
      query string, reject the singular `…/issue/{index}` form.
- [x] `resource.ParseIssueComments` — parse
      `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments`, constrain kind to
      `issue|pr`, reject the singular `…/comment/{id}` form and non-numeric indices.
- [x] `repoIssuesResourceHandler` — row payload with no bodies; `state` (default
      `open`, unknown values fall back), comma-separated `labels`, `page`/`limit`
      through the shared `pageLimit` helper; `limit+1` fetch so `Bounded` can detect
      truncation; sentinel names `list_repo_issues`.
- [x] `issueCommentsResourceHandler` — full comment bodies, `page`/`limit`, sentinel
      names `list_issue_comments`; PR kind resolves through the issue-comment API.
- [x] Register both templates in `RegisterIssueResources`, with bound parameters and
      the cap stated in each description.
- [x] Unit tests for both handlers (mock Forgejo server, query-string assertions) and
      for both parsers.
- [x] Mutation-check the tests: ignoring `state` and dropping the `limit+1` fetch each
      fail the suite.
- [x] Resource-table rows in `README.md` and `AGENTS.md`.
- [x] `go build ./...`, `go vet ./operation/...`, `go test ./operation/...` clean.
