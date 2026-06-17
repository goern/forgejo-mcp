#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Configure Codeberg webhooks for goern/forgejo-mcp.

Registers two webhooks:
  1. op1st-pipelines PaC controller  — push + pull_request + issue_comment
  2. arena-forge EventListener        — issue_comment only (plain-issue triage, deferred)

Usage:
    python scripts/configure-webhooks.py goern/forgejo-mcp

Env vars:
    CODEBERG_TOKEN        Codeberg personal access token (required)
    CODEBERG_URL          Codeberg base URL (default: https://codeberg.org)

Webhook secrets are read from the cluster via `oc get secret`. The caller
must be logged in to the right cluster/namespace context.
"""

import json
import os
import subprocess
import sys
import urllib.request
import urllib.error


CODEBERG_URL = os.environ.get("CODEBERG_URL", "https://codeberg.org")

PAC_WEBHOOK_URL = (
    "https://pipelines-as-code-controller-openshift-pipelines"
    ".apps.nostromo.erdgeschoss.b4mad.emea.operate-first.cloud"
)
EL_WEBHOOK_URL = "https://issue-triage.arena-forge.webhooks.b4mad.industries"

PAC_SECRET_NAME = "codeberg-org-op1st-pipelines"
PAC_SECRET_KEY = "webhook.secret"
PAC_SECRET_NS = "op1st-pipelines"

EL_SECRET_NS = "op1st-pipelines"
EL_SECRET_KEY = "secretToken"


def oc_get_secret(name: str, key: str, namespace: str) -> str:
    result = subprocess.run(
        [
            "oc", "get", "secret", name,
            "-n", namespace,
            "-o", f"jsonpath={{.data.{key.replace('.', '\\.')}}}",
        ],
        capture_output=True, text=True, check=True,
    )
    import base64
    return base64.b64decode(result.stdout.strip()).decode()


def api(token: str, method: str, path: str, body: dict | None = None):
    url = f"{CODEBERG_URL}/api/v1{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(
        url, data=data, method=method,
        headers={
            "Authorization": f"token {token}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        print(f"  HTTP {e.code} {method} {url}: {body}", file=sys.stderr)
        raise


def list_hooks(token: str, owner: str, repo: str) -> list:
    return api(token, "GET", f"/repos/{owner}/{repo}/hooks")


def find_hook(hooks: list, url: str) -> dict | None:
    for h in hooks:
        if h.get("config", {}).get("url") == url:
            return h
    return None


def upsert_hook(
    token: str, owner: str, repo: str,
    hooks: list, hook_url: str, secret: str, events: list[str],
    label: str,
):
    existing = find_hook(hooks, hook_url)
    payload = {
        "active": True,
        "config": {
            "content_type": "json",
            "secret": secret,
            "url": hook_url,
        },
        "events": events,
        "type": "forgejo",
    }
    if existing:
        hook_id = existing["id"]
        api(token, "PATCH", f"/repos/{owner}/{repo}/hooks/{hook_id}", payload)
        print(f"  [{label}] updated hook #{hook_id} → {hook_url}")
    else:
        result = api(token, "POST", f"/repos/{owner}/{repo}/hooks", payload)
        print(f"  [{label}] created hook #{result['id']} → {hook_url}")


def main():
    if len(sys.argv) != 2 or "/" not in sys.argv[1]:
        print("Usage: configure-webhooks.py <owner>/<repo>", file=sys.stderr)
        sys.exit(1)

    owner, repo = sys.argv[1].split("/", 1)
    token = os.environ.get("CODEBERG_TOKEN")
    if not token:
        print("CODEBERG_TOKEN not set", file=sys.stderr)
        sys.exit(1)

    print(f"Configuring webhooks for {owner}/{repo} ...")

    print(f"  Reading PaC secret {PAC_SECRET_NAME}/{PAC_SECRET_KEY} ...")
    pac_secret = oc_get_secret(PAC_SECRET_NAME, PAC_SECRET_KEY, PAC_SECRET_NS)

    el_secret_name = (
        f"arena-forge-eventlistener-webhook-{owner.lower()}-{repo.lower().replace('/', '-')}"
    )
    print(f"  Reading EL secret {el_secret_name}/{EL_SECRET_KEY} ...")
    try:
        el_secret = oc_get_secret(el_secret_name, EL_SECRET_KEY, EL_SECRET_NS)
    except subprocess.CalledProcessError:
        print(
            f"  ⚠️  Secret {el_secret_name} not found in {EL_SECRET_NS}.\n"
            f"     Create it first:\n"
            f"     oc create secret generic {el_secret_name} \\\n"
            f"       -n {EL_SECRET_NS} \\\n"
            f"       --from-literal={EL_SECRET_KEY}=<random-hmac-secret>",
            file=sys.stderr,
        )
        sys.exit(1)

    hooks = list_hooks(token, owner, repo)
    print(f"  Found {len(hooks)} existing hook(s).")

    upsert_hook(
        token, owner, repo, hooks,
        hook_url=PAC_WEBHOOK_URL,
        secret=pac_secret,
        events=["push", "pull_request", "issue_comment"],
        label="pac",
    )
    upsert_hook(
        token, owner, repo, hooks,
        hook_url=EL_WEBHOOK_URL,
        secret=el_secret,
        events=["issue_comment"],
        label="el-triage",
    )

    print("Done.")


if __name__ == "__main__":
    main()
