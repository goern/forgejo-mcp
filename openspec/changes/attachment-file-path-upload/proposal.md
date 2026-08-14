<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Why

Attachment upload tools currently require callers to load a file, encode it as
base64, and send the expanded payload through MCP. Local and CLI clients often
already share a filesystem with `forgejo-mcp`, making that conversion wasteful
and inconvenient, especially for release artifacts.

## What Changes

- Add `file_path` as an alternative to `content` on issue, comment, and release
  attachment creation tools.
- Require exactly one source while preserving empty base64 files.
- Infer the upload filename from a path unless the caller overrides it.
- Stream multipart requests so path uploads do not buffer entire files in
  memory.
- Document that paths belong to the `forgejo-mcp` host and relative paths use
  its process working directory.
- Gate host file reads behind `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD`, with an
  optional `FORGEJO_MCP_UPLOAD_ROOT` confining them to one directory.

## Impact

The change is additive for existing callers. Existing base64 requests keep the
same parameters and responses. Release creation uses the existing raw HTTP
multipart helper so all three attachment domains share streaming behavior.
