# AGENTS.md

This file provides guidance to AI coding assistants (Claude Code, Cursor, etc.) when working with this repository.

For detailed developer documentation, see [DEVELOPER.md](DEVELOPER.md).

## Quick Reference

```bash
make build          # Build the binary (outputs ./forgejo-mcp)
make vendor         # Tidy and verify Go module dependencies
```

## Architecture Summary

```
main.go → cmd/cmd.go (CLI parsing) → operation/operation.go (tool registration) → operation/{domain}/*.go (tool handlers)
```

Key directories:

- `operation/` - MCP tool definitions and handlers by domain
- `pkg/forgejo/` - Singleton Forgejo SDK client wrapper
- `pkg/to/` - Response formatting helpers
- `pkg/params/` - Shared parameter descriptions

## File Header

Every new source file MUST begin with an SPDX license header as the very first lines, before any package declaration or imports:

```go
// SPDX-License-Identifier: GPL-3.0-or-later
```

For non-Go files (YAML, Markdown, shell, etc.), use the appropriate comment syntax:

```yaml
# SPDX-License-Identifier: GPL-3.0-or-later
```

```bash
# SPDX-License-Identifier: GPL-3.0-or-later
```

Do not add a copyright line — the SPDX identifier line alone is sufficient.

## Adding a New Tool

1. Create or modify a file in `operation/{domain}/`
2. Define tool with `mcp.NewTool()` and implement handler function
3. Register in the domain's `RegisterTool(s *server.MCPServer)` function
4. If new domain, import and call in `operation/operation.go`
5. **Escape API paths.** If the tool uses the raw-HTTP helpers
   (`forgejo.DoJSON`, `DoJSONList`, `DoAPIRaw`, `DoMultipart`) rather than the
   SDK, build the path with `forgejo.APIPath("repos", owner, repo, …)` — it
   escapes every segment. A hand-rolled `fmt.Sprintf("/repos/%s/…")` lets an
   owner containing `/` or `?` retarget the request at another endpoint;
   `scripts/ci/check-api-path-escaping.sh` fails the build on it. Append query
   strings to the `APIPath` result.
6. **Bound the output.** If response size depends on data (not tool
   semantics), the tool MUST satisfy [docs/design/output-bounding.md](docs/design/output-bounding.md):
   client-controlled bound + resumability + documented parameters. Use the
   checklist there in the PR description.

See [DEVELOPER.md](DEVELOPER.md) for complete code examples and patterns.

## Resources

Resource templates expose Forgejo entities as `forgejo://` URIs — instance-portable, additive, coexisting with all existing tools (no tool removed). Clients that support `resources/templates/list` and `resources/read` resolve these URIs directly; others fall back to tools transparently.

When adding a new resource template, place it under `operation/<domain>/resources*.go`. Use the `operation/resource` package for URI parsing (`ParseXxx`), embedded-list bounding (`Bounded`), and error mapping (`MapForgejoError`). Embedded lists MUST use `operation/resource.Bounded` so the truncation sentinel stays consistent across resources.

See `openspec/specs/mcp-resources-core/spec.md` for the full normative spec (added by this slice when the change archives).

### Resource table

| URI template | MIME | What it returns |
| --- | --- | --- |
| `forgejo://owner/{owner}` | application/json | User or org profile |
| `forgejo://repo/{owner}/{repo}` | application/json | Repository overview |
| `forgejo://repo/{owner}/{repo}/commit/{sha}` | application/json + markdown | Commit metadata (sha must be 40 hex chars) |
| `forgejo://repo/{owner}/{repo}/commit/{sha}/status` | application/json | Combined CI status |
| `forgejo://repo/{owner}/{repo}/issue/{index}` | application/json + markdown | Issue with bounded comments (cap 30) |
| `forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}` | application/json | Bounded issue rows, no bodies (state default open; cap 30, sentinel `list_repo_issues`); in `labels`, encode a literal `/` as `%2F` and spaces as `%20` without double-encoding (`+` does not work), and note that names are AND-ed and an unrecognised one is silently dropped — if none match you get the unfiltered set, not an empty one |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` | application/json + markdown | Single comment |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}` | application/json | Bounded comment thread, full bodies (cap 30, sentinel `list_issue_comments`) |
| `forgejo://repo/{owner}/{repo}/pr/{index}` | application/json + markdown | PR with bounded comments + reviews (cap 30) |
| `forgejo://repo/{owner}/{repo}/branch_protections` | application/json | Bounded list of branch protection rules |
| `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` | application/json | Single branch protection rule; rule names are branch patterns, so encode a literal `/` as `%2F` and spaces as `%20` without double-encoding (`release%2Fv1`) — a raw `/` does not resolve |
| `forgejo://repo/{owner}/{repo}/label/{id}` | application/json | Single repository label |
| `forgejo://repo/{owner}/{repo}/labels{?page,limit}` | application/json | Bounded repo label list (cap 30, sentinel `list_repo_labels`) |
| `forgejo://org/{org}/labels{?page,limit}` | application/json | Bounded org label list (cap 30, sentinel `list_org_labels`) |
| `forgejo://repo/{owner}/{repo}/hooks` | application/json | Bounded list of repository webhooks (cap 30, sentinel `list_repo_hooks`; secret never returned) |
| `forgejo://repo/{owner}/{repo}/hook/{id}` | application/json | Single repository webhook (secret never returned) |
| `forgejo://repo/{owner}/{repo}/wiki/{pageName}` | application/json + markdown | Wiki page with bounded revisions; use the normalized `page_name`, encoding literal `/` as `%2F` and spaces as `%20` without double-encoding |

