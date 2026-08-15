<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Attachment uploads from host file paths

<!-- captured-for: PR #481 -->
<!-- captured-at: 2026-08-14T20:39:38Z -->
<!-- captured-against: 297ed21 -->

*2026-08-14T20:39:38Z by Showboat 0.6.1*
<!-- showboat-id: 5b74b0ca-acd5-411a-ad21-e171ad65e87c -->

This demo proves the [attachment upload source specification](../openspec/specs/attachment-upload-sources/spec.md) against the public [`pisco/showboat-attachment-demo`](https://git.b4mad.industries/pisco/showboat-attachment-demo) repository. The live targets are [issue #1](https://git.b4mad.industries/pisco/showboat-attachment-demo/issues/1), comment `5988`, and [release `attachment-demo-20260814`](https://git.b4mad.industries/pisco/showboat-attachment-demo/releases/tag/attachment-demo-20260814).

Every command below was captured with the `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` gate in force: the successful `file_path` uploads ran with the gate open and `FORGEJO_MCP_UPLOAD_ROOT` confining reads to the repository root, and the rejection cases ran with the gate explicitly closed.

## Replay setup

```bash
export FORGEJO_URL=https://git.b4mad.industries
export FORGEJO_ACCESS_TOKEN=<your-token>
export FORGEJO_MCP_BIN="${FORGEJO_MCP_BIN:-forgejo-mcp}"
export FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD=1
export FORGEJO_MCP_UPLOAD_ROOT=.
```

Run all commands from the repository root. The token is supplied through the environment and is never embedded in a captured command. `FORGEJO_MCP_UPLOAD_ROOT=.` resolves to the working directory of the `forgejo-mcp` process, so every `file_path` below must sit inside the checkout.

## Upload from `file_path` with basename defaulting

`create_issue_attachment` reads the public `README.md` from the MCP host. No `filename` is supplied, so the returned attachment name must default to the path basename.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"file_path":"README.md"}' --output=text
```

```output
{"Result":{"id":374,"name":"README.md","size":39903,"download_count":0,"created_at":"2026-08-14T20:39:38Z","uuid":"b226274c-08a0-4f6a-a625-15f865260a1e","browser_download_url":"https://git.b4mad.industries/attachments/b226274c-08a0-4f6a-a625-15f865260a1e"}}
```

## Explicit filename overrides the basename

The comment upload reads the public `README.md`, while `filename` deliberately requests `acceptance-notes.md`.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_comment_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","comment_id":5988,"file_path":"README.md","filename":"acceptance-notes.md"}' --output=text
```

```output
{"Result":{"id":375,"name":"acceptance-notes.md","size":39903,"download_count":0,"created_at":"2026-08-14T20:39:39Z","uuid":"1a3d0eec-be6b-4f6e-b53c-858632ff6f4a","browser_download_url":"https://git.b4mad.industries/attachments/1a3d0eec-be6b-4f6e-b53c-858632ff6f4a"}}
```

## Relative paths resolve from the server working directory

The release upload passes the relative path `LICENSE`. Because the CLI starts in the repository root, the server resolves it from its own working directory.

```bash
${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_release_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","release_id":428,"file_path":"LICENSE"}' --output=text
```

```output
{"Result":{"id":376,"name":"LICENSE","size":34594,"download_count":0,"created_at":"2026-08-14T20:39:39Z","uuid":"29acc612-4b8a-425d-8bf7-a902a64efb1b","browser_download_url":"https://git.b4mad.industries/attachments/29acc612-4b8a-425d-8bf7-a902a64efb1b"}}
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
{"Result":{"id":377,"name":"zero-byte.txt","size":0,"download_count":0,"created_at":"2026-08-14T20:39:39Z","uuid":"0c81c84c-004d-460a-98e4-bc20e7ab167e","browser_download_url":"https://git.b4mad.industries/attachments/0c81c84c-004d-460a-98e4-bc20e7ab167e"}}
```

## The gate is closed by default

With `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` removed from the environment, the same `file_path` upload that succeeded above is rejected before the file is opened, and the error names the variable that enables the feature.

