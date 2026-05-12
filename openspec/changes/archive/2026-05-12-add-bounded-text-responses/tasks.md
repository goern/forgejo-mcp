## 1. Diff splitter helper

- [ ] 1.1 Create new package `pkg/diff/` with `splitter.go` exposing `FileSlice(rawDiff, filePath string) (slice string, found bool)`. Match on `diff --git a/<path>` or `b/<path>`. Section ends at the next `diff --git` line or end of input.
- [ ] 1.2 Add `pkg/diff/splitter_test.go` with table-driven cases: multi-file diff, single-file diff, binary-file section, renamed-file (match on either side), file not in diff, empty diff, diff that does not start at a `diff --git` line (defensive).

## 2. Bound `get_pull_request_diff` by `file_path`

- [ ] 2.1 Add `file_path` string parameter (optional, no default) to `GetPullRequestDiffTool` in `operation/pull/pull.go`. Update tool description to document both modes and the "renamed file matches either side" rule.
- [ ] 2.2 Update `GetPullRequestDiffFn`: after the SDK call, if `file_path` is set and non-empty, call `diff.FileSlice` and return either the slice or an error result naming the missing path. If empty, return the full diff (current behavior).

## 3. Bound `get_file_content` by line range

- [ ] 3.1 Add optional `start_line` and `end_line` number parameters to `GetFileContentTool` in `operation/repo/file.go`. Update tool description to state 1-indexed inclusive bounds, clamping, the omit-defaults, and the `with_metadata=true` exception.
- [ ] 3.2 Update `GetFileContentFn` to apply the slice on the plain-text path only. Implement clamping (`start<1`→1, `end>count`→count) and inverted-range error. Leave the `with_metadata=true` branch untouched.

## 4. Tests

- [ ] 4.1 Add tests for `GetPullRequestDiffFn` with `file_path`: success path (slice returned), missing-path path (error), omitted-path path (full diff returned unchanged). Use the existing pull test backend pattern (or extend it) to serve a multi-file diff fixture.
- [ ] 4.2 Add tests for `GetFileContentFn` with line-range params: in-range slice, head (no start), tail (no end), clamping (start<1, end>count), inverted-range error, omit-both returns full content, `with_metadata=true` ignores slicing.

## 5. Documentation

- [ ] 5.1 In `README.md` tool table, add the previously-missing entries for `get_pull_request_diff` and `list_pull_request_files`, with the new `file_path` parameter mentioned on the diff entry.
- [ ] 5.2 In the `get_file_content` row, mention the new `start_line`/`end_line` parameters.
- [ ] 5.3 Cross-link `docs/design/output-bounding.md` from the change's archive note when the PR lands so future tool authors can see this change as a worked example of the rule.

## 6. Verification

- [ ] 6.1 `go test ./...` passes.
- [ ] 6.2 `make build` passes.
- [ ] 6.3 `openspec validate add-bounded-text-responses --strict` passes.
- [ ] 6.4 Manual smoke against a real Codeberg PR: `get_pull_request_diff` with no `file_path` returns the whole diff; with `file_path` pointing at a known modified file returns just that section.
- [ ] 6.5 Manual smoke against a large file (e.g. README): `get_file_content` with `start_line=10`, `end_line=20` returns 11 lines.
- [ ] 6.6 Update bd `forgejo-mcp-zwr` notes with smoke outcomes; close once PR merged.
