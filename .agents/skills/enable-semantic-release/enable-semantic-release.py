#!/usr/bin/env python3

# SPDX-FileCopyrightText: 2026 Christoph Görn
#
# SPDX-License-Identifier: GPL-3.0-or-later

"""Grant b4mad-release-agent what it needs to run semantic-release on a repo.

WHY a script: the answer is never "give the bot write access". It is a chain of
four independent things, and this session proved that getting three of them
right still fails at the push. Every one of the checks below exists because it
actually bit:

  1. The agent has an SSH key and a GPG key on its account.
     Without the GPG key the release commit pushes fine and lands UNVERIFIED,
     which is the exact failure signing was meant to prevent — and nothing in
     the run reports it, because the push succeeded.

  2. The agent has write on the repo's CODE unit.
     ⚠️ THE TRAP. release-bots is a team whose nominal permission is "write",
     which reads as sufficient and is not: its units were packages + releases
     only. Forgejo authorises a branch push against unit.TypeCode specifically,
     so the agent held "write" and was still rejected with

         Forgejo: User 'b4mad-release-agent' is not allowed to push to
         branch 'main' in 'agentic-forges/semantic-release'.

     — a message that reads like branch protection and is not. A tag push goes
     through the same unit, so dropping @semantic-release/git does not dodge it.

  3. Branch protection permits the push.
     ⚠️ THE SECOND TRAP, and the reason --force is not just a yes-flag. When a
     rule has enable_push_whitelist=false, EVERY user with code-write may push;
     the allowlist is not in force. "Adding the agent to the allowlist" then
     means switching the allowlist ON, which locks out everyone who can push
     today. That is a restriction disguised as a grant, so this script will
     append to an allowlist that is already enabled and REFUSES to enable one.

  4. The repo actually has a semantic-release config.
     Cheap to check, and a missing .releaserc turns into a confusing runtime
     error inside the Job instead of an answer here.

  5. The repo can DRIVE a release without anyone remembering the incantation,
     and can TELL the next agent how. Grant done, the next question is always
     "so how do I release?" — answered twice: a block of just targets in the
     repo's justfile (preflight, release, release-dry, clean-jobs), and ten
     lines in its AGENTS.md pointing at them.

     Both are MANAGED BLOCKS, one implementation (class Managed), fenced in
     markers carrying this skill's version AND a hash of the block body,
     because the two questions asked later are different: the version answers
     "is there a newer one", the hash answers "is it still mine to replace". An
     unedited older block is updated in place; an edited one is never touched
     without --force-block. What differs per block is the comment syntax, the
     candidate filenames, and what counts as "something like this is already
     here" — a justfile clashes on NAMES and that is fatal, Markdown clashes on
     CONTENT and that is merely a warning.

WHAT IT DOES NOT DO: it does not run the release. It prints the `oc create`
that does, so the grant and the run stay separately auditable.

Usage:

    ./enable-semantic-release.py ssh://git@git.b4mad.industries:2222/org/repo.git
    ./enable-semantic-release.py <url> --dry-run     # survey + plan, change nothing
    ./enable-semantic-release.py <url> --force       # allow the widening steps
    ./enable-semantic-release.py <url> --skip grant  # the local blocks only
    ./enable-semantic-release.py <url> --only grant  # the forge grant only

The SSH form is required rather than accepted-among-others: it is the exact
string the Job's REPO_URL takes, so a URL this script blesses is one that can be
pasted into the manifest unchanged.

RUNS FROM ANYWHERE, FETCHES NOTHING. It is installed as a skill as well as kept
in forge-agents, so it reads nothing from the cwd: the Job manifest and both
block templates ship beside it and are resolved against __file__. The one
deliberate exception is the files it INSTALLS — those have to land in a repo,
and the repo is the cwd's git top level unless --repo-dir says otherwise.

The only things it needs from the environment are an `oc` logged in to the
cluster holding namespace b4mad-forgejo, and that cluster's Forgejo — no source
repo, no package index. The managed blocks need neither, which is why
`--skip grant` exists and is testable with nothing but git.

Exit codes: 0 done (or a clean dry run) · 1 failure, including a clash no flag
can lift · 2 the plan needs --force or --force-block
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import re
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

# Deliberately not ${FORGEJO_URL:-…}; see create-forge-agent.py's note — the
# environment exports FORGEJO_URL=https://codeberg.org for the Codeberg CLI,
# which would point admin credentials at the wrong forge.
FORGEJO_URL = "https://git.b4mad.industries"
SSH_HOST, SSH_PORT = "git.b4mad.industries", 2222
NAMESPACE = "b4mad-forgejo"
ADMIN_SECRET = "forgejo-admin"

AGENT = "b4mad-release-agent"
AGENT_SECRET = f"forgejo-agent-{AGENT}"
# The org team that carries the release identity. Membership is not re-derived
# per repo: one team is the thing a human can audit in a single screen.
TEAM = "release-bots"
# The org where TEAM already exists in the shape that works — quoted verbatim in
# the "no such team" failure so the remedy is a copy, not a design decision.
TEAM_HOME = "agentic-forges"
# Forgejo's name for the unit that gates `git push` — the whole point of #2.
CODE_UNIT = "repo.code"

# Where semantic-release looks. Only the JSON/YAML forms cosmiconfig finds
# without a package.json, which is what these repos have.
CONFIG_FILES = (".releaserc", ".releaserc.json", ".releaserc.yaml",
                ".releaserc.yml", ".releaserc.js", "release.config.js")

# The Job template the final command edits. It SHIPS BESIDE THIS SCRIPT — the
# pair is the skill, and forge-agents reaches both through symlinks (root, and
# openshift/) rather than keeping second copies. Resolved against __file__, so
# it is found from any cwd and needs no network to read.
MANIFEST = "job-semantic-release.yaml"

# THE SINGLE SOURCE OF TRUTH for "which version of this skill wrote that block".
# Nothing else in the skill states a version: SKILL.md points here, and the
# release stamps here. semantic-release rewrites this literal in .releaserc's
# prepareCmd just before tarring the skill up, so a released tarball carries its
# release's version and nobody bumps anything by hand. In a forge-agents
# checkout it therefore reads 0.0.0-dev, which is the truth — the checkout is
# not any release — and makes every installed release look newer than it.
SKILL_VERSION = "1.4.1"

# The slug baked into the shipped Job manifest, which the installed `release`
# target rewrites to the local one. A no-op in the repo it ships from.
MANIFEST_HOME_SLUG = "agentic-forges/forge-agents"

# MARKER FENCES live in Managed, built once from the format strings the writer
# uses so the reader cannot drift from the writer. Both facts appear on BOTH
# lines: a half-deleted fence is then a mismatch rather than a silent
# truncation. Kept greppable and one-line — a human scanning a file must be
# able to see where the managed part starts without parsing anything.


def log(msg: str) -> None:
    print(msg, file=sys.stderr, flush=True)


class Fail(RuntimeError):
    pass


def run(cmd: list[str], *, check: bool = True) -> subprocess.CompletedProcess:
    p = subprocess.run(cmd, capture_output=True)
    if check and p.returncode != 0:
        raise Fail(f"{cmd[0]} failed ({p.returncode}): "
                   f"{p.stderr.decode(errors='replace').strip()}")
    return p


def api(method: str, path: str, *, auth, body=None, ok=(200, 201, 204)):
    """Call the Forgejo API. `auth` is (user, password) — admin basic auth.

    404 listed in `ok` means "absence is an expected answer" and returns None,
    rather than the error body, which is itself valid JSON and would read as a
    truthy result to an existence check.
    """
    req = urllib.request.Request(f"{FORGEJO_URL}/api/v1{path}", method=method)
    blob = base64.b64encode(f"{auth[0]}:{auth[1]}".encode()).decode()
    req.add_header("Authorization", f"Basic {blob}")
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, data, timeout=60) as r:
            raw, code = r.read(), r.status
    except urllib.error.HTTPError as e:
        raw, code = e.read(), e.code
    if code not in ok:
        raise Fail(f"{method} {path} -> HTTP {code}: "
                   f"{raw.decode(errors='replace')[:400]}")
    if code == 404:
        return None
    return json.loads(raw) if raw.strip() else None


def find_bundled(name: str) -> Path:
    """A file shipped beside this script.

    Resolved against __file__ and never the cwd: run as a skill the cwd is
    whatever repo the user happens to be in, and a job-semantic-release.yaml
    found there would be the wrong file rather than no file. resolve() first —
    forge-agents keeps a root symlink to this script, and the bundled files sit
    next to the real one.
    """
    here = Path(__file__).resolve().parent
    path = here / name
    if not path.is_file():
        raise Fail(f"{name} is missing from {here}. It ships with this "
                   f"script — they are one skill. Reinstall the skill, or "
                   f"copy the file from agentic-forges/forge-agents.")
    return path


def find_manifest() -> Path:
    return find_bundled(MANIFEST)


def admin_credentials() -> tuple[str, str]:
    """Read gitea_admin's break-glass credentials from the live Secret — what is
    deployed is what the API will accept (see create-forge-agent.py)."""
    out = run(["oc", "-n", NAMESPACE, "get", "secret", ADMIN_SECRET,
               "-o", "json"]).stdout
    data = json.loads(out)["data"]
    return (base64.b64decode(data["username"]).decode(),
            base64.b64decode(data["password"]).decode())


def parse_repo_url(url: str) -> tuple[str, str]:
    """A clone URL for this instance -> (owner, repo), or raise.

    Accepts the three forms git itself hands out — ssh://git@host:port/o/r.git,
    https://host/o/r.git, and git@host:o/r.git — because the caller is no longer
    only a human typing the Job's REPO_URL. `just preflight` passes whatever
    `git remote get-url` returns for the release remote, and `upstream` is very
    often added as HTTPS; rejecting that made the grant check fail on a repo
    that was perfectly releasable.

    Still pedantic about WHICH FORGE, which was always the point: a URL that
    parses but points elsewhere would produce a plan full of confident nonsense
    about the wrong instance. The scheme was never what protected against that
    — the host is — so the host is checked in every form, and the port in the
    one form that carries it.
    """
    forms = (
        r"ssh://git@(?P<host>[^:/]+):(?P<port>\d+)/(?P<owner>[^/]+)/(?P<repo>.+?)(?:\.git)?/?",
        r"https://(?P<host>[^:/]+)/(?P<owner>[^/]+)/(?P<repo>.+?)(?:\.git)?/?",
        r"git@(?P<host>[^:/]+):(?P<owner>[^/]+)/(?P<repo>.+?)(?:\.git)?/?",
    )
    m = next((m for m in (re.fullmatch(f, url) for f in forms) if m), None)
    if not m:
        raise Fail(
            f"not a clone URL: {url!r}\n"
            f"   expected ssh://git@{SSH_HOST}:{SSH_PORT}/<owner>/<repo>.git, "
            f"https://{SSH_HOST}/<owner>/<repo>.git, "
            f"or git@{SSH_HOST}:<owner>/<repo>.git")
    host = m.group("host")
    if host != SSH_HOST:
        raise Fail(f"{host} is not this instance ({SSH_HOST})")
    # Only the ssh:// form carries a port, and a wrong one there is the same
    # class of mistake as a wrong host: it is not this instance.
    port = m.groupdict().get("port")
    if port is not None and int(port) != SSH_PORT:
        raise Fail(f"{host}:{port} is not this instance ({SSH_HOST}:{SSH_PORT})")
    return m.group("owner"), m.group("repo")


class Step:
    """One line of the plan. `gated` marks a step that widens access beyond the
    one repo being enabled, and therefore may only run under --force."""

    def __init__(self, verb: str, text: str, *, gated: bool = False):
        self.verb, self.text, self.gated = verb, text, gated

    def render(self) -> str:
        return f"  {self.verb:<8} {self.text}" + ("   [--force]" if self.gated else "")


def survey(auth, owner: str, repo: str) -> dict:
    """Read every fact the plan depends on. Read-only, always."""
    s: dict = {"owner": owner, "repo": repo}

    s["repo_obj"] = api("GET", f"/repos/{owner}/{repo}", auth=auth, ok=(200, 404))
    if s["repo_obj"] is None:
        raise Fail(f"no such repo: {owner}/{repo}")
    s["branch"] = s["repo_obj"].get("default_branch") or "main"

    # (1) credentials on the account itself.
    s["ssh_keys"] = api("GET", f"/users/{AGENT}/keys", auth=auth, ok=(200, 404)) or []
    s["gpg_keys"] = api("GET", f"/users/{AGENT}/gpg_keys", auth=auth, ok=(200, 404)) or []

    # (2) effective permission, and the team that is supposed to grant it.
    perm = api("GET", f"/repos/{owner}/{repo}/collaborators/{AGENT}/permission",
               auth=auth, ok=(200, 404))
    s["permission"] = (perm or {}).get("permission")

    # A personal account has no teams at all, and /orgs/<user>/teams 404s exactly
    # like an org whose team is merely missing. The two need different advice, so
    # ask what the owner *is* rather than inferring it from the absence.
    s["owner_is_org"] = api("GET", f"/orgs/{owner}",
                            auth=auth, ok=(200, 404)) is not None

    # Direct collaborator is a different fact from the effective permission
    # above: the latter is also 'write' when a team grants it, and on a personal
    # repo it is the only grant Forgejo offers at all.
    # The per-user endpoint answers 204/404 with an empty body, which `api` cannot
    # tell apart from an absent JSON payload — so read the list, which is JSON.
    s["collaborator"] = AGENT in [
        c["login"] for c in
        api("GET", f"/repos/{owner}/{repo}/collaborators", auth=auth,
            ok=(200, 404)) or []]

    s["team"] = None
    for t in api("GET", f"/orgs/{owner}/teams", auth=auth, ok=(200, 404)) or []:
        if t["name"] == TEAM:
            # The list endpoint omits units_map; the single-team read has it.
            s["team"] = api("GET", f"/teams/{t['id']}", auth=auth)
            break
    if s["team"]:
        s["team_repos"] = [r["full_name"] for r in
                           api("GET", f"/teams/{s['team']['id']}/repos",
                               auth=auth, ok=(200, 404)) or []]
        s["team_members"] = [m["login"] for m in
                             api("GET", f"/teams/{s['team']['id']}/members",
                                 auth=auth, ok=(200, 404)) or []]
    else:
        s["team_repos"], s["team_members"] = [], []

    # (3) branch protection on the branch semantic-release will push.
    s["protection"] = api(
        "GET", f"/repos/{owner}/{repo}/branch_protections/"
               f"{urllib.parse.quote(s['branch'], safe='')}",
        auth=auth, ok=(200, 404))

    # (4) a semantic-release config in the repo.
    s["config"] = None
    for name in CONFIG_FILES:
        if api("GET", f"/repos/{owner}/{repo}/contents/{name}",
               auth=auth, ok=(200, 404)) is not None:
            s["config"] = name
            break

    # The Secret the Job mounts. Absent means the SealedSecret is not applied —
    # the failure this repo's openshift/README.md walks through.
    s["secret"] = run(["oc", "-n", NAMESPACE, "get", "secret", AGENT_SECRET],
                      check=False).returncode == 0
    return s


def missing_team_message(s: dict) -> str:
    """The '<TEAM> is missing' failure, written so the reader can act on it.

    Two different situations 404 identically, and only one of them is fixable by
    creating a team: Forgejo has no teams on personal accounts. Telling an agent
    to "create it first" on a user-owned repo sends it looking for a button that
    does not exist, so each case gets its own remedy.
    """
    owner, full = s["owner"], f"{s['owner']}/{s['repo']}"

    if not s["owner_is_org"]:
        return (
            f"'{owner}' is a personal account, not an organisation — Forgejo has "
            f"no teams there, so {full} cannot be granted the way this script "
            f"grants.\n"
            f"   Two ways forward, and they are a real choice, not a formality:\n"
            f"   a) Move the repo under an org that carries '{TEAM}' (today: "
            f"{TEAM_HOME}) and re-run this script unchanged:\n"
            f"        Repo → Settings → Danger Zone → Transfer Ownership → "
            f"{TEAM_HOME}\n"
            f"      Cost: the repo's URL changes; the old path redirects but any "
            f"REPO_URL pinned elsewhere should be updated.\n"
            f"   b) Add {AGENT} as a direct collaborator with write on {full}:\n"
            f"        {collaborator_cmd(owner, s['repo'])}\n"
            f"      Cost: this grant is invisible in the one place a human audits "
            f"bot access, and nothing removes it when the repo is done releasing. "
            f"Prefer (a) unless the repo is deliberately personal.\n"
            f"   Either way, this is the only thing missing: the keys, the "
            f"Secret and the .releaserc all checked out before this point.")

    return (
        f"org '{owner}' has no '{TEAM}' team. Create it first — this script "
        f"grants through a team on purpose, so the blast radius is auditable in "
        f"one place.\n"
        f"   The team must carry {CODE_UNIT}=write; that is the unit `git push` "
        f"is authorised against, and a team without it reads as 'write' and "
        f"still rejects the release push.\n"
        f"   Copy the shape already in use at {TEAM_HOME}:\n"
        f"        {create_team_cmd(owner)}\n"
        f"   Then re-run this script — it will add {AGENT} and {full} to the "
        f"team itself.")


def _admin_curl(method: str, path: str, body: dict | None = None) -> str:
    """A copy-pasteable curl for one API call, reading the same live admin
    Secret this script reads. Printed rather than executed: creating a team is
    an org-level act, so it stays a human's (or a deliberate agent's) keystroke.
    """
    creds = (f'"$(oc -n {NAMESPACE} get secret {ADMIN_SECRET} '
             f"-o go-template='{{{{.data.username|base64decode}}}}"
             f":{{{{.data.password|base64decode}}}}')\"")
    cmd = (f"curl -sS -u {creds} -X {method} "
           f"-H 'Content-Type: application/json' "
           f"{FORGEJO_URL}/api/v1{path}")
    if body is not None:
        cmd += f" \\\n          -d '{json.dumps(body)}'"
    return cmd


def create_team_cmd(owner: str) -> str:
    return _admin_curl("POST", f"/orgs/{owner}/teams", {
        "name": TEAM,
        "description": "CI identities that publish container packages and "
                       "release assets. Also code:write, so semantic-release "
                       "can push its CHANGELOG commit and signed tag.",
        "permission": "write",
        "units": ["repo.code", "repo.packages", "repo.releases"],
        "units_map": {"repo.code": "write", "repo.packages": "write",
                      "repo.releases": "write"},
        "includes_all_repositories": False,
        "can_create_org_repo": False,
    })


def collaborator_cmd(owner: str, repo: str) -> str:
    return _admin_curl("PUT", f"/repos/{owner}/{repo}/collaborators/{AGENT}",
                       {"permission": "write"})


def build_plan(s: dict) -> list[Step]:
    steps: list[Step] = []
    full = f"{s['owner']}/{s['repo']}"
    team, units = s["team"], (s["team"] or {}).get("units_map") or {}

    if not s["ssh_keys"]:
        raise Fail(f"{AGENT} has no SSH key — it cannot clone or push at all.\n"
                   f"   Run: ./create-forge-agent.py {AGENT} --overwrite")
    if not s["gpg_keys"]:
        raise Fail(f"{AGENT} has no GPG key — the release commit would land "
                   f"unverified.\n"
                   f"   Run: ./create-forge-agent.py {AGENT} --overwrite")
    steps.append(Step("ok", f"{AGENT} has an SSH key and a GPG key "
                            f"({s['gpg_keys'][0].get('key_id')})"))

    if not s["secret"]:
        raise Fail(f"Secret {AGENT_SECRET} is missing from namespace "
                   f"{NAMESPACE} — the Job has nothing to mount.\n"
                   f"   The SealedSecret must be listed in the b4mad-forgejo "
                   f"kustomization; see openshift/README.md.")
    steps.append(Step("ok", f"Secret {AGENT_SECRET} is applied in {NAMESPACE}"))

    if s["config"] is None:
        raise Fail(f"{full} has no semantic-release config "
                   f"({', '.join(CONFIG_FILES[:3])}, …) — nothing to run.")
    steps.append(Step("ok", f"{full} carries {s['config']}"))

    # --- the grant ----------------------------------------------------------- #
    # Where no team can exist, a standing direct collaborator IS the grant, and
    # demanding a team would be demanding the impossible. This only ACCEPTS such
    # a grant; it never creates one — see missing_team_message() for why that
    # stays a deliberate act. The unit trap does not apply here: a collaborator's
    # access mode is not split per unit, so 'write' really does mean code:write.
    # Deliberately narrow to team is None — on an org repo the team remains the
    # one auditable place, and a stray direct grant must not quietly stand in
    # for it.
    if team is None and s["collaborator"] and s["permission"] in ("write", "admin"):
        steps.append(Step("ok", f"{AGENT} is a direct collaborator on {full} "
                                f"with '{s['permission']}' — {s['owner']} is a "
                                f"personal account, so this is the grant"))
    elif team is None:
        raise Fail(missing_team_message(s))
    else:
        if AGENT not in s["team_members"]:
            steps.append(Step("add", f"{AGENT} to team '{TEAM}'"))
        else:
            steps.append(Step("keep", f"{AGENT} is already in team '{TEAM}'"))

        if full in s["team_repos"]:
            steps.append(Step("keep", f"'{TEAM}' already covers {full}"))
        elif team.get("includes_all_repositories"):
            steps.append(Step("keep", f"'{TEAM}' covers all repos in {s['owner']}"))
        else:
            steps.append(Step("add", f"{full} to team '{TEAM}'"))

        # --- the code unit -------------------------------------------------- #
        # Gated: units are a property of the TEAM, so granting code here grants
        # it on every repo the team covers. That is a decision about other repos.
        if units.get(CODE_UNIT) in ("write", "admin"):
            steps.append(Step("keep", f"'{TEAM}' already has {CODE_UNIT}="
                                      f"{units[CODE_UNIT]}"))
        else:
            others = [r for r in s["team_repos"] if r != full]
            blast = (f" — this also grants it on {', '.join(others)}"
                     if others else "")
            steps.append(Step("grant", f"{CODE_UNIT}=write to team '{TEAM}'{blast}",
                              gated=True))

    # --- branch protection -------------------------------------------------- #
    p = s["protection"]
    if p is None:
        steps.append(Step("ok", f"branch '{s['branch']}' is unprotected"))
    elif not p.get("enable_push", True):
        raise Fail(
            f"branch protection on '{s['branch']}' forbids direct pushes "
            f"outright (enable_push=false).\n"
            f"   semantic-release requires a direct push. Relax the rule by "
            f"hand, or move the release to a PR/AGit flow — this script will "
            f"not disable a protection.")
    elif not p.get("enable_push_whitelist"):
        # See the module docstring, trap #2: the allowlist is OFF, so code-write
        # is already sufficient. Switching it on to "add" the agent would lock
        # out every other pusher. Never do that silently; never do it at all.
        steps.append(Step("ok", f"'{s['branch']}' allows pushes from anyone "
                                f"with code-write (no allowlist in force)"))
    elif AGENT in (p.get("push_whitelist_usernames") or []):
        steps.append(Step("keep", f"{AGENT} is on the '{s['branch']}' push "
                                  f"allowlist"))
    else:
        steps.append(Step("append", f"{AGENT} to the '{s['branch']}' push "
                                    f"allowlist (already enabled)", gated=True))

    if p and p.get("require_signed_commits"):
        steps.append(Step("note", "the branch requires signed commits — the "
                                  "Job signs with the agent's GPG key, so this "
                                  "is satisfied"))
    return steps


def apply(auth, s: dict, steps: list[Step]) -> None:
    full = f"{s['owner']}/{s['repo']}"
    team = s["team"]
    did = lambda v: any(st.verb == v for st in steps)

    if any(st.verb == "add" and st.text.startswith(AGENT) for st in steps):
        api("PUT", f"/teams/{team['id']}/members/{AGENT}", auth=auth, ok=(204,))
        log(f"  added {AGENT} to '{TEAM}'")

    if any(st.verb == "add" and st.text.startswith(full) for st in steps):
        api("PUT", f"/teams/{team['id']}/repos/{s['owner']}/{s['repo']}",
            auth=auth, ok=(204,))
        log(f"  added {full} to '{TEAM}'")

    if did("grant"):
        units = dict(team.get("units_map") or {})
        units[CODE_UNIT] = "write"
        # PATCH replaces the unit set wholesale, so send the merged map, not the
        # delta — and restate the scalars, which a partial body would reset.
        api("PATCH", f"/teams/{team['id']}", auth=auth, body={
            "name": team["name"],
            "description": team.get("description") or "",
            "permission": team.get("permission") or "write",
            "includes_all_repositories": bool(team.get("includes_all_repositories")),
            "can_create_org_repo": bool(team.get("can_create_org_repo")),
            "units_map": units,
        })
        log(f"  granted {CODE_UNIT}=write to '{TEAM}'")

    if did("append"):
        p = s["protection"]
        names = sorted(set(p.get("push_whitelist_usernames") or []) | {AGENT})
        api("PATCH", f"/repos/{s['owner']}/{s['repo']}/branch_protections/"
                     f"{urllib.parse.quote(s['branch'], safe='')}",
            auth=auth, body={"push_whitelist_usernames": names})
        log(f"  appended {AGENT} to the '{s['branch']}' push allowlist")


def release_command(owner: str, repo: str) -> str:
    """The `oc create` that actually runs the release, printed not executed.

    The manifest path is absolute and local, so the command is runnable as
    printed from any directory, and nothing but the cluster is contacted to
    produce it.
    """
    # The slug, not the full clone URL: it appears twice in the manifest — in
    # REPO_URL and in FORGEJO_REPOSITORY, which the release plugin reads to
    # decide WHICH repo gets the Release object. Retargeting one and not the
    # other would publish this repo's release onto forge-agents. Hence /g, and
    # hence the two values are spelled identically over there.
    edits = (f"  sed -e 's|name: semantic-release-dry-run$|name: "
             f"semantic-release-{repo}|' \\\n"
             f"      -e 's|{MANIFEST_HOME_SLUG}|{owner}/{repo}|g' \\\n"
             f"      -e 's|, \"--dry-run\"||'")

    body = (edits + f" \\\n      {find_manifest()}"
            + f" | \\\n      oc -n {NAMESPACE} create -f -")

    return (f"\nRelease it with:\n\n{body}\n\n"
            f"  Drop the third -e for a dry run. `create`, not `apply` — a "
            f"Job's pod template is immutable.")


def verify(auth, s: dict) -> str:
    """Read the effective permission back. The plan reasons about teams and
    units; this asks the question the push will actually ask."""
    perm = api("GET", f"/repos/{s['owner']}/{s['repo']}/collaborators/"
                      f"{AGENT}/permission", auth=auth, ok=(200, 404)) or {}
    return perm.get("permission", "none")


# --------------------------------------------------------------------------- #
# MANAGED BLOCKS. Two consumers — the repo's justfile and its AGENTS.md — and
# exactly one implementation of the part that is genuinely the same: a fence
# carrying a version and a hash, and the six answers that pair licenses
# (create / append / no-op / update / you-edited-it / broken fence).
#
# What is NOT shared, because it genuinely differs: the comment syntax, the
# candidate filenames, and what "something like this is already here" means.
# Those three are the constructor arguments of Managed below and nothing else
# is. This half needs no cluster and no forge — only a directory — which is why
# `--skip grant` is testable anywhere, and a release runner nobody can test is
# a liability.
# --------------------------------------------------------------------------- #


def body_hash(body: str) -> str:
    """sha256 of the body AS WRITTEN, first 12 hex. Twelve because this is a
    tamper *hint* for a human reading a diff, not a security boundary."""
    return hashlib.sha256(body.encode()).hexdigest()[:12]


def version_key(v: str) -> tuple[int, int, int]:
    """X.Y.Z, for comparison only.

    Prerelease and build metadata are DROPPED. Semver orders 2.0.0-rc.1 below
    2.0.0, but the only question asked here is "is the installed block from a
    LATER release than the one I am", and a candidate ships the same block as
    its release. Anything unparseable sorts lowest rather than raising: a
    marker is a string in a file a human can edit, and 0.0.0-dev — the version
    a forge-agents checkout carries, being no release at all — must sort below
    every real one, which (0,0,0) achieves for free.
    """
    m = re.match(r"(\d+)\.(\d+)\.(\d+)", v or "")
    return (int(m[1]), int(m[2]), int(m[3])) if m else (0, 0, 0)


class Clash:
    """One reason not to append: a label, a line number, and the text that
    triggered it. Rendered by report(); produced by each consumer's detector."""

    def __init__(self, line: int, what: str, evidence: str = ""):
        self.line, self.what, self.evidence = line, what, evidence

    def render(self) -> str:
        ev = f"  — {self.evidence}" if self.evidence else ""
        return f"{self.what}  (line {self.line}){ev}"


