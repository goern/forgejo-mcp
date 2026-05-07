#!/usr/bin/env python3
"""Smoke test for the forgejo-mcp Claude Desktop Extension.

Boots the bundled binary in stdio mode, performs the MCP handshake, and verifies
that the binary's tools/list response matches the manifest's declared `tools`
array. Catches the most likely drift bug for an extension wrapping someone
else's binary: the manifest's tool inventory falls out of sync with what the
binary actually exposes.

What this DOES test:
  - Binary starts and speaks MCP over stdio
  - Manifest is well-formed JSON
  - Manifest tools[] matches the binary's tools/list (set equality on names)

What this does NOT test:
  - Actual API behavior against a real Forgejo instance (no network)
  - Sideload UX in Claude Desktop (no headless mode for that)
  - Configuration form rendering

Usage:
    python3 extension/test_smoke.py
    python3 extension/test_smoke.py --binary path/to/forgejo-mcp
    python3 extension/test_smoke.py --manifest path/to/manifest.json

Exits 0 on success, 1 on any mismatch or error.

Dependencies: Python 3.9+ stdlib only. No third-party packages.
"""

import argparse
import json
import os
import subprocess
import sys
import threading
from pathlib import Path

PROTOCOL_VERSION = "2024-11-05"
HANDSHAKE_TIMEOUT_SECONDS = 10
TOOLS_LIST_TIMEOUT_SECONDS = 10


def send_message(proc, payload):
    """Write one line-delimited JSON-RPC message to the server's stdin."""
    line = json.dumps(payload) + "\n"
    proc.stdin.write(line.encode())
    proc.stdin.flush()


def read_response(proc, timeout):
    """Read one line-delimited JSON-RPC response from the server's stdout.

    Uses a thread + Event to enforce a timeout, since stdout.readline() blocks.
    """
    result = {"line": None, "error": None}
    done = threading.Event()

    def reader():
        try:
            result["line"] = proc.stdout.readline()
        except Exception as exc:  # pragma: no cover
            result["error"] = exc
        finally:
            done.set()

    thread = threading.Thread(target=reader, daemon=True)
    thread.start()
    if not done.wait(timeout):
        raise TimeoutError(f"No response within {timeout}s")
    if result["error"]:
        raise result["error"]
    if not result["line"]:
        # Drain stderr for diagnostic context if the server died.
        stderr = proc.stderr.read().decode(errors="replace")
        raise RuntimeError(
            "Server closed stdout without responding."
            + (f"\n--- stderr ---\n{stderr}" if stderr else "")
        )
    return json.loads(result["line"])


def run_smoke(binary_path: Path, manifest_path: Path) -> int:
    if not binary_path.exists():
        print(f"FAIL: binary not found at {binary_path}", file=sys.stderr)
        return 1
    if not os.access(binary_path, os.X_OK):
        print(f"FAIL: binary at {binary_path} is not executable", file=sys.stderr)
        return 1
    if not manifest_path.exists():
        print(f"FAIL: manifest not found at {manifest_path}", file=sys.stderr)
        return 1

    with manifest_path.open() as f:
        manifest = json.load(f)
    manifest_tools = {t["name"] for t in manifest.get("tools", [])}
    if not manifest_tools:
        print("FAIL: manifest declares no tools", file=sys.stderr)
        return 1

    # The binary calls VerifyConnection on startup, which validates that the
    # token is real (not just that the URL resolves). So this smoke test needs
    # a valid token in FORGEJO_ACCESS_TOKEN. A read-only token is plenty —
    # we only call metadata endpoints (initialize, tools/list).
    #
    # If no token is set, skip rather than fail so unattended CI without
    # secrets passes cleanly. Local runs and CI runs with a configured
    # secret get full coverage.
    if not os.environ.get("FORGEJO_ACCESS_TOKEN"):
        print("SKIP: FORGEJO_ACCESS_TOKEN not set — this test needs a real Forgejo")
        print("      token to run. Local: export FORGEJO_ACCESS_TOKEN=<your-token>.")
        print("      CI: configure as a secret. A read-only token is sufficient.")
        return 0

    url = os.environ.get("FORGEJO_MCP_TEST_URL", "https://codeberg.org")
    env = os.environ.copy()
    proc = subprocess.Popen(
        [
            str(binary_path),
            "--transport", "stdio",
            "--url", url,
        ],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        env=env,
    )

    try:
        # MCP initialize handshake.
        send_message(proc, {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {},
                "clientInfo": {"name": "forgejo-mcp-smoke-test", "version": "0.1"},
            },
        })
        init_resp = read_response(proc, HANDSHAKE_TIMEOUT_SECONDS)
        if init_resp.get("error"):
            print(f"FAIL: initialize returned error: {init_resp['error']}", file=sys.stderr)
            return 1

        # initialized notification (no response expected).
        send_message(proc, {
            "jsonrpc": "2.0",
            "method": "notifications/initialized",
        })

        # tools/list — what we actually care about.
        send_message(proc, {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/list",
        })
        tools_resp = read_response(proc, TOOLS_LIST_TIMEOUT_SECONDS)
        if tools_resp.get("error"):
            print(f"FAIL: tools/list returned error: {tools_resp['error']}", file=sys.stderr)
            return 1
        binary_tools = {t["name"] for t in tools_resp.get("result", {}).get("tools", [])}
    finally:
        # Clean shutdown — close stdin, give the process a moment, then kill.
        try:
            proc.stdin.close()
        except Exception:
            pass
        try:
            proc.wait(timeout=2)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()

    # Compare.
    only_in_manifest = manifest_tools - binary_tools
    only_in_binary = binary_tools - manifest_tools

    if not only_in_manifest and not only_in_binary:
        print(f"PASS: manifest and binary agree on {len(manifest_tools)} tools")
        return 0

    print("FAIL: manifest tool inventory disagrees with the binary's tools/list", file=sys.stderr)
    if only_in_manifest:
        print(f"  In manifest but not in binary ({len(only_in_manifest)}):", file=sys.stderr)
        for name in sorted(only_in_manifest):
            print(f"    - {name}", file=sys.stderr)
    if only_in_binary:
        print(f"  In binary but not in manifest ({len(only_in_binary)}):", file=sys.stderr)
        for name in sorted(only_in_binary):
            print(f"    - {name}", file=sys.stderr)
    return 1


def main():
    here = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument(
        "--binary",
        type=Path,
        default=here / "bin" / "forgejo-mcp",
        help="Path to the forgejo-mcp binary (default: extension/bin/forgejo-mcp)",
    )
    parser.add_argument(
        "--manifest",
        type=Path,
        default=here / "manifest.json",
        help="Path to the extension manifest (default: extension/manifest.json)",
    )
    args = parser.parse_args()
    sys.exit(run_smoke(args.binary, args.manifest))


if __name__ == "__main__":
    main()
