<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `crossRepoArgs` helper resolves an optional owner/repo pair, defaulting
      to the target issue's own owner/repo when the argument is absent or
      empty.
- [x] `add_issue_dependency` accepts optional `depends_on_owner`/
      `depends_on_repo`, wired into the IssueMeta request body sent to
      Forgejo.
- [x] `remove_issue_dependency` accepts optional `dependency_owner`/
      `dependency_repo`, wired into the IssueMeta request body sent to
      Forgejo.
- [x] Self-dependency check scoped to same-repository: the same index in a
      different repository is not rejected.
- [x] Tool descriptions document the cross-repo arguments and their default.
- [x] Unit tests: cross-repo request body shape for add (resolved owner/repo
      reaches the body, not the target repo's), cross-repo same-index
      allowed.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` clean.
