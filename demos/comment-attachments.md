# Demo: comment attachments — full CRUD

*2026-04-26T13:12:00Z*
<!-- showboat-id: b2e3c4d5-comment-attachments-demo-2026 -->

## What these tools do

Six MCP tools mirror the issue-attachment tools but operate on issue/PR comment attachments — the same lifecycle (list, get, download, create, edit, delete), keyed by `comment_id` instead of `index`:

- `list_comment_attachments`
- `get_comment_attachment`
- `download_comment_attachment` (inline if < 1 MiB; metadata + URL otherwise)
- `create_comment_attachment`
- `edit_comment_attachment`
- `delete_comment_attachment`

Same wire format, same 1 MiB cap, same "always include `browser_download_url`" contract as the issue-attachment tools. See `demos/issue-attachments.md` for the design rationale.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=...
make build
```

## End-to-end lifecycle

### 1. Create a comment to attach to

```bash
./forgejo-mcp --cli create_issue_comment \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108,"body":"Comment for attachment demo"}'
```

The response includes the new comment's `id` — use that as `comment_id` below.

### 2. Upload an attachment to the comment

```bash
B64=$(base64 -w0 /tmp/snippet.txt)
./forgejo-mcp --cli create_comment_attachment \
  --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\",\"comment_id\":13781165,\"content\":\"$B64\",\"filename\":\"snippet.txt\",\"mime_type\":\"text/plain\"}"
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":{\"id\":1174940,\"name\":\"snippet.txt\",\"size\":24,\"download_count\":0,\"created_at\":\"2026-04-26T12:59:29+02:00\",\"uuid\":\"251ef6f2-b2c4-4fc5-877e-cc4573bdd884\",\"browser_download_url\":\"https://codeberg.org/attachments/251ef6f2-b2c4-4fc5-877e-cc4573bdd884\"}}"
  }
]
```

### 3. List comment attachments

```bash
./forgejo-mcp --cli list_comment_attachments \
  --args '{"owner":"goern","repo":"forgejo-mcp","comment_id":13781165}'
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":[{\"id\":1174940,\"name\":\"snippet.txt\",\"size\":24,...}]}"
  }
]
```

### 4. Get single-attachment metadata

```bash
./forgejo-mcp --cli get_comment_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","comment_id":13781165,"attachment_id":1174940}'
```

### 5. Download

```bash
./forgejo-mcp --cli download_comment_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","comment_id":13781165,"attachment_id":1174940}'
```

Same response shape as `download_issue_attachment` — a JSON text part and (if under cap) an `EmbeddedResource` with the base64 blob.

### 6. Rename

```bash
./forgejo-mcp --cli edit_comment_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","comment_id":13781165,"attachment_id":1174940,"name":"renamed.txt"}'
```

### 7. Delete

```bash
./forgejo-mcp --cli delete_comment_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","comment_id":13781165,"attachment_id":1174940}'
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":{\"status\":\"deleted\"}}"
  }
]
```

## Pattern: agent walks an issue with attachments scattered across comments

```text
1. get_issue_by_index            → see issue body, comment count
2. list_issue_attachments        → see issue-level attachments
3. list_issue_comments           → enumerate comment IDs
4. for each comment_id:
     list_comment_attachments    → discover comment-level attachments
5. download_*_attachment         → pull bytes for any relevant file
```

This pattern lets an agent fully reconstruct an issue's attachment graph — the gap reported in #106.
