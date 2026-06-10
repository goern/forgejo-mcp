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
5. **Bound the output.** If response size depends on data (not tool
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
|---|---|---|
| `forgejo://owner/{owner}` | application/json | User or org profile |
| `forgejo://repo/{owner}/{repo}` | application/json | Repository overview |
| `forgejo://repo/{owner}/{repo}/commit/{sha}` | application/json + markdown | Commit metadata (sha must be 40 hex chars) |
| `forgejo://repo/{owner}/{repo}/commit/{sha}/status` | application/json | Combined CI status |
| `forgejo://repo/{owner}/{repo}/issue/{index}` | application/json + markdown | Issue with bounded comments (cap 30) |
| `forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}` | application/json + markdown | Single comment |
| `forgejo://repo/{owner}/{repo}/pr/{index}` | application/json + markdown | PR with bounded comments + reviews (cap 30) |
| `forgejo://repo/{owner}/{repo}/branch_protections` | application/json | Bounded list of branch protection rules |
| `forgejo://repo/{owner}/{repo}/branch_protection/{rule}` | application/json | Single branch protection rule |
| `forgejo://repo/{owner}/{repo}/label/{id}` | application/json | Single repository label |
| `forgejo://repo/{owner}/{repo}/labels{?page,limit}` | application/json | Bounded repo label list (cap 30, sentinel `list_repo_labels`) |
| `forgejo://org/{org}/labels{?page,limit}` | application/json | Bounded org label list (cap 30, sentinel `list_org_labels`) |

## Blocked Features

Some features are blocked on upstream API/SDK support. See `docs/plans/` for:

- `wiki-support.md` - Wiki API (blocked on forgejo-sdk)
- `projects-support.md` - Projects/Kanban API (blocked on Gitea 1.26.0)

## Repository Labels

Labels for goern/forgejo-mcp on Codeberg:

| ID | Name | Color | Description |
|----|------|-------|-------------|
| 335058 | Kind/Feature | 0288d1 | New functionality |
| 335061 | Kind/Enhancement | 84b6eb | Improve existing functionality |
| 335091 | Status/Blocked | 880e4f | Something is blocking this issue or pull request |
| 335103 | Priority/Medium | e64a19 | The priority is medium |
| 1702838 | RFC - Request For Comments | 0e8a16 | Request For Comments — design/spec open for feedback before implementation |

### Usage with Codeberg MCP

When adding labels via the `mcp__codeberg__add_issue_labels` tool, use the numeric ID:

```
mcp__codeberg__add_issue_labels(
  owner: "goern",
  repo: "forgejo-mcp",
  index: <issue_number>,
  labels: "<label_id>"  # e.g., "335091" for Status/Blocked
)
```

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
