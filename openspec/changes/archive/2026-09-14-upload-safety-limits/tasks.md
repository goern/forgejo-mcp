<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `pkg/upload.Open()` rejects a base64 `content` argument over 90MiB
      (`upload.MaxContentB64Bytes`) before decoding it, with an error naming
      the received size and the limit. This applies uniformly to
      `create_issue_attachment`, `create_comment_attachment`, and
      `create_release_attachment`, since all three resolve their source
      through `Open()`.
- [x] A base64 decode failure in `Open()` reports how many bytes this server
      received alongside the stdlib error — again uniformly across all three
      tools.
- [x] `operation/attachment.openAttachmentSource()` deleted: it duplicated
      `upload.Open`'s "exactly one of content or file_path" and "filename is
      required" checks. `CreateIssueAttachmentFn` / `CreateCommentAttachmentFn`
      now call `upload.Open(upload.SourceFromArguments(args))` directly, like
      `CreateReleaseAttachmentFn` already did.
- [x] `create_issue_attachment` / `create_comment_attachment` wrap their
      multipart upload in a 45s context timeout, under the existing 60s HTTP
      client timeout. `create_release_attachment` is explicitly NOT wrapped —
      release assets may need more upload headroom.
- [x] A timeout that fires is reported as "upload timed out after 45s"
      (`wrapUploadTimeout`), not a bare "context deadline exceeded".
- [x] `attachmentUploadTimeout` is a package-level var defaulting to the 45s
      constant, so tests can shrink it; production code never reassigns it.
- [x] Unit tests in `pkg/upload/input_test.go`: oversized-content rejection,
      received-length diagnostic on decode failure.
- [x] Unit test in `operation/release/release_test.go`: oversized-content
      rejection reaches `create_release_attachment` through the same
      `upload.Open` call.
- [x] Unit tests in `operation/attachment/attachment_test.go`: `httptest`
      backend that blocks on `<-r.Context().Done()` with the timeout shrunk
      to 100ms, asserting the call returns within 1s with an "upload timed
      out after" error, for both `create_issue_attachment` and
      `create_comment_attachment`.
- [x] `wrapUploadTimeout` checks the parent context's own deadline before
      attributing a timeout to the upload's own 45s budget, so a caller-side
      deadline is never misreported as "upload timed out after 45s". Covered
      by `TestWrapUploadTimeout_ParentDeadlineNotMisattributed`.
- [x] `test/transport/sweep_test.go` builds the current tree into a temp
      binary via `TestMain` (`go build -o $TMP/forgejo-mcp ../..`), so the
      test is self-contained on a clean checkout and always exercises the
      current source, not a stale prebuilt binary.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` clean from a clean
      checkout (`go clean -testcache` then `go test ./...`).