class Managed:
    """A block this skill owns inside a file it does not own.

    `comment` is the (prefix, suffix) pair that makes a line invisible to
    whatever parses the file: ("# ", "") for a justfile, ("<!-- ", " -->") for
    Markdown, where '#' would be a heading rather than a comment.

    `clash` answers "is there already something like this here, hand-written?"
    and is per-consumer because the question is: a justfile clashes on NAMES
    (fatal — just refuses to parse a doubly-defined recipe), Markdown clashes
    on CONTENT (redundant, not fatal). Hence `clash_is_fatal`.
    """

    def __init__(self, *, part: str, what: str, template: str, candidates,
                 comment: tuple[str, str], clash, clash_is_fatal: bool,
                 subs: dict[str, str] | None = None, gap: str = "\n"):
        self.part, self.what, self.template = part, what, template
        self.candidates, self.clash, self.gap = candidates, clash, gap
        self.clash_is_fatal, self.subs = clash_is_fatal, subs or {}
        pre, suf = comment
        self.open_t = (f"{pre}>>> enable-semantic-release v{{version}} "
                       f"sha256:{{hash}} (managed block — do not edit) >>>{suf}")
        self.close_t = (f"{pre}<<< enable-semantic-release v{{version}} "
                        f"sha256:{{hash}} <<<{suf}")
        # Built from the same strings the writer uses, so the reader can never
        # drift from the writer — the one bug this class exists to not have.
        self.re_open = re.compile(
            re.escape(self.open_t).replace(r"\{version\}", r"(?P<version>\S+)")
                                  .replace(r"\{hash\}", r"(?P<hash>[0-9a-f]+)")
            + "$")
        self.re_close = re.compile(
            re.escape(self.close_t).replace(r"\{version\}", r"(?P<version>\S+)")
                                   .replace(r"\{hash\}", r"(?P<hash>[0-9a-f]+)")
            + "$")

    # --- the block itself --------------------------------------------------- #

    def body(self) -> str:
        """The template with the few things that are not derivable in-repo
        filled in. Everything else each block needs it works out at run time,
        so the installed text is identical in every repo and its hash is
        stable — a hash that varied per repo would answer no question."""
        text = find_bundled(self.template).read_text()
        for placeholder, value in self.subs.items():
            text = text.replace(placeholder, value)
        if "@@" in text:
            raise Fail(f"{self.template} still has an unsubstituted @@…@@ "
                       f"placeholder — the template and this script disagree.")
        return text.rstrip("\n")

    def render(self, body: str | None = None,
               version: str | None = None) -> str:
        # SKILL_VERSION is read HERE, not captured as a default argument: a
        # default binds at import time, which would freeze the version for
        # anything that rewrites the constant afterwards — the release stamp,
        # and every test that needs to reason about older and newer.
        version = SKILL_VERSION if version is None else version
        body = self.body() if body is None else body
        h = body_hash(body)
        return (self.open_t.format(version=version, hash=h) + "\n"
                + body + "\n"
                + self.close_t.format(version=version, hash=h) + "\n")

    # --- reading ------------------------------------------------------------ #

    def locate(self, repo: Path) -> tuple[Path, bool]:
        """(path, exists). Every candidate spelling is looked for; the first
        name is what gets created if none is there. resolve() de-duplicates the
        case where one candidate is a symlink to another — forge-agents' own
        CLAUDE.md -> AGENTS.md — so that is one file, not two."""
        for name in self.candidates:
            p = repo / name
            if p.is_file():
                return p, True
        return repo / self.candidates[0], False

    def others_present(self, repo: Path, chosen: Path) -> list[Path]:
        """Candidates that exist, are not the chosen file, and are not the same
        file by another name. These are the ones we will NOT write to, and the
        caller must say so out loud rather than silently updating one of two
        files an agent might read."""
        out = []
        for name in self.candidates:
            p = repo / name
            if p.is_file() and not p.samefile(chosen):
                out.append(p)
        return out

    def find(self, lines: list[str]) -> dict | None:
        """This skill's block in those lines, or None. Raises on a broken
        fence — that is a hand edit too, and guessing where the block ends is
        how a tool eats the section that came after it."""
        opens = [(i, m) for i, l in enumerate(lines) if (m := self.re_open.match(l))]
        closes = [(i, m) for i, l in enumerate(lines) if (m := self.re_close.match(l))]
        if not opens and not closes:
            return None
        if len(opens) != 1 or len(closes) != 1:
            raise Fail(f"{self.what} has {len(opens)} opening and {len(closes)} "
                       f"closing enable-semantic-release markers. Exactly one of "
                       f"each is expected — repair it by hand; this script will "
                       f"not guess which text is managed.")
        (i, om), (j, cm) = opens[0], closes[0]
        if j <= i:
            raise Fail(f"the enable-semantic-release closing marker in "
                       f"{self.what} comes before the opening one. Repair by hand.")
        if (om["version"], om["hash"]) != (cm["version"], cm["hash"]):
            raise Fail(f"the enable-semantic-release markers in {self.what} "
                       f"disagree: opens as v{om['version']}/{om['hash']}, closes "
                       f"as v{cm['version']}/{cm['hash']}. One of them was "
                       f"edited; repair by hand or delete the block and re-run.")
        return {"start": i, "end": j, "version": om["version"],
                "hash": om["hash"], "body": "\n".join(lines[i + 1:j])}

    # --- deciding ----------------------------------------------------------- #

    def plan(self, repo: Path) -> dict:
        """What to do. Reads; never writes."""
        body = self.body()
        path, exists = self.locate(repo)
        plan = {"m": self, "path": path, "body": body, "text": "", "also": []}

        if not exists:
            return plan | {"action": "create"}

        plan["also"] = self.others_present(repo, path)
        text = path.read_text()
        plan["text"] = text
        lines = text.splitlines()
        block = self.find(lines)

        if block is None:
            clashes = self.clash(body, lines)
            if clashes:
                return plan | {"action": "clash", "clashes": clashes}
            return plan | {"action": "append"}

        if body_hash(block["body"]) != block["hash"]:
            return plan | {"action": "edited", "block": block}
        if block["body"] == body and block["version"] == SKILL_VERSION:
            return plan | {"action": "noop", "block": block}
        # Newer than me. Replacing it would be a DOWNGRADE, and the commonest
        # way to hit one is running a forge-agents checkout (0.0.0-dev, which
        # is no release at all) over a repo that installed a real tarball. Not
        # silently: an unedited block is normally replaced without a flag, and
        # this is the one direction where that would lose work rather than
        # deliver it.
        if version_key(block["version"]) > version_key(SKILL_VERSION):
            return plan | {"action": "downgrade", "block": block}
        return plan | {"action": "update", "block": block}

    # --- writing ------------------------------------------------------------ #

    def write(self, plan: dict) -> None:
        path, action = plan["path"], plan["action"]
        block = self.render(plan["body"])

        if action == "create":
            path.write_text(block)
            log(f"  wrote {path}")
            return

        text = plan["text"]
        if action in ("append", "clash"):
            sep = "" if text.endswith("\n" + self.gap) else (
                self.gap if text.endswith("\n") else "\n" + self.gap)
            path.write_text(text + sep + block)
            log(f"  appended the managed block to {path}")
            return

        # update / forced replace: splice, so anything before and after
        # survives. This is the whole reason the fence records where it ends.
        lines = text.splitlines(keepends=True)
        b = plan["block"]
        path.write_text("".join(lines[:b["start"]]) + block
                        + "".join(lines[b["end"] + 1:]))
        log(f"  replaced the v{b['version']} block in {path} with v{SKILL_VERSION}")


