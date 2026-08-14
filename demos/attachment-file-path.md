<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Attachment uploads from host file paths

<!-- captured-for: PR #481 -->
<!-- captured-at: 2026-08-14T16:41:36Z -->
<!-- captured-against: 73d4d9a69ee042a526f720d9b4ea15057e03d4c5 -->

*2026-08-14T16:41:36Z by Showboat 0.6.1*
<!-- showboat-id: e0e1a294-1a5e-46f6-a5bc-131a3e9aaf65 -->

This demo proves the [attachment upload source OpenSpec delta](../openspec/changes/attachment-file-path-upload/specs/attachment-upload-sources/spec.md) against the public [`pisco/showboat-attachment-demo`](https://git.b4mad.industries/pisco/showboat-attachment-demo) repository. The live targets are [issue #1](https://git.b4mad.industries/pisco/showboat-attachment-demo/issues/1), comment `5988`, and [release `attachment-demo-20260814`](https://git.b4mad.industries/pisco/showboat-attachment-demo/releases/tag/attachment-demo-20260814).

## Replay setup

```bash
export FORGEJO_URL=https://git.b4mad.industries
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-forgejo-mcp}"
export FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD=1
```

Run all commands from the repository root. The token is supplied through the environment and is never embedded in a captured command.

> **Note.** The capture below predates the `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD`
> gate; every `file_path` command shown now additionally requires that variable,
> which is why the replay setup exports it. Without it each `file_path` call
> returns `file_path uploads are disabled on this host`. Setting
> `FORGEJO_MCP_UPLOAD_ROOT` to the repository root would further confine these
> reads. Base64 `content` commands in this demo are unaffected.

## Upload from `file_path` with basename defaulting

`create_issue_attachment` reads the public `README.md` from the MCP host. No `filename` is supplied, so the returned attachment name must default to the path basename.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"file_path":"README.md"}' --output=text
```

```output
{"Result":{"id":369,"name":"README.md","size":39215,"download_count":0,"created_at":"2026-08-14T16:43:04Z","uuid":"614777c5-8054-48fa-8831-34a6eadf1d0d","browser_download_url":"https://git.b4mad.industries/attachments/614777c5-8054-48fa-8831-34a6eadf1d0d"}}
```

## Explicit filename overrides the basename

The comment upload reads the public `README.md`, while `filename` deliberately requests `acceptance-notes.md`.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_comment_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","comment_id":5988,"file_path":"README.md","filename":"acceptance-notes.md"}' --output=text
```

```output
{"Result":{"id":370,"name":"acceptance-notes.md","size":39215,"download_count":0,"created_at":"2026-08-14T16:43:33Z","uuid":"9b0ea2fc-0801-4fa4-ab45-4e882df63f01","browser_download_url":"https://git.b4mad.industries/attachments/9b0ea2fc-0801-4fa4-ab45-4e882df63f01"}}
```

## Relative paths resolve from the server working directory

The release upload passes the relative path `LICENSE`. Because the CLI starts in the repository root, the server resolves it from its own working directory.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_release_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","release_id":428,"file_path":"LICENSE"}' --output=text
```

```output
{"Result":{"id":371,"name":"LICENSE","size":34826,"download_count":0,"created_at":"2026-08-14T16:43:54Z","uuid":"14e74bb2-55eb-4f5e-8c67-b9eb403f8500","browser_download_url":"https://git.b4mad.industries/attachments/14e74bb2-55eb-4f5e-8c67-b9eb403f8500"}}
```

## Reject both upload sources

Argument selection is based on key presence. Supplying `content` together with `file_path`, even when `content` is empty, is rejected before any Forgejo upload.

```bash
set -o pipefail; ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"content":"","file_path":"README.md","filename":"invalid.txt"}' --output=text 2>&1 | sed -n '/^Error: tool execution failed:/p'; status=$?; echo "exit_status=$status"; test "$status" -ne 0
```

```output
Error: tool execution failed: exactly one of content or file_path is required
exit_status=1
```

## Reject a missing upload source

Omitting both `content` and `file_path` is rejected by the same preflight validation.

```bash
set -o pipefail; ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"filename":"missing-source.txt"}' --output=text 2>&1 | sed -n '/^Error: tool execution failed:/p'; status=$?; echo "exit_status=$status"; test "$status" -ne 0
```

```output
Error: tool execution failed: exactly one of content or file_path is required
exit_status=1
```

## Empty `content` remains a valid zero-byte upload

An explicitly present empty `content` string is different from an omitted key and creates a zero-byte attachment.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"content":"","filename":"zero-byte.txt"}' --output=text
```

```output
{"Result":{"id":372,"name":"zero-byte.txt","size":0,"download_count":0,"created_at":"2026-08-14T16:45:15Z","uuid":"c804a5f0-b731-40a6-9d42-492af6f9882d","browser_download_url":"https://git.b4mad.industries/attachments/c804a5f0-b731-40a6-9d42-492af6f9882d"}}
```

## Large release asset streams from `file_path`

A 16 MiB fixture makes the motivating release-asset case concrete. The fixture is generated in the server working directory, uploaded through `file_path`, and then removed locally; the release asset remains available for inspection.

```bash
dd if=/dev/zero of=.showboat-large-asset.bin bs=1M count=16 status=none && wc -c .showboat-large-asset.bin
```

```output
16777216 .showboat-large-asset.bin
```

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_release_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","release_id":428,"file_path":".showboat-large-asset.bin","filename":"forgejo-mcp-streaming-demo.bin"}' --output=text
```

```output
{"Result":{"id":373,"name":"forgejo-mcp-streaming-demo.bin","size":16777216,"download_count":0,"created_at":"2026-08-14T16:46:16Z","uuid":"1c7e76e8-fb7e-4bdf-973e-38725e510502","browser_download_url":"https://git.b4mad.industries/attachments/1c7e76e8-fb7e-4bdf-973e-38725e510502"}}
```

```bash
rm -f .showboat-large-asset.bin && test ! -e .showboat-large-asset.bin && echo "local fixture removed"
```

```output
local fixture removed
```

## Result

All three attachment surfaces accept host file paths. Basename defaulting, explicit override, relative resolution, presence-based source validation, zero-byte base64 compatibility, and a large streamed release asset are demonstrated by live Forgejo responses.
