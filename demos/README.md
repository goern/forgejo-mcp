# forgejo-mcp Demos

End-to-end, copy-pasteable walkthroughs for the MCP tools shipped by
`forgejo-mcp`. Each demo is a single Markdown file containing real
`./forgejo-mcp --cli` invocations against `codeberg.org` together with
the output they produced — the same payload an MCP client would see.

## How to read a demo

Every demo follows the same shape:

1. **Background / What these tools do** — the user-facing problem the
   feature solves and the tool surface it adds.
2. **Setup** — environment variables and the `make build` line. Identical
   across demos; once your shell is set up, skip it on subsequent reads.
3. **Walkthrough** — numbered, runnable shell blocks paired with the
   exact output produced. Where helpful, a Python one-liner formats the
   raw JSON envelope down to the fields that matter.
4. **End-to-end / Autonomous workflow** — how an agent strings the
   primitives together into a useful task (triage, review, time-track,
   etc.).

Demos use the CLI front-end (`--cli <tool> --args '<json>'`) because it
is the same code path as MCP `tools/call` but plays well with shell
pipelines. Everything shown also works over stdio MCP and the streamable
HTTP transport.

## Setup once, run anywhere

```bash
export FORGEJO_URL=https://codeberg.org
export FORGEJO_ACCESS_TOKEN=<your-token>
make build
```

After that, every command in every demo starts with `./forgejo-mcp --cli`.

---

## Demos by topic

### 1. Issues, labels, milestones

Discovery and write tools for the core issue-tracking primitives.
Autonomous agents need to map names → numeric IDs before they can call
the mutating tools; the discovery demos cover that, the label demos
cover the mutations.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [list-milestones-labels.md](list-milestones-labels.md) | `list_repo_labels`, `list_repo_milestones` | Discover the ID↔name mapping needed by `add_issue_labels` and `update_issue` |
| [issue-labels.md](issue-labels.md) | `add_issue_labels`, `remove_issue_labels` | Full add/remove cycle on a real issue, plus multi-label calls |
| [org-labels.md](org-labels.md) | `list_org_labels`, merged `list_repo_labels` | Org-scope labels surfaced through the same ID space, with `scope` field and opt-out |
| [label-management.md](label-management.md) | `create_repo_label`, `edit_repo_label`, `delete_repo_label`, `get_repo_label`, `create_org_label`, `edit_org_label`, `delete_org_label`, `get_org_label` + 3 resource templates | Full label lifecycle: create with color normalisation, PATCH-edit, safe-delete with in-use guard, URI-addressable resources |

**Use case.** Build a label lookup table once, then have the agent
classify issues and apply labels without ever leaving the MCP loop.
`label-management.md` covers the full lifecycle — agents can now also
*create* the label taxonomy from scratch without leaving MCP.

### 2. Attachments

Forgejo lets users drop files on issues and on individual comments.
These demos cover the full CRUD shape — list, get, download, create,
edit, delete — for both surfaces.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [issue-attachments.md](issue-attachments.md) | 6 tools keyed by `index` | Upload, inspect, download, rename, delete attachments on an issue/PR |
| [comment-attachments.md](comment-attachments.md) | 6 tools keyed by `comment_id` | Same lifecycle on individual comment attachments |

**Use case.** An agent triaging a bug report needs to fetch the
attached log file before reasoning about it; an agent writing a release
note needs to attach a generated changelog to the release comment.

### 3. Releases

Tag-anchored release records and their binary assets. CRUD on releases (with a client-side `state` filter and `target_commitish` for new tags) plus the full attachment lifecycle on each release.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [release-management.md](release-management.md) | 14 tools — 8 release + 6 release-attachment | Read flow against `goern/forgejo-mcp` (list/latest/by-tag/state filter/list assets/over-cap download) plus the parameter surface for the write tools and the autonomous "draft notes for the next tag" workflow |

**Use case.** A release-housekeeping agent that reads `get_latest_release`, summarises the commit log since `published_at` into Markdown notes, drafts the next release with `create_release`, optionally uploads built binaries with `create_release_attachment`, and waits for a human to flip `draft=false` via `edit_release`.

### 4. Time tracking

Forgejo carries a per-issue tracked-time ledger and a live stopwatch.
Two demos split read/write of the ledger from the stopwatch transitions,
because they are different mental models.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [issue-time-tracking.md](issue-time-tracking.md) | 6 tools — list, add, delete, reset, user/repo aggregates | Manage the tracked-time ledger directly |
| [issue-stopwatch.md](issue-stopwatch.md) | 4 tools — start, stop, cancel, list mine | Drive the live stopwatch so the server computes the elapsed time |

**Use case.** Agents that run long-lived tasks can record the time
they actually spent without having to compute deltas themselves —
start the stopwatch when work begins, stop it when work ends, let
Forgejo do the math.

### 5. Notifications

Two demos: the lightweight "what's new" check, and the full
notification-management API for marking read, fetching threads, and
clearing inboxes.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [check-notifications.md](check-notifications.md) | `check_notifications` | Read-only inbox poll across all watched repos |
| [notifications-management.md](notifications-management.md) | list/get/mark-read tools | 100% notification API coverage — per-thread and bulk |

**Use case.** A daily-standup agent that opens with "since yesterday,
N notifications across M repos" and can clear them as it processes
each one.

### 6. Organization management

| Demo | Tools | What it shows |
|------|-------|---------------|
| [org-management.md](org-management.md) | 15 tools in the `org` domain | CRUD on the org itself, membership, and teams |

