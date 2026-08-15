<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `resource.ParseIssues` — parse `forgejo://repo/{owner}/{repo}/issues`, ignore the
      query string, reject the singular `…/issue/{index}` form.
- [x] `resource.ParseIssueComments` — parse
      `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments`, constrain kind to
      `issue|pr`, reject the singular `…/comment/{id}` form and non-numeric indices.
- [x] `repoIssuesResourceHandler` — row payload with no bodies; `state` (default
      `open`, unknown values fall back), comma-separated `labels`, `page`/`limit`
      through the shared `pageLimit` helper; page size equals the caller's limit
      exactly, with truncation taken from the response's `rel="next"` link; sentinel
      names `list_repo_issues`.
- [x] `issueCommentsResourceHandler` — full comment bodies, `page`/`limit`, sentinel
      names `list_issue_comments`; PR kind resolves through the issue-comment API.
- [x] `BoundedResult.WithMoreRemaining` — truncation reported by the server rather
      than inferred from an over-fetch, carrying the `X-Total-Count` total when the
      server sends one and inventing no total when it does not.
- [x] Register both templates in `RegisterIssueResources` **with their RFC 6570 query
      expansions in the template string**, with bound parameters and the cap stated in
      each description. Registering the bare path makes every query-bearing read fail
      to route before any handler runs.
- [x] Apply the page offset client-side in the comment thread when Forgejo returns the
      whole thread regardless of `page`/`limit`, and not when it honours them.
- [x] Unit tests for both handlers (mock Forgejo server, query-string assertions) and
      for both parsers.
- [x] Dispatch-level tests through `server.HandleMessage` with real `resources/read`
      envelopes, for a bare and a query-bearing URI on each resource — the handler
      tests construct their own request and so cannot see a routing failure.
- [x] Mutation-check the tests: ignoring `state`, restoring the `limit+1` page size,
      forcing "more exists" to false, dropping the `X-Total-Count` read, reverting
      either template registration to its bare path, and both directions of the
      client-side comment windowing each fail the suite.
- [x] Resource-table rows in `README.md` and `AGENTS.md`.
- [x] `go build ./...`, `go vet ./operation/...`, `go test ./operation/...` clean.