```bash
set -o pipefail; env -u FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"file_path":"README.md"}' --output=text 2>&1 | sed -n '/^Error: tool execution failed:/p'; status=$?; echo "exit_status=$status"; test "$status" -ne 0
```

```output
Error: tool execution failed: file_path uploads are disabled on this host; set FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD=1 to enable them, or pass base64 content instead
exit_status=1
```

A value that is present but not truthy is treated the same way.

```bash
set -o pipefail; FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD=maybe ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"file_path":"README.md"}' --output=text 2>&1 | sed -n '/^Error: tool execution failed:/p'; status=$?; echo "exit_status=$status"; test "$status" -ne 0
```

```output
Error: tool execution failed: file_path uploads are disabled on this host; set FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD=1 to enable them, or pass base64 content instead
exit_status=1
```

## Paths outside `FORGEJO_MCP_UPLOAD_ROOT` are rejected

The gate is open here, but the configured root confines reads to the checkout. An absolute path outside it is rejected before anything is uploaded. The rejection quotes the symlink-resolved root, which is the directory this capture ran in.

```bash
set -o pipefail; ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"file_path":"/etc/hostname"}' --output=text 2>&1 | sed -n '/^Error: tool execution failed:/p'; status=$?; echo "exit_status=$status"; test "$status" -ne 0
```

```output
Error: tool execution failed: file_path must resolve inside FORGEJO_MCP_UPLOAD_ROOT (/var/home/goern/Source/agentic-forge/forgejo-mcp/.claude/worktrees/forgejo-mcp-2vo)
exit_status=1
```

A `..` escape out of the root is rejected by the same check.

```bash
set -o pipefail; ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"file_path":"../../../etc/hostname"}' --output=text 2>&1 | sed -n '/^Error: tool execution failed:/p'; status=$?; echo "exit_status=$status"; test "$status" -ne 0
```

```output
Error: tool execution failed: file_path must resolve inside FORGEJO_MCP_UPLOAD_ROOT (/var/home/goern/Source/agentic-forge/forgejo-mcp/.claude/worktrees/forgejo-mcp-2vo)
exit_status=1
```

## Base64 `content` is unaffected by the gate

The gate only governs host file reads. With `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` removed, a `content` upload still succeeds.

```bash
env -u FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD ${FORGEJO_MCP_BIN:-forgejo-mcp} --cli create_issue_attachment --args '{"owner":"pisco","repo":"showboat-attachment-demo","index":1,"content":"Z2F0ZS1jbG9zZWQgYmFzZTY0IHVwbG9hZAo=","filename":"gate-closed-base64.txt"}' --output=text
```

```output
{"Result":{"id":378,"name":"gate-closed-base64.txt","size":26,"download_count":0,"created_at":"2026-08-14T20:39:40Z","uuid":"e329ed3e-e608-4cd1-882e-370396ffe3f7","browser_download_url":"https://git.b4mad.industries/attachments/e329ed3e-e608-4cd1-882e-370396ffe3f7"}}
```

## Large release asset streams from `file_path`

A 16 MiB fixture makes the motivating release-asset case concrete. The fixture is generated in the server working directory — inside `FORGEJO_MCP_UPLOAD_ROOT` — uploaded through `file_path`, and then removed locally; the release asset remains available for inspection.

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
{"Result":{"id":379,"name":"forgejo-mcp-streaming-demo.bin","size":16777216,"download_count":0,"created_at":"2026-08-14T20:39:51Z","uuid":"08f9af08-3b80-4fab-b5b5-cf3ed9281174","browser_download_url":"https://git.b4mad.industries/attachments/08f9af08-3b80-4fab-b5b5-cf3ed9281174"}}
```

```bash
rm -f .showboat-large-asset.bin && test ! -e .showboat-large-asset.bin && echo "local fixture removed"
```

```output
local fixture removed
```

## Result

All three attachment surfaces accept host file paths when the operator opts in. Basename defaulting, explicit override, relative resolution, presence-based source validation, zero-byte base64 compatibility, and a large streamed release asset are demonstrated by live Forgejo responses. The gate rejects `file_path` when `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` is unset or not truthy, `FORGEJO_MCP_UPLOAD_ROOT` rejects paths that escape the configured root, and base64 `content` uploads are unaffected by either.