**Use case.** Provisioning workflows — spin up a new org, add the
team, attach repos, all from a single agent transcript with no
web-UI clicks.

### 7. Code review (bounded I/O)

| Demo | Tools | What it shows |
|------|-------|---------------|
| [bounded-responses.md](bounded-responses.md) | `get_pull_request_diff` with `file_path`, `get_file_content` with `start_line`/`end_line` | Cut payloads to just the file or line range the agent needs (measured 16× / 41× reductions on real data) |

**Use case.** Reviewing a PR no longer means pulling the whole diff
into the model's context. Pick one file's hunks, optionally read a
few lines of surrounding source around each hunk, repeat. Per-call
payloads stay proportional to what the agent actually inspects.

This is the user-facing half of the architectural rule in
[`../docs/design/output-bounding.md`](../docs/design/output-bounding.md):
every data-proportional response in this server must be bounded by
the caller. Expect future tools to follow the same pattern.

### 8. Transport / infrastructure

| Demo | Feature | What it shows |
|------|---------|---------------|
| [streamable-http-transport.md](streamable-http-transport.md) | `--transport http` | Run forgejo-mcp as a remote MCP server compatible with Claude.ai's custom-connector flow |

**Use case.** Hosting a single forgejo-mcp instance behind an HTTPS
endpoint and pointing multiple MCP clients at it, instead of every
client spawning its own stdio subprocess.

### 9. Branch protection (governance)

CRUD on a repository's branch protection rules — require status checks
or approvals before merge, and whitelist specific users (e.g. a release
bot) to push to an otherwise locked branch. This demo is **token-free**:
it proves the surface through the CLI tool registry and the `httptest`
suite, since reading/writing real protection needs a repo-admin token.
It is co-located with its spec under `openspec/`, not in this folder.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [../openspec/specs/branch-protection/branch-protection.demo.md](../openspec/specs/branch-protection/branch-protection.demo.md) | 5 tools — `list`/`get`/`create`/`edit`/`delete_branch_protection` | Registration, the `branch_name`-required guard, push/merge/approvals whitelist params, and PATCH null-safety (unpassed fields never wipe an existing rule) |

**Use case.** A governance agent that locks `main`, requires green CI
before merge, and whitelists a release bot to push tags — without
relaxing protection for anyone else.

### 10. Repository webhooks

CRUD on a repository's webhooks — list, get, create, edit, delete, and trigger
a test delivery. Two resource templates expose the hook collection and single
hooks as `forgejo://` URIs. The core surface proof is **token-free**; it
verifies tool registration, parameter validation, and the secret-exclusion
guarantee through the CLI tool registry and source analysis. It is co-located
with its spec under `openspec/`, not in this folder.

| Demo | Tools | What it shows |
|------|-------|---------------|
| [../openspec/changes/repo-webhook-tools/specs/repo-webhook-tools/repo-webhook-tools.demo.md](../openspec/changes/repo-webhook-tools/specs/repo-webhook-tools/repo-webhook-tools.demo.md) | 6 tools — `list`/`get`/`create`/`edit`/`delete_repo_hook` + `test_repo_hook`; 2 resources — `forgejo://repo/{owner}/{repo}/hooks` + `.../hook/{id}` | Tool registration, required-param guards (all 6 tools), `safeHook()` explicit-allowlist proof that `secret` is never echoed, `test_repo_hook` live-delivery warning, URI parser `invalid-params` vs `not-found` distinction |

**Use case.** An automation agent that registers a CI notification hook on
every newly created repository, tests the delivery to confirm the endpoint is
reachable, and removes stale hooks whose URLs no longer respond.

---

## Cross-cutting workflows

The demos individually cover single tool families. The interesting
agent workflows compose across them:

- **Autonomous issue triage.** §1 (discover labels) → read issue
  body → §2 (fetch attachments if any) → §1 (apply labels) →
  §4 (start stopwatch if the agent will keep working on it).
- **Code review.** §7 (per-file diff slices + per-range file reads)
  → review-write tools (covered in the top-level README, not yet in a
  dedicated demo) → merge.
- **Release housekeeping.** §3 (draft notes via `get_latest_release` +
  commit log, then `create_release` with `draft=true`) → §5 (process
  notifications on the new release) → §6 (rotate team membership if
  needed).

## Conventions

- **`scope` field on labels.** Returned by `list_repo_labels` and
  `list_org_labels`. `"repo"` or `"org"`. Both can be passed to
  `add_issue_labels` without distinction.
- **`index` vs `comment_id`.** Issue/PR-level tools take `index`
  (the per-repo issue number). Comment-level tools take `comment_id`
  (the global comment ID returned by `list_issue_comments`).
- **JSON envelope.** CLI responses are an array of MCP `Content`
  blocks, e.g. `[{"type":"text","text":"..."}]`. Plain-text tools
  wrap the payload in a second `{"Result":...}` layer; the demo
  scripts unwrap both.
- **Showboat stamps.** The `<!-- showboat-id: ... -->` comment at
  the top of each demo lets the Showboat tool detect and refresh
  the file in place when the feature evolves.

## Adding a new demo

When a new feature ships, add a demo file under `demos/` and register
it in the right section of this README. Keep the existing shape:
background → setup → numbered walkthrough with output blocks →
end-to-end workflow. Use real data from `codeberg.org` where possible
so the numbers (sizes, counts, IDs) stay honest.
