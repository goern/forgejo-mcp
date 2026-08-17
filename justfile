# forgejo-mcp task recipes.
#
# The canonical build tooling is `make` (see Makefile). This justfile holds
# auxiliary recipes that the Make targets don't cover. Run `just --list`.

# Validate anchored Showboat demos against their specs (see .claude/skills/showboat).
check-demos:
    ./scripts/ci/check-spec-demo-anchors.sh

# >>> enable-semantic-release v1.4.2 sha256:98b96f591752 (managed block — do not edit) >>>
# Release tasks, installed by the enable-semantic-release skill.
#
# `just release` cuts a real release; everything else here exists so that the
# ways it is known to fail are caught before the Job is created rather than 90
# seconds into a run that already pushed something.
#
# THE THING TO INTERNALISE: the Job clones the repo from the FORGE. It never
# sees this working tree. So a dirty tree is harmless and an unpushed commit is
# not — it releases what is on the release remote's main, which is why
# `preflight` checks the remote ref and only warns about local dirt.
#
# Everything here is prefixed `sr_` or named for what it does; there is no
# `set shell` and no `default` recipe, because this block is appended to a
# justfile that may already have both and just rejects a duplicate setting.

sr_namespace := "b4mad-forgejo"
sr_agent     := "b4mad-release-agent"

# WHICH REMOTE IS THE RELEASE TARGET. Not hardcoded to `origin`: a checkout
# whose `origin` is a personal fork and whose canonical repo is a second remote
# is a normal arrangement, and hardcoding sent the Job at the fork — it would
# clone stale code and hang the Release object off the wrong repo.
#
# `upstream` WINS OVER THE TRACKING REMOTE, which is the one thing experience
# corrected. The tracking remote looks like the right answer and is not: in a
# fork-based checkout `main` almost always tracks the fork, because that is
# what `git clone` of your own fork sets up and what `git push` should keep
# meaning. So the tracking remote re-elected the fork in exactly the layout
# this variable exists to handle. Releases belong to the canonical repo — it
# holds the tag history the next version is computed FROM (a tagless fork
# computes 1.0.0 and is wrong every time) and it is where a bot can actually
# be granted push. `upstream` is the near-universal name for it; the tracking
# remote stays the fallback, then `origin`, so single-remote checkouts are
# unaffected. Override for one run with `just sr_remote=origin <recipe>`.
sr_remote := ```
    if git remote get-url upstream >/dev/null 2>&1; then
        printf 'upstream'
    else
        r=$(git config --get "branch.$(git rev-parse --abbrev-ref HEAD 2>/dev/null).remote" 2>/dev/null || true)
        printf '%s' "${r:-origin}"
    fi
```

# shell() rather than backticks: backticks do NOT interpolate {{…}}, so a
# plain `git remote get-url {{sr_remote}}` silently asks for a remote named
# literally "{{sr_remote}}". shell() passes the value as $1 instead.
#
# The slug is the LAST TWO PATH SEGMENTS, not "whatever follows :<port>/". Once
# the release remote stopped being `origin` it also stopped being reliably an
# SSH URL: `upstream` is very often added as HTTPS, which has no :<port>/ to
# anchor on, and the old expression then yielded the whole
# https://host/owner/repo as the "slug" — a value that reaches the Job manifest
# and hangs the Release off nothing. owner/repo is the one shape all three
# forms (ssh://…:2222/o/r.git, https://host/o/r.git, git@host:o/r.git) share.
# Only the slug is needed downstream, so an HTTPS remote is now fine: the Job
# clones over SSH regardless, from the URL the manifest builds.
sr_repo_url := shell("git remote get-url \"$1\"", sr_remote)
sr_slug     := shell("git remote get-url \"$1\" | sed -E 's#\\.git$##; s#.*[/:]([^/]+/[^/]+)$#\\1#'", sr_remote)
sr_repo     := shell("git remote get-url \"$1\" | sed -E 's#.*/##; s#\\.git$##'", sr_remote)