# --------------------------------------------------------------------------- #
# Consumer 1: the repo's justfile.
# --------------------------------------------------------------------------- #

# Names just resolves in a single namespace each. A clash is FATAL — just
# refuses to parse a file that defines `release` twice — so it is detected
# before writing rather than discovered by the next `just --list`.
RE_RECIPE = re.compile(r"^(?:@)?([A-Za-z_][A-Za-z0-9_-]*)(?:\s+[^:=]*?)?\s*:(?!=)")
RE_ASSIGN = re.compile(r"^([A-Za-z_][A-Za-z0-9_-]*)\s*:=")
RE_ALIAS = re.compile(r"^alias\s+([A-Za-z_][A-Za-z0-9_-]*)\s*:=")


def declared_names(lines: list[str]) -> dict[str, int]:
    """Recipe, alias and variable names declared in those lines.

    Line-based on purpose: a real just parser is not needed to answer "would
    appending this block define something twice", and being slightly over-eager
    here costs a warning, while being under-eager costs a justfile that no
    longer parses. It does miss names pulled in by `import` or `mod`.
    """
    names: dict[str, int] = {}
    for n, raw in enumerate(lines, start=1):
        line = raw.rstrip()
        if not line or line[0] in "# \t[":
            continue
        if line.startswith("export "):
            line = line[len("export "):]
        if m := RE_ALIAS.match(line):
            names.setdefault(m.group(1), n)
            continue
        if m := RE_ASSIGN.match(line) or RE_RECIPE.match(line):
            names.setdefault(m.group(1), n)
    return names


