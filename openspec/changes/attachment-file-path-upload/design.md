<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## Decisions

1. `content` and `file_path` remain separate optional MCP parameters. Runtime
   validation requires exactly one key because the tool schema cannot express
   this exclusive relationship.
2. Presence is based on the argument key, not a non-empty value. This preserves
   `content: ""` as a valid zero-byte upload and rejects requests containing
   both keys even when one is empty.
3. Relative paths resolve from the server process working directory. Paths are
   normalized, opened read-only, and must resolve to regular files.
4. A shared `pkg/upload` helper owns source validation, file opening, basename
   inference, and base64 decoding.
5. Multipart bodies stream through `io.Pipe`. Release uploads use the same raw
   HTTP helper instead of the SDK creation method, which buffers the body.

## Compatibility

Existing base64 callers continue to provide `content` and `filename`. Response
types, API endpoints, authentication, and optional MIME hints do not change.