# The Job manifest and the grant script both ship WITH the skill; a checkout
# that keeps its own copy (forge-agents does, at these paths) wins. Found here
# rather than baked in as an absolute path, so this block is machine-independent
# and survives being committed.
sr_manifest := ```
    for p in openshift/job-semantic-release.yaml \
             job-semantic-release.yaml \
             .agents/skills/enable-semantic-release/job-semantic-release.yaml; do
        [ -f "$p" ] && { printf '%s' "$p"; break; }
    done
    true   # an empty result is an answer; a non-zero exit here fails the parse
```
sr_tool := ```
    for p in ./enable-semantic-release.py \
             .agents/skills/enable-semantic-release/enable-semantic-release.py; do
        [ -x "$p" ] && { printf '%s' "$p"; break; }
    done
    true
```

# Every precondition for a release, checked read-only. Safe to run any time.
preflight:
    #!/usr/bin/env bash
    set -uo pipefail
    fail=0
    note() { printf '  %-4s %s\n' "$1" "$2"; }
    echo
    # The remote is auto-detected, so name it: a silent wrong guess here is the
    # whole failure this variable exists to prevent.
    echo "preflight for {{sr_slug}} (remote: {{sr_remote}}):"
    echo

    # Warn, do not fail, when `origin` is the only candidate. A single-remote
    # checkout is the common case and usually correct — but it is also what a
    # fork looks like before anyone adds `upstream`, and that fork releases
    # green off the wrong tag history. Naming the ambiguity is the whole fix;
    # deciding it here would break every repo that legitimately has one remote.
    if [ "{{sr_remote}}" = "origin" ] && ! git remote get-url upstream >/dev/null 2>&1; then
        note "note" "no 'upstream' remote — releasing to origin ({{sr_slug}}). If this is a fork, that is the wrong repo: add upstream, or pass sr_remote=."
    fi

    if ! command -v oc >/dev/null; then
        note "✗" "oc is not on PATH"; fail=1
    elif ! user=$(oc whoami 2>&1); then
        note "✗" "oc is not logged in: $user"; fail=1
    elif ! oc get namespace {{sr_namespace}} >/dev/null 2>&1; then
        note "✗" "namespace {{sr_namespace}} not reachable from this cluster"; fail=1
    else
        note "ok" "oc is $user, {{sr_namespace}} reachable"
    fi

    # .releaserc releases from the release branch only; from any other branch
    # semantic-release exits 0 having done nothing, which reads as success.
    branch=$(git rev-parse --abbrev-ref HEAD)
    if [ "$branch" != "main" ]; then
        note "✗" "on branch '$branch' — .releaserc releases from main only"; fail=1
    else
        note "ok" "on main"
    fi

    # The check that actually matters: the Job releases {{sr_remote}}/main.
    git fetch -q {{sr_remote}} main 2>/dev/null || true
    if ! ahead=$(git rev-list --count {{sr_remote}}/main..HEAD 2>/dev/null); then
        note "✗" "no {{sr_remote}}/main to compare against"; fail=1
    elif [ "$ahead" != "0" ]; then
        note "✗" "$ahead commit(s) not pushed to {{sr_remote}} — the Job would release without them"; fail=1
    else
        behind=$(git rev-list --count HEAD..{{sr_remote}}/main)
        [ "$behind" = "0" ] \
            && note "ok" "{{sr_remote}}/main is exactly this commit" \
            || note "ok" "{{sr_remote}}/main is $behind ahead; that is what gets released"
    fi

    # Informational only — the Job never sees the working tree.
    git diff --quiet && git diff --cached --quiet \
        || note "note" "working tree is dirty; harmless, the Job clones from the forge"

    # The failure this whole arrangement exists to prevent: a grant that looks
    # fine and rejects the push. Delegated to the tool that knows all four parts.
    if [ -n "{{sr_tool}}" ]; then
        if out=$({{sr_tool}} "{{sr_repo_url}}" --dry-run --only grant 2>&1); then
            if grep -q "nothing to do" <<<"$out"; then
                note "ok" "{{sr_agent}} already has write on {{sr_slug}}"
            else
                note "✗" "{{sr_agent}} is not fully granted on {{sr_slug}} — the push would be rejected"
                sed 's/^/       /' <<<"$out"
                echo "       fix: {{sr_tool}} {{sr_repo_url}}"
                fail=1
            fi
        else
            note "✗" "the grant check itself failed:"; sed 's/^/       /' <<<"$out"; fail=1
        fi
    else
        note "✗" "enable-semantic-release.py not found — reinstall the skill"; fail=1
    fi

    echo
    [ "$fail" = "0" ] && echo "✔ ready to release" || echo "✗ not ready — fix the ✗ lines above"
    exit $fail