def justfile_clash(body: str, lines: list[str]) -> list[Clash]:
    mine = set(declared_names(body.splitlines()))
    theirs = declared_names(lines)
    return [Clash(theirs[n], n) for n in sorted(mine & set(theirs))]


# --------------------------------------------------------------------------- #
# Consumer 2: the repo's AGENTS.md.
# --------------------------------------------------------------------------- #

# THE HEURISTIC, and it is a heuristic. Markdown has no namespace, so there is
# nothing to collide with mechanically; the question is the human one, "does
# this file already tell an agent how to release?" Two signals, both requiring
# the word to appear OUTSIDE a managed block:
#
#   a) a heading whose text is about releasing
#   b) any line naming the machinery: `just release`, `just preflight`,
#      semantic-release, release-dry
#
# TRADEOFF, stated plainly. Signal (b) is deliberately over-eager: forge-agents'
# own AGENTS.md mentions semantic-release in a heading about its bead database
# and will trip it, which is a FALSE POSITIVE and the intended failure
# direction — a spurious warning costs one `--force-block`, a spurious append
# costs a file that contradicts itself about how the repo releases, silently,
# in the one document agents are told to trust. The known FALSE NEGATIVE is a
# file that explains releasing without ever naming the tools ("push to main and
# the bot does the rest"); nothing short of reading it catches that, and an
# agent running this skill is expected to have read it.
#
# Unlike the justfile, this clash is NOT fatal: appending a second, redundant
# section produces valid Markdown. So --force-block overrides it here and
# cannot there.
RE_MD_HEADING = re.compile(r"^\s{0,3}#{1,6}\s+(.*\S)\s*$")
RE_MD_RELEASE_HEADING = re.compile(r"releas", re.I)
MD_TELLS = ("just release", "just preflight", "just release-dry",
            "semantic-release", "release-dry")


