## Why

Codeberg issue [#124](https://codeberg.org/goern/forgejo-mcp/issues/124) reports that `get_pull_request_diff` returns the full unified diff in a single response. Large PRs blow up the agent's context window, and the only mitigation today (truncating the response envelope) loses information without giving the agent a way to fetch the rest. The same unbounded-text problem applies to `get_file_content`, which returns whole files with no way to ask for a slice. Agents need bounded, addressable access to these payloads so they can decide what to read instead of paying the full cost up front.

## What Changes

- Add an optional `file_path` parameter to `get_pull_request_diff`. When set, return only the hunks for that file (sliced on `diff --git` boundaries). When omitted, behavior is unchanged.
- Add optional `start_line` and `end_line` parameters to `get_file_content`. When either is set, return a 1-indexed inclusive line slice instead of the full file. When both are omitted, behavior is unchanged. Out-of-range bounds clamp to the file extent rather than erroring.
- Document `get_pull_request_diff` and `list_pull_request_files` in the README tool table — currently both are missing.
- No breaking changes. All new parameters are optional and default to current behavior.

## Capabilities

### New Capabilities

- `bounded-pr-diff`: per-file slicing of the pull request diff so agents can request one file at a time, using `list_pull_request_files` as the index.
- `bounded-file-content`: line-range slicing of `get_file_content` so agents can read part of a large file.

### Modified Capabilities

None. The two existing tools (`get_pull_request_diff`, `get_file_content`) gain optional parameters but have no prior spec coverage in `openspec/specs/`, so the new capabilities define their bounded behavior from scratch.

## Impact

- **Code**: `operation/pull/pull.go` (`GetPullRequestDiffTool`, `GetPullRequestDiffFn`); `operation/repo/file.go` (`GetFileContentTool`, `GetFileContentFn`). Likely a small unified-diff splitter helper in `pkg/` (e.g. `pkg/diff/` — new) reusable by tests.
- **API surface**: three new optional MCP tool parameters (`file_path`, `start_line`, `end_line`). No SDK upgrade required — slicing is server-side post-fetch.
- **Docs**: README tool table updated with two missing entries plus the new parameters.
- **Tests**: unit tests for the diff splitter (multi-file fixture, file not in diff, single-file diff, binary diff) and for the line slicer (in-range, partial overflow, both bounds omitted, start>end rejection).
- **Out of scope** (deferred to follow-up issues if needed): byte-range paging on the raw diff, server-side default-limit tightening on `list_pull_review_comments` / `list_pull_reviews`, paging on `list_repo_commits`. The umbrella issue #124 will track these as siblings.
