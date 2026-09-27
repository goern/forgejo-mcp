## 1. Binary-safe repository write content

- [x] 1.1 Add a shared repository-write content selector based on argument presence rather than string value.
- [x] 1.2 Preserve existing plain-text behavior by Base64-encoding `content`.
- [x] 1.3 Accept strict RFC 4648 standard Base64 through `content_base64` and forward it unchanged.
- [x] 1.4 Require exactly one of `content` or `content_base64` for create/update.
- [x] 1.5 Preserve explicit empty text and explicit empty Base64 as valid zero-byte content.
- [x] 1.6 Extend `create_file` and `update_file` schemas and implementations with `content_base64`.
- [x] 1.7 Unit-test plain text, binary Base64, invalid Base64, ambiguous/missing representations, and zero-byte content.

## 2. Atomic multi-file write tool

- [x] 2.1 Add and register `change_files`.
- [x] 2.2 Accept owner, repo, message, source branch, optional new branch, and a non-empty file-operation array.
- [x] 2.3 Support create, update, and delete operations.
- [x] 2.4 Validate the complete batch before issuing the upstream write.
- [x] 2.5 Reject duplicate target paths.
- [x] 2.6 Require update/delete SHA and reject SHA on create.
- [x] 2.7 Apply the same `content` / `content_base64` rules to create/update and reject content on delete.
- [x] 2.8 Submit the validated batch through Forgejo's native repository contents endpoint in one request.
- [x] 2.9 Unit-test the native atomic request and invalid batch shapes.

## 3. Documentation and qualification

- [x] 3.1 Add `change_files` to the README file-tool table.
- [x] 3.2 Document binary-safe `content_base64` support on `create_file` and `update_file`.
- [x] 3.3 `go test ./operation/repo/...` passes.
- [x] 3.4 `go test ./...` passes.
- [x] 3.5 `go vet ./...` passes.
- [x] 3.6 `go build ./...` passes.
- [x] 3.7 `git diff --check` passes.
- [x] 3.8 OpenSpec strict validation passes.