def agents_md_clash(body: str, lines: list[str]) -> list[Clash]:
    out: list[Clash] = []
    for n, raw in enumerate(lines, start=1):
        line = raw.rstrip()
        # Fenced code is NOT skipped: a sample that says `just release` is
        # still a claim about how this repo releases. See the tradeoff above.
        if m := RE_MD_HEADING.match(line):
            if RE_MD_RELEASE_HEADING.search(m.group(1)):
                out.append(Clash(n, "a heading about releasing", m.group(1)))
                continue
        low = line.lower()
        for tell in MD_TELLS:
            if tell in low:
                out.append(Clash(n, "names the release machinery",
                                 line.strip()[:70]))
                break
    return out


# --------------------------------------------------------------------------- #

JUSTFILE = Managed(
    part="justfile",
    what="the justfile",
    template="justfile-block.just",
    candidates=("justfile", "Justfile", ".justfile"),
    comment=("# ", ""),
    clash=justfile_clash,
    clash_is_fatal=True,
    subs={"@@NAMESPACE@@": NAMESPACE, "@@AGENT@@": AGENT,
          "@@MANIFEST_HOME_SLUG@@": MANIFEST_HOME_SLUG},
)

# AGENTS.md first: it is the cross-tool spelling, and where both exist this is
# the one that gets written. CLAUDE.md is a candidate rather than a second
# target because writing two files that an agent reads as one instruction set
# is how they end up disagreeing.
AGENTS_MD = Managed(
    part="agents-md",
    what="AGENTS.md",
    template="agents-md-block.md",
    candidates=("AGENTS.md", "CLAUDE.md"),
    comment=("<!-- ", " -->"),
    clash=agents_md_clash,
    clash_is_fatal=False,
    gap="\n",
)

