# Claude Desktop Extension

This directory packages `forgejo-mcp` as a [Claude Desktop Extension](https://www.anthropic.com/engineering/desktop-extensions) (`.mcpb` format), so users of the Claude Desktop app can install Forgejo MCP support with a drag-and-drop or file-picker install rather than hand-editing `claude_desktop_config.json`.

The server binary itself is unchanged — this is purely packaging. Existing usage paths (CLI invocation, `mcpServers` config, `claude mcp add`) keep working identically.

## What's here

- `manifest.json` — the extension manifest (schema version 0.3). Includes the full tools list discovered from the binary's `tools/list` response.
- `test_smoke.py` — verifies the bundled binary boots and that the manifest's tools array matches the binary's actual `tools/list`. The most likely drift bug for an extension wrapping someone else's binary is the manifest's tool list falling out of sync; this catches it.
- `bin/` — destination for the binary at build time (gitignored).

## Build

```sh
# 1. Build the binary into the extension's bin/ directory
go build -o extension/bin/forgejo-mcp .

# 2. Pack the extension
npx @anthropic-ai/mcpb pack extension/ extension/forgejo-mcp.mcpb
```

The resulting `forgejo-mcp.mcpb` is a single file users can drag into Claude Desktop's Extensions settings (or sideload via the file picker — drag-and-drop has been intermittent in some Claude Desktop versions).

## Test

The smoke test verifies the bundled binary's `tools/list` matches the manifest's declared tools.

```sh
# Required: a valid token for whichever Forgejo instance the test will hit.
# A read-only token is sufficient — no write operations are performed.
export FORGEJO_ACCESS_TOKEN="your-token"

# Optional: override the default test URL (https://codeberg.org).
# export FORGEJO_MCP_TEST_URL="https://your-forgejo.example"

python3 extension/test_smoke.py
```

If `FORGEJO_ACCESS_TOKEN` is not set, the test prints a `SKIP:` message and exits 0 — appropriate for unattended CI without secrets.

What the smoke test catches:
- Binary fails to start
- Binary's `tools/list` response omits a tool the manifest declares (or vice versa)
- Manifest is malformed JSON

What it doesn't catch:
- Sideload UX in Claude Desktop (no headless mode for that)
- Configuration form rendering
- End-to-end "user calls a tool, it does the right thing" — that's living human verification

## Updating the manifest's tools list when the binary's tools change

When the binary gains or loses a tool, the manifest's `tools` array needs updating. Two paths:

**Option A: regenerate from the binary.** Build the new binary, run a small one-shot script that calls `tools/list` and rewrites the array in `manifest.json`. This is what was done to seed the initial list. The script lives in commit history; reach for it (or write a fresh equivalent) when needed.

**Option B: edit by hand.** When changes are small (one or two tools), editing directly is fine. The smoke test will fail loudly if the result diverges from the binary.

After either path: `npx @anthropic-ai/mcpb validate manifest.json`, then `python3 extension/test_smoke.py`.

## Versioning

The manifest's `version` field tracks the upstream binary version it was packaged against. Bump it when the bundled binary version changes. It does not need to match a Git tag exactly — Claude Desktop uses it to detect upgrades and display version info to users.

## Possible follow-ups (not part of the initial extension scaffolding)

- Hook the `.mcpb` build into `.goreleaser.yml` so a pre-built archive is produced on each release alongside the platform binaries.
- Add cross-platform binary bundling via the manifest's `server.mcp_config.platform_overrides` field, populated from goreleaser's per-platform builds.
- Submit to [Anthropic's curated extensions directory](https://www.anthropic.com/engineering/desktop-extensions) once the packaging stabilizes — that's the maintainer's call to make.
- Sign the `.mcpb` (`npx @anthropic-ai/mcpb sign`).
