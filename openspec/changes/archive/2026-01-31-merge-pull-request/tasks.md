## 1. Tool Definition and Handler

- [x] 1.1 Add `MergePullRequestToolName` constant and `MergePullRequestTool` variable in `operation/pull/pull.go` with required params (owner, repo, index, style) and optional params (title, message, delete_branch_after_merge, force_merge, merge_when_checks_succeed)
- [x] 1.2 Implement `MergePullRequestFn` handler in `operation/pull/pull.go` — extract params, build `MergePullRequestOption`, call `forgejo.Client().MergePullRequest()`, return success message (note scheduled merge if `merge_when_checks_succeed` is set)

## 2. Registration and Build

- [x] 2.1 Register `MergePullRequestTool` with `MergePullRequestFn` in `pull.RegisterTool()`
- [x] 2.2 Run `make build` and verify compilation succeeds