PARTS = ("grant", "justfile", "agents-md", "ci")
BLOCKS = {"justfile": JUSTFILE, "agents-md": AGENTS_MD}


# --------------------------------------------------------------------------- #
# Consumer 3: whole files — the four `ci` part targets under .tekton/ and
# OWNERS. These are NOT spliced into a file that might already have other
# content: they ARE the file, installed by name. Managed's fence exists to let
# a block coexist with hand-written content around it; that question does not
# apply here, so WholeFile below is a separate, smaller class rather than a
# third mode bolted onto Managed. What IS reused, deliberately, is the
# version/hash vocabulary (body_hash, version_key) — "is this mine, is it
# current, is it edited, is it newer" is the identical six-way question, only
# the marker is a single header LINE rather than an open/close fence, and a
# stale copy is replaced WHOLE rather than spliced in place.
# --------------------------------------------------------------------------- #


class WholeFile:
    """A file this skill installs by name rather than by splicing into
    existing content. `body_fn` is a zero-argument callable — usually a
    closure over already-resolved per-repo facts (language, repo name) —
    because unlike a Managed block's body, a whole file's content can differ
    by repo and the hash still has to describe THAT repo's installed copy.
    """

    def __init__(self, *, part: str, what: str, dest: str, body_fn,
                 header_prefix: str = "# "):
        self.part, self.what, self.dest = part, what, dest
        self.body_fn, self.header_prefix = body_fn, header_prefix
        self.re_header = re.compile(
            re.escape(header_prefix)
            + r">>> enable-semantic-release v(?P<version>\S+) "
              r"sha256:(?P<hash>[0-9a-f]+) \(managed file — do not edit by "
              r"hand\) >>>$")

    def header_line(self, h: str, version: str | None = None) -> str:
        version = SKILL_VERSION if version is None else version
        return (f"{self.header_prefix}>>> enable-semantic-release v{version} "
                f"sha256:{h} (managed file — do not edit by hand) >>>\n")

    def render(self, body: str, version: str | None = None) -> str:
        return self.header_line(body_hash(body), version) + body

    def find(self, text: str) -> dict | None:
        """This skill's header at the top of `text`, or None. Unlike Managed
        there is no fence integrity to police — a single header line either
        matches or it does not, and everything after it is the body,
        whatever it is."""
        if not text:
            return None
        first, _, rest = text.partition("\n")
        m = self.re_header.match(first)
        if not m:
            return None
        return {"version": m["version"], "hash": m["hash"], "body": rest}

    def plan(self, repo: Path) -> dict:
        path = repo / self.dest
        body = self.body_fn()
        plan = {"m": self, "path": path, "body": body}
        if not path.is_file():
            return plan | {"action": "create"}

        found = self.find(path.read_text())
        if found is None:
            return plan | {"action": "clash"}
        if body_hash(found["body"]) != found["hash"]:
            return plan | {"action": "edited", "block": found}
        if found["body"] == body and found["version"] == SKILL_VERSION:
            return plan | {"action": "noop", "block": found}
        if version_key(found["version"]) > version_key(SKILL_VERSION):
            return plan | {"action": "downgrade", "block": found}
        return plan | {"action": "update", "block": found}

    def write(self, plan: dict) -> None:
        path = plan["path"]
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(self.render(plan["body"]))


def report_wholefile(plan: dict, repo: Path, *, force: bool) -> None:
    w: WholeFile = plan["m"]
    path, action = plan["path"], plan["action"]
    rel = path.relative_to(repo) if path.is_relative_to(repo) else path
    log(f"\n{w.what} in {repo} (v{SKILL_VERSION}):\n")

    if action == "create":
        log(f"  create   {rel}")
    elif action == "noop":
        log(f"  keep     {rel} already carries v{SKILL_VERSION}, unedited")
    elif action == "update":
        was = plan["block"]["version"]
        move = (f"v{was} → v{SKILL_VERSION}" if was != SKILL_VERSION else
                f"the v{was} copy no longer matches the template")
        log(f"  update   {rel}: {move}, replaced whole")
    elif action == "clash":
        verb = "⚠️ force " if force else "⚠️ skip  "
        log(f"  {verb} {rel} exists and carries no enable-semantic-release "
            f"header — hand-written or foreign.")
        if force:
            log("           --force-block given: overwriting it.")
        else:
            log("           Nothing was written. --force-block to overwrite "
                "it whole, or move it aside and re-run.")
    elif action == "downgrade":
        b = plan["block"]
        verb = "⚠️ force " if force else "⚠️ skip  "
        log(f"  {verb} {rel} carries v{b['version']}, which is NEWER than "
            f"this copy of the skill (v{SKILL_VERSION}).")
        log("           " + (
            f"--force-block given: downgrading it to v{SKILL_VERSION}."
            if force else
            f"Writing would downgrade it. Install the newer skill and run "
            f"that, or --force-block if you really mean to go back."))
    elif action == "edited":
        b = plan["block"]
        verb = "⚠️ force " if force else "⚠️ skip  "
        log(f"  {verb} the v{b['version']} copy of {rel} no longer hashes to "
            f"{b['hash']} — it was hand-edited after this skill wrote it.")
        log("           " + (
            f"--force-block given: replacing it with v{SKILL_VERSION}; those "
            f"edits are gone."
            if force else
            f"Re-run with --force-block to replace it with v{SKILL_VERSION} "
            f"and lose those edits."))
    log("")


def install_wholefile(w: WholeFile, repo: Path, *, dry_run: bool,
                      force: bool) -> int:
    """0 done or nothing to do · 1 a clash, liftable with --force-block ·
    2 a downgrade or a hand-edit, liftable with --force-block.

    Unlike a justfile clash, a whole-file clash is NEVER structurally fatal:
    there is no append and nothing else in the file to break by overwriting
    it, so --force-block lifts all three "no" cases here, not just two.
    """
    plan = w.plan(repo)
    action = plan["action"]
    needs_force = action in ("clash", "downgrade", "edited")
    report_wholefile(plan, repo, force=needs_force and force)

    if needs_force and not force:
        return 1 if action == "clash" else 2
    if action == "noop":
        return 0
    if dry_run:
        log(f"  dry run — {plan['path'].name} was not touched.\n")
        return 0
    w.write(plan)
    log("")
    return 0


