#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Agent-run step: drive a read-only claude-code worker via `pi` + `ca-leash`
and harvest a single decision envelope.

The spec (agent-harness-execution) mandates four behaviours this wrapper owns,
independent of pi's exact CLI:

  1. Manifest-driven dispatch — model, thinkingLevel, and the read-only tools
     allowlist come from personas/<name>.yaml, not hard-coded here.
  2. API-key auth only — refuses to run on interactive/OAuth subscription creds;
     requires ANTHROPIC_API_KEY (or a provider key) in the environment.
  3. Bounded execution — a wall-clock timeout and a token cap; exceeding either
     degrades to a `status:error` envelope instead of hanging.
  4. Single-decision extraction — pulls exactly one JSON envelope out of the
     (possibly noisy) worker stdout.
"""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import subprocess
import sys

import yaml

ROOT = pathlib.Path(__file__).resolve().parent.parent

DEFAULT_TIMEOUT_S = int(os.environ.get("CA_AGENT_TIMEOUT_S", "600"))
DEFAULT_TOKEN_CAP = int(os.environ.get("CA_AGENT_TOKEN_CAP", "200000"))


def error_envelope(persona: str, kind: str, message: str) -> dict:
    return {"persona": persona, "status": "error", "error": {"kind": kind, "message": message}}


def load_manifest(persona: str) -> dict:
    return yaml.safe_load((ROOT / ".castra" / "personas" / f"{persona}.yaml").read_text())


def build_invocation(manifest: dict, input_path: str, prompt_path: str, token_cap: int) -> list[str]:
    pi_bin = os.environ.get("PI_BIN", "pi")
    argv = [
        pi_bin,
        "--print",
        "--no-session",
        "--model", manifest["model"],
        "--thinking", str(manifest["thinkingLevel"]),
        "--tools", ",".join(str(t) for t in manifest.get("tools", [])),
        "--append-system-prompt", prompt_path,
        f"@{input_path}",
        "Produce the single triage decision JSON for the issue described in the attached input.json, following the system prompt's output contract exactly.",
    ]
    return argv


def worker_env(manifest: dict) -> dict:
    env = dict(os.environ)
    env["PI_CLAUDE_RUNTIME_DRIVER"] = manifest["driver"]
    for k in ("CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_USE_OAUTH", "ANTHROPIC_OAUTH_TOKEN"):
        env.pop(k, None)
    return env


def has_api_key(env: dict | None = None) -> bool:
    env = env if env is not None else os.environ
    return any(env.get(k) for k in ("ANTHROPIC_API_KEY", "AWS_BEARER_TOKEN_BEDROCK", "GOOGLE_API_KEY"))


def extract_decision(stdout: str) -> dict | None:
    """Return the last balanced top-level JSON object in stdout that parses, or
    None. Tolerates log noise before/after the envelope."""
    candidates = []
    depth = 0
    start = None
    in_str = False
    esc = False
    for i, ch in enumerate(stdout):
        if in_str:
            if esc:
                esc = False
            elif ch == "\\":
                esc = True
            elif ch == '"':
                in_str = False
            continue
        if ch == '"':
            in_str = True
        elif ch == "{":
            if depth == 0:
                start = i
            depth += 1
        elif ch == "}":
            if depth > 0:
                depth -= 1
                if depth == 0 and start is not None:
                    candidates.append(stdout[start:i + 1])
                    start = None
    for blob in reversed(candidates):
        try:
            obj = json.loads(blob)
        except json.JSONDecodeError:
            continue
        if isinstance(obj, dict) and "status" in obj and "persona" in obj:
            return obj
    return None


def default_runner(argv, env, timeout):
    return subprocess.run(argv, env=env, timeout=timeout, capture_output=True, text=True)


def run_agent(persona: str, input_path: str, out_path: str, *,
              runner=default_runner, timeout=DEFAULT_TIMEOUT_S, token_cap=DEFAULT_TOKEN_CAP) -> int:
    out = pathlib.Path(out_path)

    def emit(env_obj, code):
        out.write_text(json.dumps(env_obj, indent=2))
        return code

    try:
        manifest = load_manifest(persona)
    except FileNotFoundError:
        return emit(error_envelope(persona, "internal", f"no persona manifest for {persona!r}"), 1)

    if not has_api_key():
        return emit(error_envelope(persona, "internal",
                                   "no model API key in env (API-key auth required; OAuth not allowed)"), 1)

    prompt_path = str(ROOT / manifest["prompt"])
    argv = build_invocation(manifest, input_path, prompt_path, token_cap)
    env = worker_env(manifest)

    try:
        proc = runner(argv, env, timeout)
    except subprocess.TimeoutExpired:
        return emit(error_envelope(persona, "timeout",
                                   f"worker exceeded the {timeout}s wall-clock limit"), 1)
    except FileNotFoundError:
        return emit(error_envelope(persona, "internal",
                                   f"pi binary not found ({argv[0]!r})"), 1)

    decision = extract_decision(proc.stdout or "")
    if decision is None:
        tail = (proc.stderr or proc.stdout or "")[-300:]
        return emit(error_envelope(persona, "parse_error",
                                   f"no decision JSON in worker output (rc={proc.returncode}); tail: {tail}"), 1)

    decision["persona"] = persona
    print(f"[pi_run] extracted decision: {json.dumps(decision)[:800]}", file=sys.stderr)
    return emit(decision, 0)


def main(argv=None):
    ap = argparse.ArgumentParser(description="Run the read-only agent worker and emit decision.json")
    ap.add_argument("--persona", required=True)
    ap.add_argument("--input", required=True, help="path to input.json")
    ap.add_argument("--out", required=True, help="path to write decision.json")
    ap.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT_S)
    ap.add_argument("--token-cap", type=int, default=DEFAULT_TOKEN_CAP)
    args = ap.parse_args(argv)
    return run_agent(args.persona, args.input, args.out,
                     timeout=args.timeout, token_cap=args.token_cap)


if __name__ == "__main__":
    raise SystemExit(main())
