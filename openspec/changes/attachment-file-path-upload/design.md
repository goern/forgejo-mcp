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
6. `file_path` is off by default. It hands whatever drives the MCP client a
   read primitive over the host filesystem, so a prompt-injected agent could
   turn `~/.ssh/id_ed25519` into a public release asset. `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD`
   makes enabling it a deliberate operator act, and `FORGEJO_MCP_UPLOAD_ROOT`
   narrows the blast radius to one directory. Both are read at call time rather
   than at startup so tests and short-lived CLI runs can scope them.
7. Confinement compares symlink-resolved paths, because a symlink planted
   inside the root would otherwise read anything the process can reach.

## Compatibility

Existing base64 callers continue to provide `content` and `filename`. Response
types, API endpoints, authentication, and optional MIME hints do not change.