# --- language detection, and the repo facts the ci templates need ---------- #
#
# EVIDENCE-BASED ONLY. Guessing which CI task to install is worse than
# refusing: a python-ci.yaml installed into a bun repo is a pipeline that
# always fails, discovered only after a PR is blocked on it.

BUN_EVIDENCE = ("package.json", "bun.lock", "bun.lockb")

# Packaging evidence is sufficient on its own: a repo that declares itself a
# python package (or lists its deps) is python whether or not it has tests
# yet — python-ci.yaml's `unittest discover -s tests` just finds nothing to
# run in that case, it does not break. requirements*.txt covers
# requirements.txt / requirements-dev.txt / etc., the common unpackaged shape.
PYTHON_PACKAGING_EVIDENCE = ("pyproject.toml", "setup.py", "setup.cfg")

# No packaging file at all — e.g. forge-agents itself: stdlib scripts at the
# repo root with a tests/ dir and no pyproject.toml/setup.py, nothing to
# `pip install`. That shape is exactly what python-ci.yaml targets (no
# install step), so tests/ + at least one root-level *.py counts as evidence
# too. tests/ alone would not: an empty or unrelated tests/ dir proves
# nothing about the language.
PYTHON_SCRIPT_SHAPE_HINT = "a tests/ directory plus a *.py file at the repo root"


def _has_python_evidence(repo: Path) -> bool:
    if any((repo / name).is_file() for name in PYTHON_PACKAGING_EVIDENCE):
        return True
    if list(repo.glob("requirements*.txt")):
        return True
    if (repo / "tests").is_dir() and list(repo.glob("*.py")):
        return True
    return False


def detect_ci_language(repo: Path) -> str:
    has_bun = any((repo / name).is_file() for name in BUN_EVIDENCE)
    has_python = _has_python_evidence(repo)
    if has_bun and has_python:
        raise Fail(
            f"{repo} carries evidence of BOTH bun ({', '.join(BUN_EVIDENCE)}) "
            f"and python ({', '.join(PYTHON_PACKAGING_EVIDENCE)}, "
            f"requirements*.txt, or {PYTHON_SCRIPT_SHAPE_HINT}) — refusing to "
            f"guess which CI task to install. Install .tekton/tasks/"
            f"python-ci.yaml or bun-ci.yaml by hand instead.")
    if has_bun:
        return "bun"
    if has_python:
        return "python"
    raise Fail(
        f"{repo} has neither bun (one of {', '.join(BUN_EVIDENCE)}) nor "
        f"python ({', '.join(PYTHON_PACKAGING_EVIDENCE)}, requirements*.txt, "
        f"or {PYTHON_SCRIPT_SHAPE_HINT}) evidence — refusing to guess which "
        f"CI task to install. Add the missing manifest, or install "
        f".tekton/tasks/<lang>-ci.yaml by hand and skip this part with "
        f"--skip ci.")


def detect_repo_name(repo: Path) -> str:
    """The slug that goes into the PipelineRun's name and the Task refs.
    Prefers the origin remote's slug — the same source the justfile block
    reads — and falls back to the directory's own name, which is all that is
    left for a repo with no remote configured yet (a fresh `git init`, or a
    test's scratch dir)."""
    out = run(["git", "-C", str(repo), "remote", "get-url", "origin"],
              check=False)
    if out.returncode == 0:
        url = out.stdout.decode().strip()
        m = re.search(r"/([^/]+?)(?:\.git)?/?$", url)
        if m:
            return m.group(1)
    return repo.name


def _ci_body(name: str):
    return lambda: find_bundled(name).read_text()


def _on_pr_body(ctx: dict):
    def render() -> str:
        text = find_bundled("ci-on-pull-request.yaml").read_text()
        subs = {
            "@@REPO_NAME@@": ctx["repo_name"],
            "@@CI_TASK_FILE@@": ctx["task_file"],
            "@@CI_TASK_NAME@@": ctx["task_name"],
            "@@CI_STORAGE@@": ctx["storage"],
        }
        for placeholder, value in subs.items():
            text = text.replace(placeholder, value)
        if "@@" in text:
            raise Fail("ci-on-pull-request.yaml still has an unsubstituted "
                       "@@…@@ placeholder — the template and this script "
                       "disagree.")
        return text
    return render


def ci_targets(ctx: dict) -> list[WholeFile]:
    return [
        WholeFile(part="ci", what=".tekton/on-pull-request.yaml",
                  dest=".tekton/on-pull-request.yaml",
                  body_fn=_on_pr_body(ctx)),
        WholeFile(part="ci", what=f".tekton/tasks/{ctx['language']}-ci.yaml",
                  dest=ctx["task_file"],
                  body_fn=_ci_body(f"ci-{ctx['language']}-ci.yaml")),
        WholeFile(part="ci", what=".tekton/tasks/commit-title-check.yaml",
                  dest=".tekton/tasks/commit-title-check.yaml",
                  body_fn=_ci_body("ci-commit-title-check.yaml")),
        WholeFile(part="ci", what="OWNERS", dest="OWNERS",
                  body_fn=_ci_body("ci-OWNERS")),
    ]


def install_ci(repo: Path, *, dry_run: bool, force: bool) -> int:
    lang = detect_ci_language(repo)
    ctx = {
        "language": lang,
        "task_file": f".tekton/tasks/{lang}-ci.yaml",
        "task_name": f"{lang}-ci",
        "storage": "1Gi" if lang == "python" else "2Gi",
        "repo_name": detect_repo_name(repo),
    }
    rc = 0
    for w in ci_targets(ctx):
        rc = install_wholefile(w, repo, dry_run=dry_run, force=force) or rc
    return rc


def report_block(plan: dict, repo: Path, *, force: bool) -> None:
    m: Managed = plan["m"]
    path, action = plan["path"], plan["action"]
    rel = path.relative_to(repo) if path.is_relative_to(repo) else path
    log(f"\n{m.what} in {repo} (v{SKILL_VERSION}):\n")

    if action == "create":
        log(f"  create   {rel} with the managed block")
    elif action == "append":
        log(f"  append   the managed block to {rel}")
    elif action == "noop":
        log(f"  keep     {rel} already carries v{SKILL_VERSION}, unedited")
    elif action == "update":
        was = plan["block"]["version"]
        # Same version, different body only happens on 0.0.0-dev, where the
        # version is by definition not a version. Say so rather than printing
        # "v0.0.0-dev → v0.0.0-dev" and looking broken.
        move = (f"v{was} → v{SKILL_VERSION}" if was != SKILL_VERSION else
                f"the v{was} block no longer matches the template")
        log(f"  update   {rel}: {move}, in place, surrounding content untouched")
    elif action == "clash":
        verb = "⚠️ force " if force else "⚠️ skip  "
        log(f"  {verb} {rel} already looks like it covers this:")
        for c in plan["clashes"][:8]:
            log(f"           {c.render()}")
        if len(plan["clashes"]) > 8:
            log(f"           … and {len(plan['clashes']) - 8} more")
        if m.clash_is_fatal:
            log(f"           Nothing was written, and --force-block will not "
                f"write it either: appending would define those names twice "
                f"and just would refuse to parse the file at all. Rename "
                f"yours, or delete a hand-rolled copy of this block, then "
                f"re-run.")
        elif force:
            log(f"           --force-block given: appending anyway. Read the "
                f"result — you now have two accounts of how this repo "
                f"releases, and only one of them is maintained.")
        else:
            log(f"           Nothing was written. If those lines are about "
                f"something else, re-run with --force-block to append; if they "
                f"are a hand-written version of this block, delete them first.")
    elif action == "downgrade":
        b = plan["block"]
        verb = "⚠️ force " if force else "⚠️ skip  "
        log(f"  {verb} {rel} carries v{b['version']}, which is NEWER than this "
            f"copy of the skill (v{SKILL_VERSION}).")
        log("           " + (
            f"--force-block given: downgrading it to v{SKILL_VERSION}."
            if force else
            f"Writing would downgrade it. Install the newer skill and run "
            f"that, or --force-block if you really mean to go back."))
    elif action == "edited":
        b = plan["block"]
        verb = "⚠️ force " if force else "⚠️ skip  "
        log(f"  {verb} the v{b['version']} block in {rel} no longer hashes to "
            f"{b['hash']} — it was hand-edited after this skill wrote it.")
        log("           " + (
            f"--force-block given: replacing it with v{SKILL_VERSION}; those "
            f"edits are gone."
            if force else
            f"Re-run with --force-block to replace it with v{SKILL_VERSION} "
            f"and lose those edits."))

    for other in plan["also"]:
        log(f"\n  ⚠️ NOTE   {other.name} also exists here and was NOT touched. "
            f"It is a separate file from {rel}, so an agent reading it will "
            f"not see this block. Make one a symlink to the other, or copy "
            f"the block across by hand.")
    log("")