# Survey a release without cutting one: no tag, no commit, no Release object
release-dry: preflight
    @just _semantic-release-run dry-run "--debug --no-ci --dry-run"

# Cut a real release: CHANGELOG commit, signed tag, Release, release assets
release: preflight
    @just _semantic-release-run release "--debug --no-ci"

# Shared runner. Name carries the epoch because a Job's pod template is
# immutable — `create`, never `apply`, and never the same name twice.
_semantic-release-run kind args:
    #!/usr/bin/env bash
    set -euo pipefail
    [ -n "{{sr_manifest}}" ] || { echo "✗ job-semantic-release.yaml not found"; exit 1; }
    job="semantic-release-{{sr_repo}}-{{kind}}-$(date +%s)"
    argv=$(printf '"%s", ' {{args}}); argv="[${argv%, }]"

    echo
    echo "creating $job ({{kind}})"
    # The third -e retargets the manifest at THIS repo. The slug appears twice
    # over there — REPO_URL and FORGEJO_REPOSITORY, which decides which repo gets
    # the Release object — so it is /g, and a no-op in the repo it ships from.
    sed -e "s|name: semantic-release-dry-run\$|name: $job|" \
        -e "s|^          args: .*|          args: $argv|" \
        -e "s|agentic-forges/forge-agents|{{sr_slug}}|g" \
        {{sr_manifest}} | oc -n {{sr_namespace}} create -f -

    echo "waiting…"
    for _ in $(seq 1 60); do
        s=$(oc -n {{sr_namespace}} get job "$job" \
              -o jsonpath='{.status.conditions[0].type}' 2>/dev/null || true)
        case "$s" in
            Complete|SuccessCriteriaMet) echo; echo "✔ $job complete"
                # --debug interleaves multi-line object dumps that contain the
                # same phrases; drop them before summarising or the summary is
                # a wall of JSON. Anchored on the logger prefix for the same
                # reason: commit messages can quote these strings verbatim.
                oc -n {{sr_namespace}} logs "job/$job" -c semantic-release 2>&1 \
                  | grep -vE '^[0-9]{4}-[0-9]{2}-[0-9]{2}' \
                  | grep -E "\[semantic-release\].*(next release version|Created tag|Published (file|Forgejo|release)|complete: no release)" \
                  | sed -E 's/^\[[0-9:]+ [AP]M\] //; s/^/  /' || true
                exit 0 ;;
            Failed|FailureTarget) echo; echo "✗ $job failed"
                oc -n {{sr_namespace}} logs "job/$job" -c semantic-release 2>&1 \
                  | grep -vE '^[0-9]{4}-[0-9]{2}-[0-9]{2}' \
                  | grep -A6 "An error occurred" | head -20
                echo
                echo "full log: oc -n {{sr_namespace}} logs job/$job -c semantic-release"
                exit 1 ;;
        esac
        sleep 5
    done
    echo "✗ timed out after 5m; job $job left in place for inspection"
    exit 1

# Remove finished release Jobs for THIS repo. Leaves other repos' Jobs alone.
clean-jobs:
    #!/usr/bin/env bash
    set -euo pipefail
    jobs=$(oc -n {{sr_namespace}} get jobs -o name \
             | grep "job.batch/semantic-release-{{sr_repo}}-" || true)
    [ -z "$jobs" ] && { echo "no {{sr_repo}} release Jobs to remove"; exit 0; }
    echo "$jobs"
    read -rp "delete these? [y/N] " a
    [ "$a" = "y" ] && oc -n {{sr_namespace}} delete $jobs || echo "left alone"
# <<< enable-semantic-release v1.4.2 sha256:98b96f591752 <<<
