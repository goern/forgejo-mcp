#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""prepare-context step: the authoritative in-pipeline gate (D5 defense-in-depth).

PaC already gated the trigger at the edge (webhook secret, on-comment match,
collaborator policy). This step re-verifies the comment author against the LIVE
Forgejo collaborator/permission API before any tokens are spent, because webhook
payload fields can lie if the HMAC secret leaks. If the author is not authorized
the run fails here, before the agent step.

On success it writes a schema-valid `input.json` (carrying NO credentials) to the
shared workspace for the agent step.

Forgejo I/O goes through an injectable client so this is unit-testable.
"""
from __future__ import annotations

import argparse
import json
import os
import pathlib
import sys
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMAS = ROOT / "schemas"

try:
    from jsonschema import Draft202012Validator
except ImportError:  # pragma: no cover
    Draft202012Validator = None


class AuthorCheck:
    """Returns the author's repo permission, or None if not a collaborator."""

    def permission(self, owner: str, repo: str, user: str) -> str | None:
        raise NotImplementedError


class HttpAuthorCheck(AuthorCheck):
    def __init__(self, base_url: str, token: str):
        self.base = base_url.rstrip("/") + "/api/v1"
        self.headers = {"Authorization": f"token {token}", "Accept": "application/json"}

    def permission(self, owner, repo, user):
        url = f"{self.base}/repos/{owner}/{repo}/collaborators/{user}/permission"
        req = urllib.request.Request(url, headers=self.headers, method="GET")
        try:
            with urllib.request.urlopen(req) as r:
                return (json.loads(r.read()) or {}).get("permission")
        except urllib.error.HTTPError as e:
            if e.code == 404:
                return self._repo_permission(owner, repo)
            if e.code == 403:
                return None
            raise

    def _repo_permission(self, owner, repo):
        url = f"{self.base}/repos/{owner}/{repo}"
        req = urllib.request.Request(url, headers=self.headers, method="GET")
        try:
            with urllib.request.urlopen(req) as r:
                perms = (json.loads(r.read()) or {}).get("permissions") or {}
        except urllib.error.HTTPError as e:
            if e.code in (403, 404):
                return None
            raise
        if perms.get("admin"):
            return "admin"
        if perms.get("push"):
            return "write"
        return None


ALLOWED_PERMISSIONS = {"write", "admin", "owner"}


def is_authorized(checker: AuthorCheck, owner, repo, user, static_allowlist=None) -> bool:
    if static_allowlist and user not in static_allowlist:
        return False
    perm = checker.permission(owner, repo, user)
    return perm in ALLOWED_PERMISSIONS


def build_input(persona, owner, repo, issue_number, author, issue_url,
                command_name, command_args, comment_id=None, is_pull_request=False) -> dict:
    event = {
        "provider": "forgejo",
        "repo": {"owner": owner, "name": repo},
        "kind": "pull_request_comment" if is_pull_request else "issue_comment",
        "issue_number": int(issue_number),
        "author": author,
        "is_pull_request": bool(is_pull_request),
    }
    if comment_id:
        event["comment_id"] = int(comment_id)
    return {
        "persona": persona,
        "event": event,
        "command": {"name": command_name, "args": command_args or []},
        "inputs": {"issue_url": issue_url},
    }


def validate_input(doc: dict) -> None:
    if Draft202012Validator is None:
        raise RuntimeError("jsonschema not installed")
    v = Draft202012Validator(json.loads((SCHEMAS / "input.schema.json").read_text()))
    errs = sorted(v.iter_errors(doc), key=lambda e: e.path)
    if errs:
        raise ValueError(f"built input.json is invalid: {errs[0].message}")


def run(args, checker: AuthorCheck) -> int:
    static = None
    if args.static_allowlist:
        static = {x.strip() for x in args.static_allowlist.split(",") if x.strip()}

    if not is_authorized(checker, args.owner, args.repo, args.author, static):
        print(f"author {args.author!r} not authorized for {args.owner}/{args.repo} "
              f"(live collaborator re-check failed); refusing to run", file=sys.stderr)
        return 2

    doc = build_input(
        args.persona, args.owner, args.repo, args.issue_number, args.author,
        args.issue_url, args.command_name, (args.command_args or "").split(),
        comment_id=args.comment_id, is_pull_request=args.is_pull_request,
    )
    validate_input(doc)
    pathlib.Path(args.out).write_text(json.dumps(doc, indent=2))
    print(f"author authorized; wrote {args.out}")
    return 0


def main(argv=None):
    ap = argparse.ArgumentParser(description="In-pipeline author re-check + input.json builder")
    ap.add_argument("--persona", required=True)
    ap.add_argument("--owner", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--issue-number", required=True)
    ap.add_argument("--author", required=True)
    ap.add_argument("--issue-url", required=True)
    ap.add_argument("--command-name", required=True)
    ap.add_argument("--command-args", default="")
    ap.add_argument("--comment-id", default=None)
    ap.add_argument("--is-pull-request", action="store_true")
    ap.add_argument("--static-allowlist", default=os.environ.get("CA_STATIC_ALLOWLIST", ""))
    ap.add_argument("--out", required=True)
    ap.add_argument("--forgejo-url", default=os.environ.get("FORGEJO_URL", "https://codeberg.org"))
    args = ap.parse_args(argv)

    token = os.environ.get("FORGEJO_READ_TOKEN") or os.environ.get("CODEBERG_TOKEN")
    if not token:
        print("no FORGEJO_READ_TOKEN in env", file=sys.stderr)
        return 1
    return run(args, HttpAuthorCheck(args.forgejo_url, token))


if __name__ == "__main__":
    raise SystemExit(main())
