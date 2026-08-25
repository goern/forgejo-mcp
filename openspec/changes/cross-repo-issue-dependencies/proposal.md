<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

Forgejo's IssueMeta dependency body carries `owner`/`repo`/`index` precisely
so a dependency can name an issue in a different repository, but
`add_issue_dependency` and `remove_issue_dependency` hardcoded the body's
`owner`/`repo` to the target issue's own repository. That made a cross-repo
dependency — "this issue blocks on that issue over in another repo" —
unreachable through these tools even though the underlying API supports it,
and even though `list_issue_dependencies`/`list_issue_dependents` already
return dependency issues regardless of which repository they live in.

## What Changes

- `add_issue_dependency` gains optional `depends_on_owner`/`depends_on_repo`
  arguments, defaulting to the target issue's own `owner`/`repo`, naming the
  repository the dependency issue lives in.
- `remove_issue_dependency` gains the equivalent optional
  `dependency_owner`/`dependency_repo` arguments for the dependency being
  removed.
- Both resolve through a shared `crossRepoArgs` helper and wire the resolved
  owner/repo into the IssueMeta request body actually sent to Forgejo, rather
  than the target repository's own owner/repo.
- The self-dependency check ("an issue cannot depend on itself") is scoped to
  same-repository: the same index in a *different* repository is a distinct
  issue, not a self-dependency, and adding it SHALL succeed.

No new tool is added; `list_issue_dependencies` and `list_issue_dependents`
are unchanged — they already return whatever repository the dependency issue
resolves to.

## Capabilities

### New Capabilities

- **Issue dependency management**: list, add, and remove dependency/blocking
  relationships between issues, including a dependency that lives in a
  different repository than the issue it blocks.

## Impact

- **Code**: `operation/issue/dependency.go` — `crossRepoArgs`,
  `AddIssueDependencyFn`, `RemoveIssueDependencyFn`.
- **Tests**: `operation/issue/dependency_test.go` — cross-repo request body
  shape for add, cross-repo same-index allowed (not treated as
  self-dependency), alongside the pre-existing same-repo coverage.
- **APIs**: one POST (add) or DELETE (remove) per call, the same endpoints
  these tools already use; the request body now carries the resolved
  dependency owner/repo instead of always the target repo's.
- **Dependencies**: none added.
- **Risk**: low — additive optional arguments, defaulting to the prior
  same-repo behavior when omitted.
