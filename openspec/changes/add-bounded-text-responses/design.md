## Context

Two MCP tools today return arbitrarily large text payloads with no caller-side bound:

- `get_pull_request_diff` returns the SDK's unified diff for the entire PR in one shot. For a PR touching 30 files with 200 LOC each, the response easily exceeds 30 kB and pushes the model into truncation territory or context blow-up. The SDK call (`forgejo.Client().GetPullRequestDiff`) accepts no per-file or paging options.
- `get_file_content` returns full files via the SDK `/raw/` endpoint. A 5000-line generated source file is the same shape as a 50-line one to the tool.

The architectural rule for these tools is already codified in [`docs/design/output-bounding.md`](../../../docs/design/output-bounding.md): every data-proportional response MUST be bounded by the caller. Diffs bound by *per-file slice* (file_path), files bound by *line range* (start_line/end_line). Issue [#124](https://codeberg.org/goern/forgejo-mcp/issues/124) is the reporter case for both.

The change is constrained:

- No SDK upgrade. The slicing happens server-side after the SDK call.
- Backwards compatible. New params are optional; default behavior preserved.
- No dependency on caching or session state; slicing is per-call deterministic.

## Goals / Non-Goals

**Goals:**

- Let callers request a single file's hunks from a PR diff via a `file_path` parameter.
- Let callers request a 1-indexed inclusive line range from a file via `start_line` + `end_line`.
- Provide a small reusable helper for unified-diff per-file slicing so the rule can extend to any future diff tool (commit diff, branch compare) without copy-paste.
- Land docs that surface the new params in the README's tool table and patch the two existing tools that were previously absent from the table.

**Non-Goals:**

- Byte-range paging on raw diff bytes (mentioned in proposal as out-of-scope). The `diff --git` boundary is the natural unit; bytes are a last resort.
- Server-side caps on `list_pull_review_comments`, `list_pull_reviews`, `list_repo_commits` (separate follow-up tracked by #124 umbrella).
- Cross-tool result caching or session state.
- Changing the *plain text* vs *with_metadata* contract on `get_file_content`. The slice applies only to the plain-text path; metadata mode returns the SDK's `ContentsResponse` unchanged because that response carries base64 content that the slicer cannot meaningfully cut without breaking the encoding.
- Pagination on the diff (multi-page slicing). One file at a time is the unit.

## Decisions

### D1. Slice diffs on `diff --git` lines

A unified diff produced by `git diff` (or Forgejo's equivalent) starts each file section with a `diff --git a/<path> b/<path>` header. Every subsequent line up to the next `diff --git` (or EOF) belongs to that file. This is the standard, well-defined boundary used by every patch-handling tool.

Alternatives considered:

- Parse the diff into a structured AST (e.g. via a third-party `go-diff` lib). Rejected: adds a dependency and a runtime cost for a use case that is fundamentally a string split.
- Split on `--- a/<path>` / `+++ b/<path>` lines. Rejected: `diff --git` is the unambiguous outer boundary; `---` / `+++` are inner markers that also appear inside hunks.

### D2. `pkg/diff/` for the splitter

Place the helper in a new package `pkg/diff/` so:

- `operation/pull/` and any future diff consumer (e.g. a commit-diff tool) import it without circular dependency.
- It is unit-testable in isolation against fixtures without spinning up a Forgejo backend.

Single export: `FileSlice(rawDiff, filePath string) (slice string, found bool)`. Two return values keep the contract simple: callers decide whether "not found" is an error (it is for `get_pull_request_diff`) or empty (it would be for a hypothetical future caller).

### D3. File path matching is exact and post-rename-aware

The `diff --git` line carries both the pre- and post-rename path (`a/old` and `b/new`). The slicer SHALL match on either side. Reasoning: the natural caller flow is `list_pull_request_files` → pick a `filename` → pass it as `file_path`. The `list_pull_request_files` response gives the post-rename path; renames in diffs are uncommon but real, so we accept either.

If neither matches, `FileSlice` returns `("", false)`. The handler maps that to a tool error so the caller can correct.

### D4. Line slicing happens on the plain-text path only

`get_file_content` has two modes:

1. `with_metadata=false` (default): plain text via SDK `GetFile`. Slicing here is a simple `strings.Split("\n")` + range.
2. `with_metadata=true`: full `ContentsResponse` with base64 content. Slicing this would force the handler to decode, slice, re-encode, and also lie about `sha` / `size`. Not worth it. Document that `start_line`/`end_line` are ignored when `with_metadata=true`.

### D5. Line range semantics

- 1-indexed inclusive on both bounds (matches what humans and `awk`/`sed` users expect).
- If `start_line` < 1, clamp to 1.
- If `end_line` > line count, clamp to line count.
- If `start_line` > `end_line` after clamping, return a tool error explaining the inversion.
- If both are omitted (or both zero), no slicing — full content returned. Preserves the current contract.
- If `start_line` is set without `end_line`, treat `end_line` as line count (tail from start to EOF).
- If `end_line` is set without `start_line`, treat `start_line` as 1 (head up to end_line).

### D6. Document missing tools while we're here

`get_pull_request_diff` and `list_pull_request_files` are missing from the README's tool table today — caught during the issue triage that produced this change. Add both to the table in the same PR. Single drive-by fix.

## Risks / Trade-offs

- **[Binary diffs]** A `diff --git` section for a binary file may contain `Binary files a/x and b/y differ` with no hunks. `FileSlice` returns the same section — caller gets a useful (and short) result. → No mitigation needed.
- **[Renamed-file disambiguation]** `diff --git a/old b/new` matches either path. If `old` and `new` are both passed in separate calls, the slicer returns the same section twice (correct, but possibly surprising). → Documented in the tool description.
- **[`start_line` semantics on CRLF files]** `strings.Split("\n")` on a CRLF file gives lines with trailing `\r`. Acceptable — agents that care about exact bytes set `with_metadata=true`. → Documented.
- **[Performance on huge diffs]** A 10 MB diff string scanned once for a header line is still <50 ms. Not a concern at MCP scale. → No mitigation.

## Migration Plan

Additive only. Default behavior of both tools is unchanged when the new params are omitted.

- `get_pull_request_diff` callers who pass nothing keep getting full diffs.
- `get_file_content` callers who pass nothing keep getting full files.
- Existing tests of both tools continue to pass without modification (verified post-impl).

No rollback complexity: the new params are purely additive; reverting the commit removes the params and the package without leaving dead state.

## Open Questions

_None._ Issue #124 reporter's ask is well-scoped to the two tools; the umbrella issue tracks adjacent paging work as siblings, deferred per proposal's "out of scope" section.
