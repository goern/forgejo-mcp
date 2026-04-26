# Demo: issue attachments — full CRUD

*2026-04-26T13:10:00Z*
<!-- showboat-id: a1f2b3c4-issue-attachments-demo-2026 -->

## What these tools do

Six MCP tools cover the full attachment lifecycle on issues and pull requests, closing the gap reported in [#106](https://codeberg.org/goern/forgejo-mcp/issues/106) where attachments uploaded via the web UI were invisible to MCP clients:

- `list_issue_attachments` — Enumerate attachments on an issue/PR.
- `get_issue_attachment` — Fetch metadata for a single attachment.
- `download_issue_attachment` — Pull the bytes (inline if < 1 MiB; metadata + URL otherwise).
- `create_issue_attachment` — Upload a new attachment from base64 content.
- `edit_issue_attachment` — Rename an attachment.
- `delete_issue_attachment` — Remove an attachment.

The split between *inline bytes* and *metadata + URL* is deliberate. For files under the 1 MiB cap, the response carries the file as a base64-encoded `BlobResourceContents` (the MCP-protocol-native binary mechanism). For larger files, the response includes only metadata and the `browser_download_url`; the caller is expected to fetch that URL with the same auth token. See `docs/plans/issue-attachments.md` for the design rationale and the discussion on issue #106.

## Setup

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=...
make build
```

## End-to-end lifecycle

### 1. Upload a file (base64-encoded)

```bash
B64=$(base64 -w0 /tmp/demo.txt)
./forgejo-mcp --cli create_issue_attachment \
  --args "{\"owner\":\"goern\",\"repo\":\"forgejo-mcp\",\"index\":108,\"content\":\"$B64\",\"filename\":\"demo-notes.txt\",\"mime_type\":\"text/plain\"}"
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":{\"id\":1174982,\"name\":\"demo-notes.txt\",\"size\":67,\"download_count\":0,\"created_at\":\"2026-04-26T13:08:14+02:00\",\"uuid\":\"d6f13ced-2fc6-4319-8160-10f15ce55b0a\",\"browser_download_url\":\"https://codeberg.org/attachments/d6f13ced-2fc6-4319-8160-10f15ce55b0a\"}}"
  }
]
```

### 2. List attachments

```bash
./forgejo-mcp --cli list_issue_attachments \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108}'
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":[{\"id\":1174982,\"name\":\"demo-notes.txt\",\"size\":67,\"...\"}]}"
  }
]
```

### 3. Get single-attachment metadata

```bash
./forgejo-mcp --cli get_issue_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108,"attachment_id":1174982}'
```

### 4. Download — inline (file is under 1 MiB)

```bash
./forgejo-mcp --cli download_issue_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108,"attachment_id":1174982}'
```

```output
[
  {
    "type": "text",
    "text": "{\"attachment\":{\"id\":1174982,\"name\":\"demo-notes.txt\",\"size\":67,\"...\":\"https://codeberg.org/attachments/d6f13ced-2fc6-4319-8160-10f15ce55b0a\"},\"inline\":true,\"bytes_included\":67}"
  },
  {
    "type": "resource",
    "resource": {
      "uri": "https://codeberg.org/attachments/d6f13ced-2fc6-4319-8160-10f15ce55b0a",
      "mimeType": "text/plain; charset=utf-8",
      "blob": "RGVtbyBQREYgY29udGVudCBmb3IgZm9yZ2Vqby1tY3AgaXNzdWUtYXR0YWNobWVudHMuIFNIQT0xNzc3MjAxNjk0Cg=="
    }
  }
]
```

The response carries two content blocks: a JSON text part (metadata + `inline:true` + `bytes_included`) and an `EmbeddedResource` whose `Blob` is the base64-encoded file content.

### 5. Download — over the cap (file ≥ 1 MiB)

When the attachment exceeds the 1 MiB inline cap, the tool returns only metadata; no `Blob` field. The caller fetches the bytes with `curl` using the same token:

```output
[
  {
    "type": "text",
    "text": "{\"attachment\":{...,\"size\":2097152,\"browser_download_url\":\"https://codeberg.org/attachments/abc\"},\"inline\":false,\"reason\":\"size 2097152 bytes >= inline cap 1048576; fetch browser_download_url with Authorization: token <TOKEN>\"}"
  }
]
```

```bash
curl -H "Authorization: token $FORGEJO_ACCESS_TOKEN" \
  https://codeberg.org/attachments/abc -o big.bin
```

### 6. Rename

```bash
./forgejo-mcp --cli edit_issue_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108,"attachment_id":1174982,"name":"renamed-demo.txt"}'
```

### 7. Delete

```bash
./forgejo-mcp --cli delete_issue_attachment \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108,"attachment_id":1174982}'
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":{\"status\":\"deleted\"}}"
  }
]
```

### 8. List again — empty

```bash
./forgejo-mcp --cli list_issue_attachments \
  --args '{"owner":"goern","repo":"forgejo-mcp","index":108}'
```

```output
[
  {
    "type": "text",
    "text": "{\"Result\":[]}"
  }
]
```

A list against an issue with no attachments returns an empty array, never a 404 — the raw-HTTP helper transparently maps Forgejo's 404-on-empty-list quirk to `[]`.

## Why the 1 MiB cap

The 1 MiB cap on inline base64 is the result of design discussion on issue #106. Earlier iterations proposed a 10 MiB cap or a `save_to_path` parameter; both were rejected. The 1 MiB cap keeps small attachments ergonomic in the MCP response while preventing context-window blowout, and the always-included `browser_download_url` lets agents fall through to a direct authenticated fetch for anything bigger. No file paths cross the MCP boundary, so the same code works identically across stdio, SSE, and streamable-HTTP transports.
