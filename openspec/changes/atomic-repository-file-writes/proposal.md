## Why

`forgejo-mcp` currently exposes `create_file`, `update_file`, and `delete_file`, but each tool creates an independent repository commit. A caller that needs to change several related files therefore cannot apply the change atomically.

The existing single-file create/update tools also accept plain-text `content` only. They base64-encode that text before calling Forgejo, which prevents callers from supplying arbitrary binary bytes without an unsafe text conversion.

Forgejo's native repository contents API supports an atomic multi-file write, and its file-content representation is Base64. Exposing those capabilities directly lets MCP clients make coherent multi-file commits and write binary files without changing the existing plain-text workflow.

## What Changes

- Add `change_files(owner, repo, message, branch_name, files, new_branch_name?)`.
- `change_files` performs one native Forgejo repository-contents request for the complete batch so all requested create/update/delete operations are committed atomically by Forgejo.
- Validate the complete batch before making the write:
  - at least one operation is required;
  - operations are `create`, `update`, or `delete`;
  - duplicate target paths are rejected;
  - create requires no `sha`;
  - update/delete require the current `sha`;
  - create/update require exactly one of `content` or `content_base64`;
  - delete accepts neither content representation.
- Extend `create_file` and `update_file` with optional `content_base64`.
- For create/update, exactly one of `content` or `content_base64` is required.
- `content` remains plain text and is Base64-encoded by the MCP server as before.
- `content_base64` is strict RFC 4648 standard Base64 and is forwarded unchanged after validation, allowing arbitrary file bytes.
- Explicit empty `content` and empty `content_base64` remain valid and represent a zero-byte file.
- Existing single-file tools remain available and retain their existing plain-text behavior.
- Document `change_files` and the binary-safe content option in the README.

## Capabilities

### New Capabilities

- `atomic-repository-file-writes`: Atomic multi-file create/update/delete commits plus binary-safe repository file write inputs.

### Modified Capabilities

None.

## Impact

- **Affected code**: `operation/repo/change_files.go`, repository file-content validation helpers, `operation/repo/file.go`, repository tool registration, and tests.
- **API surface**: one additive MCP tool (`change_files`) and one additive optional argument (`content_base64`) on `create_file` and `update_file`.
- **No new external dependencies.**
- **No breaking changes** to existing callers using plain-text `content`.
- **Documentation**: README file-tool table describes the new tool and binary-safe write support.

## Out of Scope

- Diff/patch application; that is a separate contribution.
- Milestone CRUD or milestone resources.
- Changes to repository read APIs.
- Client-side emulation of atomicity through multiple single-file calls.
