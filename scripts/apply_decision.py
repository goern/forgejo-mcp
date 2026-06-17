#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Trusted, deterministic apply step (the leash's enforcement end).

Reads a decision envelope, validates it against the pinned envelope + triage
payload schemas, and applies the decision to Forgejo through the ONLY mounted
write credential. The agent's output is treated as data: comments are rendered
here from structured fields via trusted templates — agent text is never posted
as markdown verbatim, and suggested labels are checked against the repo's live
label set.

Exit codes:
  0  applied successfully (or idempotent no-op)
  2  schema-invalid decision  -> nothing applied
  3  status:error decision     -> nothing applied (error surfaced)
  4  decision references an unknown agent-suggested label -> nothing applied

Forgejo I/O goes through the ForgejoClient ABC so unit tests inject a fake.
"""
from __future__ import annotations

import argparse
import dataclasses
import datetime
import hashlib
import json
import os
import pathlib
import sys
import urllib.error
import urllib.request

try:
    from jsonschema import Draft202012Validator
except ImportError:  # pragma: no cover - environment guard
    Draft202012Validator = None

ROOT = pathlib.Path(__file__).resolve().parent.parent
SCHEMAS = ROOT / "schemas"

MARKER_TPL = "<!-- ca-pipeline:{persona}:issue={issue} -->"
BLOCKED_LABEL = "Status/Blocked"
DUPLICATE_LABEL = "Reviewed/Duplicate"


class ForgejoClient:
    def list_repo_labels(self, owner: str, repo: str) -> list[dict]:
        raise NotImplementedError

    def get_issue_labels(self, owner: str, repo: str, index: int) -> list[dict]:
        raise NotImplementedError

    def add_issue_labels(self, owner: str, repo: str, index: int, label_ids: list[int]) -> None:
        raise NotImplementedError

    def list_issue_comments(self, owner: str, repo: str, index: int) -> list[dict]:
        raise NotImplementedError

    def create_comment(self, owner: str, repo: str, index: int, body: str) -> dict:
        raise NotImplementedError

    def edit_comment(self, owner: str, repo: str, comment_id: int, body: str) -> dict:
        raise NotImplementedError


class HttpForgejoClient(ForgejoClient):
    def __init__(self, base_url: str, token: str):
        self.web_base = base_url.rstrip("/")
        self.base = self.web_base + "/api/v1"
        self.headers = {
            "Authorization": f"token {token}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        }

    def _call(self, method: str, path: str, body=None):
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(self.base + path, data=data, headers=self.headers, method=method)
        with urllib.request.urlopen(req) as r:
            raw = r.read()
            return json.loads(raw) if raw else None

    def list_repo_labels(self, owner, repo, index=None):
        out, page = [], 1
        while True:
            batch = self._call("GET", f"/repos/{owner}/{repo}/labels?limit=100&page={page}")
            if not batch:
                break
            out.extend(batch)
            if len(batch) < 100:
                break
            page += 1
        return out

    def get_issue_labels(self, owner, repo, index):
        return self._call("GET", f"/repos/{owner}/{repo}/issues/{index}/labels") or []

    def add_issue_labels(self, owner, repo, index, label_ids):
        self._call("POST", f"/repos/{owner}/{repo}/issues/{index}/labels", {"labels": label_ids})

    def list_issue_comments(self, owner, repo, index):
        return self._call("GET", f"/repos/{owner}/{repo}/issues/{index}/comments") or []

    def create_comment(self, owner, repo, index, body):
        return self._call("POST", f"/repos/{owner}/{repo}/issues/{index}/comments", {"body": body})

    def edit_comment(self, owner, repo, comment_id, body):
        return self._call("PATCH", f"/repos/{owner}/{repo}/issues/comments/{comment_id}", {"body": body})


class RejectedError(Exception):
    def __init__(self, code: int, message: str):
        super().__init__(message)
        self.code = code
        self.message = message


def _load_schema(name: str) -> dict:
    return json.loads((SCHEMAS / name).read_text())


def validate_decision(envelope: dict) -> None:
    if Draft202012Validator is None:
        raise RuntimeError("jsonschema not installed")
    env_v = Draft202012Validator(_load_schema("decision.envelope.schema.json"))
    errs = sorted(env_v.iter_errors(envelope), key=lambda e: e.path)
    if errs:
        raise RejectedError(2, f"envelope invalid: {errs[0].message}")
    if envelope.get("status") == "ok":
        pay_schema = _load_schema("triage.decision.schema.json")
        _clamp_to_schema(envelope.get("decision", {}), pay_schema)
        pay_v = Draft202012Validator(pay_schema)
        perrs = sorted(pay_v.iter_errors(envelope.get("decision", {})), key=lambda e: e.path)
        if perrs:
            raise RejectedError(2, f"triage payload invalid: {perrs[0].message}")


def _clamp_to_schema(decision: dict, schema: dict) -> None:
    props = schema.get("properties", {})
    for field in ("triage_summary",):
        maxlen = props.get(field, {}).get("maxLength")
        val = decision.get(field)
        if maxlen and isinstance(val, str) and len(val) > maxlen:
            decision[field] = val[: maxlen - 1].rstrip() + "…"


def _neutralize(text: str) -> str:
    return (text or "").replace("<!--", "").replace("-->", "").strip()


def render_comment(persona: str, issue: int, decision: dict) -> str:
    action = decision["action"]
    marker = MARKER_TPL.format(persona=persona, issue=issue)
    lines = ["🤖 **Automated triage**", ""]

    if action == "insufficient":
        q = _neutralize(decision["clarification_questions"][0])
        lines += [
            "This issue needs a bit more detail before it can be triaged:",
            "",
            f"> {q}",
            "",
            "_No labels were applied. Please follow up and re-run `/triage`._",
        ]
    elif action == "sufficient":
        summary = _neutralize(decision["triage_summary"])
        labels = ", ".join(f"`{l}`" for l in decision.get("suggested_labels", []))
        lines += [f"**Summary:** {summary}", ""]
        if labels:
            lines.append(f"**Labels applied:** {labels}")
    elif action == "blocked":
        summary = _neutralize(decision.get("triage_summary", ""))
        url = _neutralize(decision["blocking_url"])
        if summary:
            lines += [f"**Summary:** {summary}", ""]
        lines.append(f"**Blocked by:** {url}")
    elif action == "duplicate":
        raw = decision["duplicate_of"]
        if isinstance(raw, str):
            import re as _re
            m = _re.search(r"/issues/(\d+)$", raw)
            n = int(m.group(1)) if m else raw
        else:
            n = raw
        lines.append(f"This appears to duplicate #{n}. Leaving it open for a maintainer to confirm.")
    else:  # pragma: no cover
        raise RejectedError(2, f"unknown action {action!r}")

    lines += ["", marker]
    return "\n".join(lines)


@dataclasses.dataclass
class ApplyResult:
    action: str
    labels_added: list[str]
    labels_skipped: list[str]
    comment_action: str
    comment_id: int | None


def _label_index(repo_labels: list[dict]) -> dict[str, dict]:
    return {l["name"]: l for l in repo_labels}


def _resolve_labels(names, repo_index, *, strict):
    ids, resolved, skipped = [], [], []
    for n in names:
        lbl = repo_index.get(n)
        if lbl is None:
            if strict:
                raise RejectedError(4, f"suggested label not in repo vocabulary: {n!r}")
            skipped.append(n)
            continue
        ids.append(lbl["id"])
        resolved.append(n)
    return ids, resolved, skipped


def apply(envelope: dict, event: dict, client: ForgejoClient) -> ApplyResult:
    persona = envelope["persona"]
    decision = envelope["decision"]
    action = decision["action"]
    owner = event["repo"]["owner"]
    repo = event["repo"]["name"]
    issue = event["issue_number"]

    repo_index = _label_index(client.list_repo_labels(owner, repo))

    if action == "sufficient":
        ids, added, skipped = _resolve_labels(decision.get("suggested_labels", []), repo_index, strict=True)
    elif action == "blocked":
        ids, added, skipped = _resolve_labels([BLOCKED_LABEL], repo_index, strict=False)
    elif action == "duplicate":
        ids, added, skipped = _resolve_labels([DUPLICATE_LABEL], repo_index, strict=False)
    else:
        ids, added, skipped = [], [], []

    if ids:
        current_ids = {l["id"] for l in client.get_issue_labels(owner, repo, issue)}
        net_ids = [i for i in ids if i not in current_ids]
        added = [n for n, i in zip(added, ids) if i in net_ids]
        if net_ids:
            client.add_issue_labels(owner, repo, issue, net_ids)

    marker = MARKER_TPL.format(persona=persona, issue=issue)
    body = render_comment(persona, issue, decision)
    prior = next(
        (c for c in client.list_issue_comments(owner, repo, issue) if marker in (c.get("body") or "")),
        None,
    )
    if prior:
        now = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
        trigger_id = event.get("comment_id")
        web_base = getattr(client, "web_base", None)
        if trigger_id and web_base:
            trigger_url = f"{web_base}/{owner}/{repo}/issues/{issue}#issuecomment-{trigger_id}"
            edit_note = f"\n---\n*Re-triaged {now} · [triggered by comment]({trigger_url})*"
        else:
            edit_note = f"\n---\n*Re-triaged {now}*"
        marker = MARKER_TPL.format(persona=persona, issue=issue)
        body = body.replace(f"\n{marker}", f"{edit_note}\n\n{marker}")
        client.edit_comment(owner, repo, prior["id"], body)
        comment_action, comment_id = "edited", prior["id"]
    else:
        created = client.create_comment(owner, repo, issue, body)
        comment_action, comment_id = "created", (created or {}).get("id")

    return ApplyResult(action, added, skipped, comment_action, comment_id)


def run(decision_path, input_path, audit_path, client) -> int:
    envelope = json.loads(pathlib.Path(decision_path).read_text())
    context = json.loads(pathlib.Path(input_path).read_text())
    event = context["event"]

    audit = {
        "input_digest": hashlib.sha256(pathlib.Path(input_path).read_bytes()).hexdigest(),
        "persona": envelope.get("persona"),
        "status": envelope.get("status"),
        "decision": envelope.get("decision"),
        "outcome": None,
        "applied": None,
        "error": None,
    }

    def finish(code, outcome, *, applied=None, error=None):
        audit["outcome"] = outcome
        audit["applied"] = applied
        audit["error"] = error
        if audit_path:
            pathlib.Path(audit_path).write_text(json.dumps(audit, indent=2))
        return code

    if envelope.get("status") == "error":
        msg = (envelope.get("error") or {}).get("message", "agent reported an error")
        print(f"agent status=error, applying nothing: {msg}", file=sys.stderr)
        return finish(3, "rejected:status-error", error=msg)

    try:
        validate_decision(envelope)
    except RejectedError as e:
        print(f"decision rejected ({e.code}): {e.message}", file=sys.stderr)
        return finish(e.code, "rejected:schema", error=e.message)

    try:
        result = apply(envelope, event, client)
    except RejectedError as e:
        print(f"decision rejected ({e.code}): {e.message}", file=sys.stderr)
        return finish(e.code, "rejected:label", error=e.message)
    except urllib.error.HTTPError as e:  # pragma: no cover
        msg = f"forgejo API error: {e.code} {e.read().decode()[:200]}"
        print(msg, file=sys.stderr)
        return finish(1, "error:forgejo", error=msg)

    print(
        f"applied action={result.action} labels_added={result.labels_added} "
        f"labels_skipped={result.labels_skipped} comment={result.comment_action}"
    )
    return finish(0, "applied", applied=dataclasses.asdict(result))


def main(argv=None):
    ap = argparse.ArgumentParser(description="Trusted apply step for the coding-agent pipeline.")
    ap.add_argument("--decision", required=True, help="path to decision.json")
    ap.add_argument("--input", required=True, help="path to input.json")
    ap.add_argument("--audit", default=None, help="path to write the run audit artifact")
    ap.add_argument("--forgejo-url", default=os.environ.get("FORGEJO_URL", "https://codeberg.org"))
    args = ap.parse_args(argv)

    token = os.environ.get("FORGEJO_WRITE_TOKEN") or os.environ.get("CODEBERG_TOKEN")
    if not token:
        print("no FORGEJO_WRITE_TOKEN in env", file=sys.stderr)
        return 1
    client = HttpForgejoClient(args.forgejo_url, token)
    return run(args.decision, args.input, args.audit, client)


if __name__ == "__main__":
    raise SystemExit(main())
