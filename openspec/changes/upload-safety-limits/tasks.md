<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Tasks

- [x] `decodeAttachmentContent` rejects a base64 `content` argument over
      90MiB before decoding it, with an error naming the received size and
      the limit.
- [x] A base64 decode failure reports how many bytes this server received
      alongside the stdlib error.
- [x] `create_issue_attachment` / `create_comment_attachment` wrap their
      multipart upload in a 45s context timeout, under the existing 60s HTTP
      client timeout.
- [x] A timeout that fires is reported as "upload timed out after 45s"
      (`wrapUploadTimeout`), not a bare "context deadline exceeded".
- [x] `attachmentUploadTimeout` is a package-level var defaulting to the 45s
      constant, so tests can shrink it; production code never reassigns it.
- [x] Unit tests: oversized-content rejection, received-length diagnostic on
      decode failure.
- [x] Unit tests: `httptest` backend that blocks on `<-r.Context().Done()`
      with the timeout shrunk to 100ms, asserting the call returns within 1s
      with an "upload timed out after" error, for both
      `create_issue_attachment` and `create_comment_attachment`.
- [x] `test/transport/sweep_test.go` drives the real built binary over the
      actual stdio JSON-RPC wire across payload sizes 1KB-100KB, verifying
      byte-for-byte fidelity via sha256.
- [x] `go build ./...`, `go vet ./...`, `go test ./...` clean.