Wiki tools live in `operation/wiki/`. `list_*` enumerates entities; `get_*` fetches one
entity by its server-normalized name. Always reuse the `page_name` returned by create/list.
Slash-separated titles are only a flat subpage naming convention: Forgejo does not create a
parent page or store a parent-child relationship. Create any desired parent page separately.

## Blocked Features

Some features are blocked on upstream API/SDK support. See `docs/plans/` for:

- `wiki-support.md` - historical SDK plan, superseded by the direct REST implementation
- `projects-support.md` - Projects/Kanban API (blocked on Gitea 1.26.0)

## Repository Labels

The label vocabulary is declarative, not hand-curated. `.castra/labels.json`
is the source of truth for the labels this repo wants; apply it with:

```bash
castra init-labels --repo agentic-forges/forgejo-mcp --file .castra/labels.json --dry-run
castra init-labels --repo agentic-forges/forgejo-mcp --file .castra/labels.json
```

It is idempotent: existing labels are skipped, missing ones are created.
`--file` is required — castra's default search looks in `./`, not `.castra/`.
Pass `--overwrite` to push colour/description edits onto labels that already
exist. **It never deletes**, so retiring a label means removing it from the
file *and* deleting it on the forge by hand.

Do NOT hardcode label IDs in documentation — they are per-forge and did not
survive the Codeberg → git.b4mad.industries migration. List them first:

```
mcp__b4mad__list_repo_labels(owner: "agentic-forges", repo: "forgejo-mcp")
mcp__b4mad__add_issue_labels(owner: "agentic-forges", repo: "forgejo-mcp",
                             index: <number>, labels: "<numeric ids, comma separated>")
```

### Review states

For pull requests, distinguish the four "waiting" states — they are not
interchangeable, and `Status/Blocked` in particular is easy to misuse:

| Label | Means |
| ----- | ----- |
| `Reviewed/Approved` | Review passed, no changes requested |
| `Reviewed/Changes-Requested` | Reviewed, changes required before merge |
| `Status/Awaiting-Author` | Waiting on the author to act |
| `Status/Need More Info` | We cannot proceed until a question is answered |
| `Status/Blocked` | Blocked on something **external** — never "waiting on author" |

`Reviewed/Confirmed` means "bug reproduced". It is an issue label; it does not
describe a code review.

Some labels on the forge predate this file and are deliberately not declared
in it: the `hermes-*` set (superseded bot) and the `area:*` lanes (they arrived
from castra's built-in default vocabulary and describe another repo's domain).

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:

   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```

5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**

- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->

### Session completion: two corrections to the block above

The Session Completion steps above are generated by `bd` and get rewritten
whenever the integration block is regenerated, so these two corrections live
outside it and take precedence:

- **Do not run `bd dolt push`.** This project does not sync beads to a remote:
  `.beads/config.yaml` still points `sync.remote` at the pre-migration
  Codeberg URL, so a push targets a dead endpoint. Beads live in Dolt locally
  and issue data never enters git, so nothing is stranded by skipping it.
- **"Clear stashes" is conditional.** Check what a stash holds first — it may
  be another branch's only local copy of its work. Leave those alone.

## Attribution Requirements

AI agents must disclose what tool and model they are using in the "Assisted-by" commit footer:

```text
Assisted-by: [Model Name] via [Tool Name]
```

Example:

```text
Assisted-by: GLM 4.6 via Claude Code
```

<!-- >>> enable-semantic-release v0.0.0-dev sha256:66510a787d1a (managed block — do not edit) >>> -->
## Releasing

Releases are cut by an OpenShift Job, never from your machine. The Job clones
**the release remote's main from the forge** — `just preflight` prints which
remote that is — and never sees your working tree: an unpushed commit is not in
the release, and a dirty tree is harmless.

- `just preflight` — every precondition, read-only. Run it first; `just release`
  runs it anyway and stops if it fails.
- `just release` — cut one. `just release-dry` surveys without cutting.
- Commit messages must be conventional (`feat:`, `fix:`, `chore:`, …). They are
  the only input to the version number and the changelog, so a release that
  produced nothing usually means nothing releasable was committed.

**`just release` returning is not the release finishing.** The Job creates the
tag and the Release object, then exits. If the repo has tag-triggered CI that
builds and attaches assets — binaries, SBOMs, signatures — that runs afterwards
and takes minutes longer. A release inspected in that window looks like it is
missing assets, and it is not: it is unfinished. Wait for the tag pipeline to
report success before judging anything absent.

`just --list` for the rest. Do not run semantic-release locally.
<!-- <<< enable-semantic-release v0.0.0-dev sha256:66510a787d1a <<< -->

### Releasing: what the tag pipeline does after the Job exits

The block above is generated by the `enable-semantic-release` skill and is
rewritten whenever it is regenerated, so this repo-specific detail lives
outside it.

`just release` returns as soon as the semantic-release Job finishes — which is
only the tag and the Release object. The tag push then triggers
`.tekton/on-tag-push-release.yaml` in the **`op1st-pipelines`** namespace, and
*that* is what produces the release assets: goreleaser builds the binaries and
SBOMs, `cosign-sign-release` attaches `checksums.txt.sig`, `mcpb-pack` builds
the four `.mcpb` bundles, and the image tasks build, push, and sign the
container image.

It takes several minutes. **A release inspected before that pipeline finishes
looks like it is missing assets — it is not, it is unfinished.** Check the
pipeline before concluding anything is wrong:

```bash
oc -n op1st-pipelines get pipelinerun | grep on-tag-push-release
oc -n op1st-pipelines get taskrun -l tekton.dev/pipelineRun=<run-name>
```

A complete release has **14 assets**: 4 tarballs, 4 SBOMs, 4 `.mcpb`,
`checksums.txt`, and `checksums.txt.sig`. Compare against the previous tag
rather than against memory.
