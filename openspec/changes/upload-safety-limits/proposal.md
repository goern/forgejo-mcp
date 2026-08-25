<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

Multiple agents reported `create_issue_attachment`/`create_comment_attachment`
failing with "illegal base64 data" at a variable byte offset
(~1.5KB-4.9KB observed across sessions) and, once, hanging a caller for
~30 minutes. A size-sweep repro against the real stdio transport
(`test/transport/sweep_test.go`, 1KB-100KB, byte-for-byte fidelity checked via
sha256) found no corruption or hang in this server across that range — the
root cause sits upstream of this process, most likely in the calling MCP
client/harness. That does not make an unbounded `content` argument or an
unbounded upload call safe to accept: whatever produced a runaway or stuck
request, this server should fail it fast with a clear, actionable error
rather than paying to decode a huge payload or hanging on a stuck network
path.

## What Changes

- `decodeAttachmentContent()` rejects a base64 `content` argument larger than
  90MiB before decoding it, instead of paying to allocate and decode a
  runaway argument first. A decode failure also now reports how many bytes
  this server received alongside the stdlib error, so a caller can tell at a
  glance whether truncation happened before or after this process.
- `create_issue_attachment` and `create_comment_attachment` wrap their
  multipart upload call in a 45s context timeout (under the existing 60s HTTP
  client timeout), reported as "upload timed out after 45s" rather than a
  bare "context deadline exceeded" or an unbounded hang.
- The timeout is held in a package-level var (not a const) defaulting to the
  same 45s constant, so tests can shrink it to exercise the timeout path
  without a real 45s wait; production code never reassigns it.

`create_release_attachment` is unaffected: it does not accept base64
`content` and goes through a separate multipart path.

## Capabilities

### Modified Capabilities

- `attachment-upload-sources`: gains an "Upload safety limits" requirement
  bounding the base64 `content` size and the upload call's wall-clock time
  for `create_issue_attachment` and `create_comment_attachment`.

### New Capabilities

_None._ This is additive to the existing capability only.

## Impact

- **Code**: `operation/attachment/attachment.go` — `decodeAttachmentContent`,
  `openAttachmentSource`, `wrapUploadTimeout`, the `maxAttachmentContentB64Bytes`
  and `attachmentUploadTimeout` limits.
- **Tests**: `operation/attachment/attachment_test.go` — oversized-content
  rejection, received-length diagnostic on decode failure, and an
  `httptest` backend that blocks on `<-r.Context().Done()` to prove the
  upload timeout actually fires and bounds wall-clock time.
- **APIs**: no new tool arguments or response fields; existing calls that
  stay under the limits are unaffected.
- **Dependencies**: none added.
- **Risk**: low — the limits sit well above any legitimate issue/comment
  attachment and only change behavior for a request that was already
  malformed, oversized, or stuck.