def install_block(m: Managed, repo: Path, *, dry_run: bool, force: bool) -> int:
    """0 done or nothing to do · 1 a clash that no flag can fix · 2 something
    that needs --force-block."""
    plan = m.plan(repo)
    action = plan["action"]
    # Two kinds of "no": one no flag can lift (appending would break the file),
    # and one --force-block lifts (you would only be losing your own words).
    hard = action == "clash" and m.clash_is_fatal
    soft = action in ("edited", "downgrade") or (
        action == "clash" and not m.clash_is_fatal)
    report_block(plan, repo, force=soft and force)

    if hard:
        return 1
    if soft and not force:
        return 2
    if action == "noop":
        return 0
    if dry_run:
        log(f"  dry run — {plan['path'].name} was not touched.\n")
        return 0
    m.write(plan)
    log("")
    return 0


def resolve_repo(repo_dir: str | None) -> Path:
    """The one place this script reads the cwd, and only to answer 'which repo
    am I in'. Everything it INSTALLS is resolved against __file__."""
    if repo_dir:
        repo = Path(repo_dir).resolve()
        if not repo.is_dir():
            raise Fail(f"--repo-dir {repo} is not a directory")
        return repo
    top = run(["git", "rev-parse", "--show-toplevel"], check=False)
    if top.returncode != 0:
        raise Fail("not inside a git repository, so there is no repo to "
                   "install into. Run this from the checkout, pass --repo-dir, "
                   "or --only grant.")
    return Path(top.stdout.decode().strip())


def parts_wanted(p, only: list[str], skip: list[str]) -> list[str]:
    """--only/--skip instead of a --no-X and an --X-only per part.

    Three parts times two flags each is six flags that all mean 'run some of
    it', and adding a fourth part would mean eight. One pair that takes part
    names says the same thing, composes (`--only agents-md --force-block`
    scopes a force to one part), and is the shape that does not grow.
    """
    def check(names, flag):
        bad = [n for n in names if n not in PARTS]
        if bad:
            p.error(f"{flag}: no such part {', '.join(bad)} "
                    f"(known: {', '.join(PARTS)})")
        return names
    chosen = check(only, "--only") or list(PARTS)
    chosen = [n for n in chosen if n not in check(skip, "--skip")]
    if not chosen:
        p.error("--only and --skip between them leave nothing to do")
    return chosen


def main() -> int:
    p = argparse.ArgumentParser(
        description="Grant b4mad-release-agent what semantic-release needs on a repo.")
    p.add_argument("repo_url", help=f"ssh://git@{SSH_HOST}:{SSH_PORT}/<owner>/<repo>.git")
    p.add_argument("--dry-run", action="store_true",
                   help="survey and print the plan; change nothing")
    p.add_argument("--force", action="store_true",
                   help="allow steps that widen access beyond this one repo")
    # Deliberately NOT the same flag as --force. That one consents to widening
    # somebody else's access on the forge; this one consents to losing your own
    # words in a local file. Folding them together would mean a user who wanted
    # the second silently authorised the first. Scope it with --only.
    p.add_argument("--force-block", action="store_true",
                   help="replace a hand-edited managed block, or append past a "
                        "content clash where that is not fatal")
    p.add_argument("--only", metavar="PARTS", default="",
                   help=f"run only these, comma-separated: {', '.join(PARTS)}")
    p.add_argument("--skip", metavar="PARTS", default="",
                   help="run everything except these")
    p.add_argument("--repo-dir", metavar="DIR",
                   help="repo whose files to install into "
                        "(default: the git top level of the cwd)")
    p.add_argument("--version", action="version",
                   version=f"enable-semantic-release {SKILL_VERSION}")
    args = p.parse_args()

    if args.dry_run and args.force:
        p.error("--dry-run and --force contradict each other; --dry-run already "
                "changes nothing, so there is nothing for --force to permit")
    if args.dry_run and args.force_block:
        p.error("--dry-run and --force-block contradict each other")
    split = lambda v: [x.strip() for x in v.split(",") if x.strip()]
    parts = parts_wanted(p, split(args.only), split(args.skip))

    try:
        owner, repo = parse_repo_url(args.repo_url)

        # The local parts first, because they need neither cluster nor forge:
        # a cluster that is down should not stop the repo getting its targets.
        # Their exit codes are kept and returned at the end, so a warning here
        # is not lost behind a successful grant.
        rc = 0
        blocks = [name for name in PARTS if name in parts and name in BLOCKS]
        if blocks or "ci" in parts:
            repo_dir = resolve_repo(args.repo_dir)
            for name in blocks:
                rc = install_block(BLOCKS[name], repo_dir,
                                   dry_run=args.dry_run,
                                   force=args.force_block) or rc
            if "ci" in parts:
                rc = install_ci(repo_dir, dry_run=args.dry_run,
                                force=args.force_block) or rc
        if "grant" not in parts:
            return rc

        # Checked here rather than left to the first call: `oc` is the one
        # dependency this script cannot substitute for, and run() would report
        # its absence as a FileNotFoundError traceback halfway through a survey.
        if shutil.which("oc") is None:
            raise Fail("`oc` is not on PATH. This script reads the admin and "
                       "agent Secrets out of the live cluster, so it must run "
                       "somewhere logged in to the one holding namespace "
                       f"{NAMESPACE}.")
        # Up front, though it is only used in the last line printed: a grant
        # applied and then no way to say how to use it is the worst ordering.
        find_manifest()
        auth = admin_credentials()
        s = survey(auth, owner, repo)
        steps = build_plan(s)

        log(f"\nplan for {owner}/{repo} (branch '{s['branch']}') — "
            f"{'would do' if args.dry_run else 'will do'}:\n")
        for st in steps:
            log(st.render())
        log("")

        gated = [st for st in steps if st.gated]
        todo = [st for st in steps if st.verb in ("add", "grant", "append")]

        if not todo:
            log(f"✔ nothing to do — {AGENT} already has "
                f"'{s['permission']}' on {owner}/{repo}.")
        elif args.dry_run:
            log("dry run — nothing changed. Re-run without --dry-run to apply.")
        elif gated and not args.force:
            log(f"⚠️ {len(gated)} step(s) above widen access beyond {owner}/{repo}.\n"
                f"   Re-run with --force if that is what you meant.")
            return 2
        else:
            apply(auth, s, steps)

        if not args.dry_run and todo and (args.force or not gated):
            got = verify(auth, s)
            if got not in ("write", "admin"):
                raise Fail(f"applied, but {AGENT} still has '{got}' on "
                           f"{owner}/{repo} — the push would still be rejected")
            log(f"\n✔ {AGENT} now has '{got}' on {owner}/{repo}.")

        if not args.dry_run:
            log(release_command(owner, repo))
            if "justfile" in parts:
                log("  Or, now that the targets are installed: `just release` "
                    "— same Job, with the preflight in front of it.")
        return rc
    except Fail as e:
        log(f"✗ {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
