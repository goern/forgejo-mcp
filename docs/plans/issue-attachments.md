# Issue & Comment Attachments — Implementation Plan

**Issue**: [#106](https://codeberg.org/goern/forgejo-mcp/issues/106)
**Branch**: `feature/issue-attachments` *(suggested)*

## Summary

Add full CRUD MCP tools for attachments on **issues** and **issue comments**, so an agent reading an issue through `forgejo-mcp` can discover, retrieve, upload, rename, and delete attachments. This closes the gap reported in #106, where a PDF uploaded via the web UI was invisible to the MCP client.

The Forgejo REST API exposes issue and comment attachments via dedicated endpoints that `GET /issues/{index}` does not embed. `forgejo-sdk/v3@v3.0.0` ships attachment methods for **releases only** — there is no SDK coverage for issue or comment attachments.

## Use Case

> An agent working through the MCP server reads an issue the user created in the web UI. The user uploaded a PDF attachment that contains information the issue depends on. The agent must see that the attachment exists, retrieve its bytes, and reason over the content. The agent must also be able to upload, rename, and delete attachments on both issues and issue comments.

## Current State

- `operation/issue/issue.go` registers 14 issue tools; none touch attachments.
- `pkg/forgejo/forgejo.go` exposes a singleton SDK client built from `flag.URL` + `flag.Token`; no raw-HTTP helper.
- `forgejo-sdk/v3@v3.0.0/attachment.go` defines `Attachment` + release-attachment methods (`ListReleaseAttachments`, `GetReleaseAttachment`, `CreateReleaseAttachment`, `EditReleaseAttachment`, `DeleteReleaseAttachment`). No issue or comment equivalents.
- `github.com/mark3labs/mcp-go@v0.44.0` supports binary payloads via `mcp.BlobResourceContents` wrapped in `mcp.NewEmbeddedResource`.

## Key Design Decision: SDK Gap

The SDK does not cover issue or comment attachments. Three options were considered:

1. **Upstream-first (blocked-feature pattern)** — contribute to forgejo-sdk, wait for merge, then wire MCP tools. Consistent with `wiki-support.md` / `projects-support.md`. Ship time gated on upstream review.
2. **Raw HTTP inside forgejo-mcp** — bypass the SDK, call the Forgejo API directly using stdlib `net/http` with the existing `flag.URL` and `flag.Token`. Ships immediately; introduces a second HTTP code path in this repo.
3. **Upstream contribution + temporary `replace` directive** — contribute upstream and use a fork via `go.mod replace` until the upstream PR merges.

**Decision: Option 2 (raw HTTP)**. Rationale: the attachment endpoints are stable Gitea/Forgejo API routes, the raw-HTTP surface is small and contained behind a single helper package, and shipping time beats consistency here. A future refactor can migrate to SDK methods without changing the MCP tool surface.

**Risk**: raw HTTP bypasses SDK versioning. Mitigation: one helper (`pkg/forgejo/rawhttp.go`) owns all HTTP construction; every attachment tool goes through it; helper is unit-tested with `httptest`.

## Target State

### 12 New MCP Tools

Issue-scoped (6):

| Tool | HTTP | Endpoint | Returns |
|------|------|----------|---------|
| `list_issue_attachments` | GET | `/repos/{owner}/{repo}/issues/{index}/assets` | `[]Attachment` JSON |
| `get_issue_attachment` | GET | `/repos/{owner}/{repo}/issues/{index}/assets/{attachment_id}` | `Attachment` JSON (metadata) |
| `download_issue_attachment` | GET | `{browser_download_url}` | MCP `EmbeddedResource` (`BlobResourceContents`) |
| `create_issue_attachment` | POST | `/repos/{owner}/{repo}/issues/{index}/assets` *(multipart)* | `Attachment` JSON |
| `edit_issue_attachment` | PATCH | `/repos/{owner}/{repo}/issues/{index}/assets/{attachment_id}` | `Attachment` JSON |
| `delete_issue_attachment` | DELETE | `/repos/{owner}/{repo}/issues/{index}/assets/{attachment_id}` | `"Delete attachment success"` |

Comment-scoped (6):

| Tool | HTTP | Endpoint | Returns |
|------|------|----------|---------|
| `list_comment_attachments` | GET | `/repos/{owner}/{repo}/issues/comments/{comment_id}/assets` | `[]Attachment` JSON |
| `get_comment_attachment` | GET | `/repos/{owner}/{repo}/issues/comments/{comment_id}/assets/{attachment_id}` | `Attachment` JSON |
| `download_comment_attachment` | GET | `{browser_download_url}` | MCP `EmbeddedResource` |
| `create_comment_attachment` | POST | `/repos/{owner}/{repo}/issues/comments/{comment_id}/assets` *(multipart)* | `Attachment` JSON |
| `edit_comment_attachment` | PATCH | `/repos/{owner}/{repo}/issues/comments/{comment_id}/assets/{attachment_id}` | `Attachment` JSON |
| `delete_comment_attachment` | DELETE | `/repos/{owner}/{repo}/issues/comments/{comment_id}/assets/{attachment_id}` | `"Delete attachment success"` |

### Tool Parameter Schemas

Shared parameters (added to `pkg/params/params.go`):

```go
AttachmentID      = "Attachment ID"
AttachmentName    = "New name for the attachment"
AttachmentContent = "Base64-encoded file bytes to upload"
AttachmentFilename = "Filename to associate with the uploaded attachment (e.g. \"requirements.pdf\")"
AttachmentMIME    = "MIME type hint for uploaded file (optional; inferred from filename if omitted)"
```

The MCP tool accepts base64 bytes only — deliberately no file-path parameter, because the MCP server is often invoked in a context where the agent's filesystem is not the same as the Forgejo server's. A future CLI convenience wrapper may accept `--file path` and encode before dispatch; that is out of scope for the tool definitions themselves.

Parameter matrix:

| Tool | `owner` | `repo` | `index` / `comment_id` | `attachment_id` | `name` | `content` | `filename` | `mime_type` |
|------|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| `list_*_attachments` | ✓ | ✓ | ✓ | | | | | |
| `get_*_attachment` | ✓ | ✓ | ✓ | ✓ | | | | |
| `download_*_attachment` | ✓ | ✓ | ✓ | ✓ | | | | |
| `create_*_attachment` | ✓ | ✓ | ✓ | | | ✓ | ✓ | opt |
| `edit_*_attachment` | ✓ | ✓ | ✓ | ✓ | ✓ | | | |
| `delete_*_attachment` | ✓ | ✓ | ✓ | ✓ | | | | |

### Architecture

```
main.go
 └─ cmd/cmd.go
     └─ operation/operation.go
         ├─ operation/issue/            (existing)
         └─ operation/attachment/        ← NEW domain package
             └─ attachment.go            (12 tools + handlers)

pkg/forgejo/
 ├─ forgejo.go                           (existing SDK client singleton)
 └─ rawhttp.go                           ← NEW raw-HTTP helper
```

#### `pkg/forgejo/rawhttp.go`

Single place that owns raw HTTP access to the Forgejo API. All attachment tools call into this helper; no `net/http` imports anywhere in `operation/attachment/`.

Signature sketch:

```go
package forgejo

// DoJSON performs an authenticated JSON request. Encodes body as JSON (nil ok),
// decodes 2xx response into out (nil ok for no-content). Non-2xx → error with
// status code + sanitized body snippet.
func DoJSON(ctx context.Context, method, relPath string, body, out any) (*http.Response, error)

// DoMultipart performs an authenticated multipart POST with a single file part.
// fieldName is the form field (e.g. "attachment"); filename is sent in the part header;
// r supplies the bytes.
func DoMultipart(ctx context.Context, method, relPath, fieldName, filename, mimeType string, r io.Reader, out any) (*http.Response, error)

// DoRaw fetches bytes from a URL (absolute or relative to the configured base URL),
// adding the configured auth header. Returns body bytes + content type.
// Used by download_*_attachment to fetch from browser_download_url.
func DoRaw(ctx context.Context, absoluteOrRelativeURL string) (body []byte, contentType string, err error)
```

- `flag.URL` + `flag.Token` + `flag.UserAgent` feed the request (same as SDK client).
- 401/403 → `ErrUnauthorized` wrapper so callers can distinguish auth failure.
- 404 on `GET …/assets` → **empty slice**, not error (list semantics).
- 404 on all other endpoints → error.

#### `operation/attachment/attachment.go`

Each handler is thin: parse args → call `rawhttp.DoX` → `to.TextResult` or build `EmbeddedResource`.

Download handlers are the only interesting ones:

```go
func DownloadIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. GET .../assets/{aid}  → Attachment metadata
    // 2. If Size > MaxInlineDownloadBytes: return metadata + message
    //    "file exceeds 10 MiB inline cap; fetch {browser_download_url} directly"
    // 3. Otherwise: DoRaw(browser_download_url) → base64 → BlobResourceContents
    // 4. Wrap in mcp.NewEmbeddedResource and return
}
```

Constant:

```go
const MaxInlineDownloadBytes = 10 * 1024 * 1024 // 10 MiB; see Open Questions
```

### Registration

`operation/operation.go` gains:

```go
import "codeberg.org/goern/forgejo-mcp/v2/operation/attachment"
// …
attachment.RegisterTool(s)
```

## Implementation Steps

### Part 1 — Verify API contract

1. Confirm endpoint shapes against a live Codeberg issue with a PDF attached:
   - `GET /api/v1/repos/{o}/{r}/issues/{index}/assets` returns `[]Attachment`.
   - `POST` multipart field name is `attachment` (matches release-assets convention in SDK).
   - `PATCH` body is `{"name":"..."}` (matches SDK's `EditAttachmentOptions`).
2. Record actual response JSON for one attachment in the `Open Questions` section if any field differs from the SDK's `Attachment` struct.

### Part 2 — `pkg/forgejo/rawhttp.go`

1. Implement `DoJSON`, `DoMultipart`, `DoRaw`.
2. Unit tests with `httptest.NewServer`:
   - Auth header propagation (`Authorization: token {flag.Token}` — match SDK's `SetToken`).
   - User-Agent header propagation.
   - Multipart boundary correctness (round-trip via `mime/multipart.NewReader`).
   - 2xx decode, 4xx error mapping, 404-on-list → `io.EOF`-style sentinel the caller can treat as empty.
   - 10 MiB cap enforced by `DoRaw` (test reads response with `io.LimitReader`).
3. All tests live in `pkg/forgejo/rawhttp_test.go`; no live-API calls.

### Part 3 — `operation/attachment/attachment.go`

1. Define 12 tools with `mcp.NewTool` and a `RegisterTool(s *server.MCPServer)` function matching the pattern in `operation/issue/issue.go`.
2. Implement handlers. Shape follows:

   ```go
   func ListIssueAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
       owner, _ := req.GetArguments()["owner"].(string)
       repo, _  := req.GetArguments()["repo"].(string)
       index, _ := to.Float64(req.GetArguments()["index"])

       var out []forgejo_sdk.Attachment
       path := fmt.Sprintf("/repos/%s/%s/issues/%d/assets", owner, repo, int64(index))
       if _, err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
           return to.ErrorResult(fmt.Errorf("list issue attachments err: %v", err))
       }
       return to.TextResult(out)
   }
   ```

3. Reuse the SDK's `Attachment` struct (`forgejo_sdk.Attachment`) as the DTO — the release and issue/comment attachment JSON shapes are identical in Forgejo.

### Part 4 — Wire-up

1. Add `attachment.RegisterTool(s)` in `operation/operation.go`.
2. Add parameter descriptions to `pkg/params/params.go`.
3. Add tool row to `README.md` tool table.

### Part 5 — Demos (Showboat format)

Two files in `demos/`:

1. **`demos/issue-attachments.md`** — lifecycle walkthrough:
   - Create a scratch issue (`create_issue`).
   - Upload a small PDF (`create_issue_attachment`).
   - List (`list_issue_attachments`), get metadata (`get_issue_attachment`).
   - Download & verify bytes (`download_issue_attachment`).
   - Rename (`edit_issue_attachment`).
   - Delete (`delete_issue_attachment`).
   - Close the scratch issue.
2. **`demos/comment-attachments.md`** — same lifecycle against a comment on a scratch issue.

Both use `./forgejo-mcp --cli <tool>` invocations, match the formatting of `demos/issue-labels.md`, and carry a valid Showboat header (`<!-- showboat-id: … -->`).

### Part 6 — Tests

- **Unit**: `pkg/forgejo/rawhttp_test.go` (see Part 2).
- **Handler smoke tests**: `operation/attachment/attachment_test.go` with a mock HTTP server injected via a test hook on `rawhttp` (simplest: make the base URL configurable at request time, then point tests at `httptest.NewServer`).
- **No live-API tests in CI** (matches repo posture).

## Deliverables

- [ ] `pkg/forgejo/rawhttp.go` + tests.
- [ ] `operation/attachment/attachment.go` + tests.
- [ ] Registration wired in `operation/operation.go`.
- [ ] Parameter descriptions added to `pkg/params/params.go`.
- [ ] `README.md` tool table updated.
- [ ] `demos/issue-attachments.md`.
- [ ] `demos/comment-attachments.md`.
- [ ] CHANGELOG entry under next version.

## Acceptance Criteria

1. `./forgejo-mcp --cli list_issue_attachments --owner goern --repo forgejo-mcp --index 106` against a Codeberg issue with a PDF returns the attachment metadata.
2. `./forgejo-mcp --cli download_issue_attachment …` for a PDF < 10 MiB returns an `EmbeddedResource` whose `Blob` decodes to the exact bytes of the uploaded file (`sha256` match).
3. Round-trip: `create_issue_attachment` → `edit_issue_attachment` (rename) → `list_issue_attachments` shows the new name → `delete_issue_attachment` → `list_issue_attachments` no longer shows it.
4. Same round-trip works against `*_comment_attachment` tools on an issue comment.
5. `list_*_attachments` on an entity with zero attachments returns `[]`, not an error.
6. `make build` passes; `go test ./...` passes.
7. Both demo files run cleanly end-to-end against a live Forgejo (manual verification).

## Open Questions

1. **Inline size cap**: 10 MiB is a guess. Should it be configurable via a new `--max-inline-download-bytes` flag? *Deferring to follow-up; ship with the constant first.*
2. **Authorization for `browser_download_url`**: private-repo attachments are served behind session auth on the web path. Confirm whether adding `Authorization: token …` works on the `/attachments/{uuid}` route or whether we need to resolve to the API asset-content path instead (`GET /repos/.../assets/{id}` with `Accept: application/octet-stream` may stream bytes directly — verify in Part 1).
3. **Multipart field name for comment attachments**: release assets use `attachment`; need to confirm issue/comment endpoints accept the same. Likely yes, verify in Part 1.
4. **Exposing `updated_at` on create**: the Forgejo API documents an optional `updated_at` field in the multipart form. YAGNI for v1 — omit unless a caller needs it.

## Risk Assessment

- **Low risk**: all 12 tools are additive; no existing tool changes behavior.
- **Medium risk**: raw HTTP bypasses SDK validation. If Forgejo renames a field or changes auth semantics, attachment tools break silently while other tools keep working. Mitigated by (a) reusing `forgejo_sdk.Attachment` as the DTO so struct drift trips compile errors as soon as the SDK updates, (b) demo files serve as live-integration canaries.
- **Low risk**: binary `EmbeddedResource` support is already used elsewhere in the mcp-go ecosystem; the 10 MiB cap bounds context-bloat blast radius.

## Follow-ups (not in this spec)

- Migrate raw-HTTP attachment calls to SDK methods once upstream adds them (track as separate beads issue; this spec is the migration target).
- Configurable inline-download cap.
- Support uploading from a URL instead of base64 bytes (saves client-side encoding for large files).
